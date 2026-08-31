# Redis 分布式锁与限流器 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `go-middleware/redis` 包新增分布式锁（`Mutex`）与限流器（`Limiter`），封装在 `redis.UniversalClient` 之上，供业务项目复用。

**Architecture:** `lock.go` 实现单实例 `SET NX PX` 加锁 + Lua 原子释放脚本 + watchdog 续期 goroutine；`limiter.go` 实现基于 Redis Hash 的令牌桶算法，Lua 脚本原子完成补充/扣减；两者都通过 `redis.UniversalClient` 复用现有连接层，错误统一走 `go-common/error` 的 `oops` 风格。

**Tech Stack:** Go 1.26（workspace `go-middleware` 子模块），`github.com/redis/go-redis/v9`（含 `redis.Script`），`github.com/alicebob/miniredis/v2` v2.38.0（测试用嵌入式 Redis），`github.com/stretchr/testify` v1.12.1。

**Spec:** `docs/superpowers/specs/2026-08-31-redis-lock-limiter-design.md`

## Global Constraints

- 模块边界：仅修改 `go-middleware/redis` 包，不引入对 `go-framework` 的依赖（siblings 规则）。
- 错误码：新增 `CodeLockAcquire=20103`、`CodeLockRelease=20104`、`CodeLimiterEval=20105`，落在 `go-middleware` 段（20000-20699）内，redis 包自有子段。
- 所有导出符号（类型/函数/常量）必须有 `// Name ...` 格式的 godoc 注释（revive lint 要求）。
- 构造函数使用 Functional Options 模式（`NewMutex`/`NewLimiter` 均为 3+ 参数），遵循 `.claude/rules/options-pattern.md`。
- 测试文件与源码同目录、`package redis`（内部测试包，与现有 `client_test.go`/`config_test.go` 一致），复用 `miniredis.RunT` 模式。
- `defer f.Close()` 类语句需显式忽略错误（`defer func() { _ = x.Close() }()`），符合 `errcheck` 规则。
- 每个 Lua 脚本用 `redis.NewScript` 包级变量定义一次，不在函数内重复构造。

---

### Task 1: 扩展错误码（Lock/Limiter）

**Files:**
- Modify: `go-middleware/redis/errors.go`
- Modify: `go-middleware/redis/errors_test.go`

**Interfaces:**
- Produces: `CodeLockAcquire = 20103`、`CodeLockRelease = 20104`、`CodeLimiterEval = 20105`（`int` 常量）；`ErrLockAcquire`、`ErrLockRelease`、`ErrLimiterEval`（`*oops.OopsErrorBuilder`，与现有 `ErrConnect` 同类型）。后续 Task 2-9 直接引用这三个 error 构造器。

- [ ] **Step 1: 写失败测试（新增错误码值 + 构造器 + HTTP 状态映射）**

在 `go-middleware/redis/errors_test.go` 末尾追加：

```go
// TestLockLimiterCodeValues 码值是 wire 契约，逐值锁定。
func TestLockLimiterCodeValues(t *testing.T) {
	assert.Equal(t, 20103, redis.CodeLockAcquire)
	assert.Equal(t, 20104, redis.CodeLockRelease)
	assert.Equal(t, 20105, redis.CodeLimiterEval)
}

// TestLockLimiterPredefinedErrors 构造器 code + public 消息符合预期。
func TestLockLimiterPredefinedErrors(t *testing.T) {
	code, public := goerror.Extract(redis.ErrLockAcquire.Wrap(errors.New("x")))
	assert.Equal(t, 20103, code)
	assert.Equal(t, "redis_lock_acquire_error", public)

	code, public = goerror.Extract(redis.ErrLockRelease.Wrap(errors.New("x")))
	assert.Equal(t, 20104, code)
	assert.Equal(t, "redis_lock_release_error", public)

	code, public = goerror.Extract(redis.ErrLimiterEval.Wrap(errors.New("x")))
	assert.Equal(t, 20105, code)
	assert.Equal(t, "redis_limiter_eval_error", public)
}

// TestLockLimiterHTTPStatusRegistration init() 注册的 HTTP 状态映射。
func TestLockLimiterHTTPStatusRegistration(t *testing.T) {
	assert.Equal(t, 409, goerror.HTTPStatus(redis.ErrLockAcquire.Wrap(errors.New("x"))))
	assert.Equal(t, 409, goerror.HTTPStatus(redis.ErrLockRelease.Wrap(errors.New("x"))))
	assert.Equal(t, 500, goerror.HTTPStatus(redis.ErrLimiterEval.Wrap(errors.New("x"))))
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-middleware/redis/... -run TestLockLimiter -v`
Expected: FAIL（`redis.CodeLockAcquire` 等未定义，编译错误）

- [ ] **Step 3: 实现错误码**

将 `go-middleware/redis/errors.go` 替换为：

```go
package redis

import (
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// Redis 错误码 20101-20105。
const (
	// CodeConnect Redis 连接/Ping 失败
	CodeConnect = 20101
	// CodeInstrument Redis OpenTelemetry 埋点初始化失败
	CodeInstrument = 20102
	// CodeLockAcquire 分布式锁加锁失败
	CodeLockAcquire = 20103
	// CodeLockRelease 分布式锁释放失败（未持有或已过期）
	CodeLockRelease = 20104
	// CodeLimiterEval 限流器 Lua 脚本执行失败
	CodeLimiterEval = 20105
)

// 预定义 Redis 错误构造器。
var (
	// ErrConnect Redis 连接/Ping 失败
	ErrConnect = goerror.Code(CodeConnect).Public("redis_connect_error")
	// ErrInstrument Redis OpenTelemetry 埋点初始化失败
	ErrInstrument = goerror.Code(CodeInstrument).Public("redis_instrument_error")
	// ErrLockAcquire 分布式锁加锁失败
	ErrLockAcquire = goerror.Code(CodeLockAcquire).Public("redis_lock_acquire_error")
	// ErrLockRelease 分布式锁释放失败
	ErrLockRelease = goerror.Code(CodeLockRelease).Public("redis_lock_release_error")
	// ErrLimiterEval 限流器 Lua 脚本执行失败
	ErrLimiterEval = goerror.Code(CodeLimiterEval).Public("redis_limiter_eval_error")
)

// init 注册 Redis 错误码的细粒度 HTTP 状态码映射。
func init() {
	goerror.RegisterHTTPStatuses(map[int]int{
		CodeConnect:     503,
		CodeInstrument:  500,
		CodeLockAcquire: 409,
		CodeLockRelease: 409,
		CodeLimiterEval: 500,
	})
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-middleware/redis/... -run TestLockLimiter -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-middleware/redis/errors.go go-middleware/redis/errors_test.go
git commit -m "feat(redis): add error codes for lock and limiter (#56)"
```

---

### Task 2: Mutex 骨架 + Options（构造器，不含加锁逻辑）

**Files:**
- Create: `go-middleware/redis/lock.go`
- Create: `go-middleware/redis/lock_test.go`

**Interfaces:**
- Consumes: `redis.UniversalClient`（`go-redis/v9`）
- Produces：
  - `type Mutex struct { client redis.UniversalClient; key string; ttl time.Duration; retryInterval time.Duration; watchdog bool; mu sync.Mutex; token string; stopCh chan struct{} }`
  - `func NewMutex(client redis.UniversalClient, key string, opts ...MutexOption) *Mutex`
  - `type MutexOption func(*mutexOptions)`
  - `func WithTTL(ttl time.Duration) MutexOption`
  - `func WithRetryInterval(interval time.Duration) MutexOption`
  - `func WithWatchdog(enabled bool) MutexOption`
  - 后续 Task 3-6 在此文件基础上追加 `TryLock`/`Unlock`/`Lock`/watchdog 方法

- [ ] **Step 1: 写失败测试（默认值 + Options 覆盖）**

创建 `go-middleware/redis/lock_test.go`：

```go
package redis

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func newTestRedisClient(t *testing.T) (*miniredis.Miniredis, goredis.UniversalClient) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, client
}

func TestNewMutex_Defaults(t *testing.T) {
	_, client := newTestRedisClient(t)

	m := NewMutex(client, "lock:test")

	assert.Equal(t, "lock:test", m.key)
	assert.Equal(t, defaultMutexTTL, m.ttl)
	assert.Equal(t, defaultRetryInterval, m.retryInterval)
	assert.True(t, m.watchdog)
}

func TestNewMutex_CustomOptions(t *testing.T) {
	_, client := newTestRedisClient(t)

	m := NewMutex(client, "lock:test",
		WithTTL(5*time.Second),
		WithRetryInterval(20*time.Millisecond),
		WithWatchdog(false),
	)

	assert.Equal(t, 5*time.Second, m.ttl)
	assert.Equal(t, 20*time.Millisecond, m.retryInterval)
	assert.False(t, m.watchdog)
}

func TestMutexOption_IgnoresInvalidValues(t *testing.T) {
	_, client := newTestRedisClient(t)

	m := NewMutex(client, "lock:test", WithTTL(0), WithRetryInterval(-1))

	assert.Equal(t, defaultMutexTTL, m.ttl)
	assert.Equal(t, defaultRetryInterval, m.retryInterval)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-middleware/redis/... -run TestNewMutex -v`
Expected: FAIL（`lock.go` 不存在，`NewMutex`/`defaultMutexTTL` 未定义）

- [ ] **Step 3: 实现 Mutex 骨架**

创建 `go-middleware/redis/lock.go`：

```go
package redis

import (
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const (
	defaultMutexTTL      = 10 * time.Second
	defaultRetryInterval = 100 * time.Millisecond
)

// Mutex 基于 Redis 的分布式互斥锁（单实例 SET NX PX 方案），不支持可重入。
// 一个 Mutex 实例代表一次加锁会话：Lock/TryLock 成功后必须调用 Unlock 才能再次加锁。
type Mutex struct {
	client        goredis.UniversalClient
	key           string
	ttl           time.Duration
	retryInterval time.Duration
	watchdog      bool

	mu     sync.Mutex
	token  string
	stopCh chan struct{}
}

// MutexOption 定义 Mutex 创建选项。
type MutexOption func(*mutexOptions)

type mutexOptions struct {
	ttl           time.Duration
	retryInterval time.Duration
	watchdog      bool
}

// WithTTL 设置锁的过期时间（必须 > 0，否则忽略，保留默认值）。
func WithTTL(ttl time.Duration) MutexOption {
	return func(o *mutexOptions) {
		if ttl > 0 {
			o.ttl = ttl
		}
	}
}

// WithRetryInterval 设置 Lock 阻塞重试的轮询间隔（必须 > 0，否则忽略）。
func WithRetryInterval(interval time.Duration) MutexOption {
	return func(o *mutexOptions) {
		if interval > 0 {
			o.retryInterval = interval
		}
	}
}

// WithWatchdog 设置是否启用续期 watchdog（默认 true）。
func WithWatchdog(enabled bool) MutexOption {
	return func(o *mutexOptions) {
		o.watchdog = enabled
	}
}

// NewMutex 创建分布式锁实例。
// 默认配置：ttl=10s，retryInterval=100ms，watchdog=true。
func NewMutex(client goredis.UniversalClient, key string, opts ...MutexOption) *Mutex {
	o := &mutexOptions{
		ttl:           defaultMutexTTL,
		retryInterval: defaultRetryInterval,
		watchdog:      true,
	}
	for _, opt := range opts {
		opt(o)
	}
	return &Mutex{
		client:        client,
		key:           key,
		ttl:           o.ttl,
		retryInterval: o.retryInterval,
		watchdog:      o.watchdog,
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-middleware/redis/... -run TestNewMutex -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-middleware/redis/lock.go go-middleware/redis/lock_test.go
git commit -m "feat(redis): add Mutex constructor and options (#56)"
```

---

### Task 3: TryLock（非阻塞加锁）

**Files:**
- Modify: `go-middleware/redis/lock.go`
- Modify: `go-middleware/redis/lock_test.go`

**Interfaces:**
- Consumes: Task 2 的 `Mutex` 结构体字段（`client`/`key`/`ttl`/`mu`/`token`）、`ErrLockAcquire`（Task 1）
- Produces: `func (m *Mutex) TryLock(ctx context.Context) (bool, error)`；内部 helper `func randomToken() (string, error)`。Task 4（Unlock）、Task 5（Lock）、Task 6（watchdog）依赖 `m.token` 由 `TryLock` 写入。

- [ ] **Step 1: 写失败测试**

在 `lock_test.go` 追加：

```go
func TestMutex_TryLock_Success(t *testing.T) {
	_, client := newTestRedisClient(t)
	m := NewMutex(client, "lock:trylock", WithWatchdog(false))

	ok, err := m.TryLock(context.Background())

	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NotEmpty(t, m.token)
}

func TestMutex_TryLock_AlreadyHeld(t *testing.T) {
	_, client := newTestRedisClient(t)
	first := NewMutex(client, "lock:contested", WithWatchdog(false))
	second := NewMutex(client, "lock:contested", WithWatchdog(false))

	ok1, err1 := first.TryLock(context.Background())
	require.NoError(t, err1)
	require.True(t, ok1)

	ok2, err2 := second.TryLock(context.Background())

	assert.NoError(t, err2)
	assert.False(t, ok2)
}
```

顶部 import 追加 `"context"` 和 `"github.com/stretchr/testify/require"`。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-middleware/redis/... -run TestMutex_TryLock -v`
Expected: FAIL（`TryLock` 未定义）

- [ ] **Step 3: 实现 TryLock**

在 `lock.go` 追加 import（`"context"`、`"crypto/rand"`、`"encoding/hex"`）并追加：

```go
func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// TryLock 单次非阻塞尝试获取锁。成功后若启用 watchdog 会自动启动续期。
func (m *Mutex) TryLock(ctx context.Context) (bool, error) {
	token, err := randomToken()
	if err != nil {
		return false, ErrLockAcquire.Wrap(err)
	}

	ok, err := m.client.SetNX(ctx, m.key, token, m.ttl).Result()
	if err != nil {
		return false, ErrLockAcquire.Wrap(err)
	}
	if !ok {
		return false, nil
	}

	m.mu.Lock()
	m.token = token
	m.mu.Unlock()

	if m.watchdog {
		m.startWatchdog()
	}
	return true, nil
}

// startWatchdog 在 Task 6 中实现；此处先占位为空操作以保证 TryLock 可编译独立测试。
func (m *Mutex) startWatchdog() {}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-middleware/redis/... -run TestMutex_TryLock -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-middleware/redis/lock.go go-middleware/redis/lock_test.go
git commit -m "feat(redis): implement Mutex.TryLock (#56)"
```

---

### Task 4: Unlock（Lua 原子释放）

**Files:**
- Modify: `go-middleware/redis/lock.go`
- Modify: `go-middleware/redis/lock_test.go`

**Interfaces:**
- Consumes: Task 3 的 `TryLock`、`m.token`、`ErrLockRelease`（Task 1）
- Produces: `func (m *Mutex) Unlock(ctx context.Context) error`；包级变量 `var releaseScript = goredis.NewScript(...)`。Task 6 的 watchdog 停止逻辑会替换本任务中 `Unlock` 里对 `stopWatchdog` 的占位调用。

- [ ] **Step 1: 写失败测试**

在 `lock_test.go` 追加：

```go
func TestMutex_Unlock_Success(t *testing.T) {
	_, client := newTestRedisClient(t)
	m := NewMutex(client, "lock:unlock", WithWatchdog(false))
	ctx := context.Background()

	ok, err := m.TryLock(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	err = m.Unlock(ctx)

	assert.NoError(t, err)

	exists, err := client.Exists(ctx, "lock:unlock").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists)
}

func TestMutex_Unlock_NotHeld(t *testing.T) {
	_, client := newTestRedisClient(t)
	m := NewMutex(client, "lock:notheld", WithWatchdog(false))

	err := m.Unlock(context.Background())

	assert.ErrorIs(t, err, ErrLockRelease)
}

func TestMutex_Unlock_HeldByOther(t *testing.T) {
	_, client := newTestRedisClient(t)
	owner := NewMutex(client, "lock:other", WithWatchdog(false))
	intruder := NewMutex(client, "lock:other", WithWatchdog(false))
	ctx := context.Background()

	ok, err := owner.TryLock(ctx)
	require.NoError(t, err)
	require.True(t, ok)
	intruder.mu.Lock()
	intruder.token = "not-the-real-token"
	intruder.mu.Unlock()

	err = intruder.Unlock(ctx)

	assert.ErrorIs(t, err, ErrLockRelease)

	exists, err := client.Exists(ctx, "lock:other").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists, "owner's lock must remain untouched")
}
```

> 注：`ErrLockRelease` 是 `*oops.OopsErrorBuilder`；`assert.ErrorIs` 依赖 `oops` 的 `Is`/`Unwrap` 实现（与现有 `client_test.go`/`config_test.go` 对 `ErrConnect` 的用法一致，此处沿用同样断言方式）。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-middleware/redis/... -run TestMutex_Unlock -v`
Expected: FAIL（`Unlock` 未定义）

- [ ] **Step 3: 实现 Unlock**

在 `lock.go` 顶部追加 import `"errors"`，包级追加：

```go
var releaseScript = goredis.NewScript(`
if redis.call('get', KEYS[1]) == ARGV[1] then
	return redis.call('del', KEYS[1])
else
	return 0
end
`)

// Unlock 释放锁：停止 watchdog，Lua 脚本校验 token 匹配后删除 key。
// 若锁未持有、已过期，或被其他持有者持有，返回 ErrLockRelease。
func (m *Mutex) Unlock(ctx context.Context) error {
	m.stopWatchdog()

	m.mu.Lock()
	token := m.token
	m.mu.Unlock()

	if token == "" {
		return ErrLockRelease.Wrap(errors.New("lock not held"))
	}

	res, err := releaseScript.Run(ctx, m.client, []string{m.key}, token).Int64()
	if err != nil {
		return ErrLockRelease.Wrap(err)
	}
	if res == 0 {
		return ErrLockRelease.Wrap(errors.New("lock not held or already expired"))
	}

	m.mu.Lock()
	m.token = ""
	m.mu.Unlock()
	return nil
}

// stopWatchdog 在 Task 6 中实现完整逻辑；此处先占位为空操作。
func (m *Mutex) stopWatchdog() {}
```

将 Task 3 遗留的 `func (m *Mutex) startWatchdog() {}` 占位保留不变（Task 6 会替换其实现）。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-middleware/redis/... -run TestMutex_Unlock -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-middleware/redis/lock.go go-middleware/redis/lock_test.go
git commit -m "feat(redis): implement Mutex.Unlock with Lua release script (#56)"
```

---

### Task 5: Lock（阻塞重试）

**Files:**
- Modify: `go-middleware/redis/lock.go`
- Modify: `go-middleware/redis/lock_test.go`

**Interfaces:**
- Consumes: Task 3 的 `TryLock`、`ErrLockAcquire`（Task 1）、`m.retryInterval`（Task 2）
- Produces: `func (m *Mutex) Lock(ctx context.Context) error`

- [ ] **Step 1: 写失败测试**

在 `lock_test.go` 追加：

```go
func TestMutex_Lock_WaitsForRelease(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	holder := NewMutex(client, "lock:wait", WithWatchdog(false), WithRetryInterval(10*time.Millisecond))
	waiter := NewMutex(client, "lock:wait", WithWatchdog(false), WithRetryInterval(10*time.Millisecond))

	ok, err := holder.TryLock(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	done := make(chan error, 1)
	go func() {
		done <- waiter.Lock(ctx)
	}()

	time.Sleep(30 * time.Millisecond)
	require.NoError(t, holder.Unlock(ctx))

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("waiter.Lock did not return after holder released the lock")
	}
}

func TestMutex_Lock_ContextCanceled(t *testing.T) {
	_, client := newTestRedisClient(t)
	holder := NewMutex(client, "lock:cancel", WithWatchdog(false))
	waiter := NewMutex(client, "lock:cancel", WithWatchdog(false), WithRetryInterval(10*time.Millisecond))

	ok, err := holder.TryLock(context.Background())
	require.NoError(t, err)
	require.True(t, ok)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	err = waiter.Lock(ctx)

	assert.ErrorIs(t, err, ErrLockAcquire)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-middleware/redis/... -run TestMutex_Lock -v`
Expected: FAIL（`Lock` 未定义）

- [ ] **Step 3: 实现 Lock**

在 `lock.go` 追加：

```go
// Lock 阻塞获取锁，直到成功或 ctx 取消。成功后自动启动 watchdog（若启用）。
func (m *Mutex) Lock(ctx context.Context) error {
	ticker := time.NewTicker(m.retryInterval)
	defer ticker.Stop()

	for {
		ok, err := m.TryLock(ctx)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}

		select {
		case <-ctx.Done():
			return ErrLockAcquire.Wrap(ctx.Err())
		case <-ticker.C:
		}
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-middleware/redis/... -run TestMutex_Lock -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-middleware/redis/lock.go go-middleware/redis/lock_test.go
git commit -m "feat(redis): implement Mutex.Lock blocking retry (#56)"
```

---

### Task 6: Watchdog 续期

**Files:**
- Modify: `go-middleware/redis/lock.go`
- Modify: `go-middleware/redis/lock_test.go`

**Interfaces:**
- Consumes: Task 3/4 中 `startWatchdog`/`stopWatchdog` 占位、`m.stopCh`（Task 2 已声明字段）
- Produces: `startWatchdog`/`stopWatchdog` 的完整实现；包级变量 `var renewScript = goredis.NewScript(...)`

- [ ] **Step 1: 写失败测试**

在 `lock_test.go` 追加（使用 `miniredis.FastForward` 模拟时间流逝，验证续期后锁未过期；以及关闭 watchdog 时锁按 TTL 正常过期）：

```go
func TestMutex_Watchdog_RenewsBeforeExpiry(t *testing.T) {
	mr, client := newTestRedisClient(t)
	ctx := context.Background()
	m := NewMutex(client, "lock:watchdog", WithTTL(90*time.Millisecond), WithWatchdog(true))

	ok, err := m.TryLock(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	// 90ms TTL，续期间隔 = ttl/3 = 30ms。累计推进 200ms（> 原始 TTL），
	// 若续期生效，key 应仍然存在。
	for i := 0; i < 10; i++ {
		mr.FastForward(20 * time.Millisecond)
		time.Sleep(5 * time.Millisecond) // 让 watchdog goroutine 有机会真实执行一次续期
	}

	exists, err := client.Exists(ctx, "lock:watchdog").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists, "watchdog should have renewed the lock before its original TTL")

	require.NoError(t, m.Unlock(ctx))
}

func TestMutex_Watchdog_StopsAfterUnlock(t *testing.T) {
	mr, client := newTestRedisClient(t)
	ctx := context.Background()
	m := NewMutex(client, "lock:watchdog-stop", WithTTL(50*time.Millisecond), WithWatchdog(true))

	ok, err := m.TryLock(ctx)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, m.Unlock(ctx))

	mr.FastForward(200 * time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	exists, err := client.Exists(ctx, "lock:watchdog-stop").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists, "key must not be recreated after Unlock stopped the watchdog")
}

func TestMutex_Watchdog_Disabled_LockExpires(t *testing.T) {
	mr, client := newTestRedisClient(t)
	ctx := context.Background()
	m := NewMutex(client, "lock:no-watchdog", WithTTL(50*time.Millisecond), WithWatchdog(false))

	ok, err := m.TryLock(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	mr.FastForward(60 * time.Millisecond)

	exists, err := client.Exists(ctx, "lock:no-watchdog").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-middleware/redis/... -run TestMutex_Watchdog -v`
Expected: FAIL（`TestMutex_Watchdog_RenewsBeforeExpiry` 失败：key 已过期，因为 `startWatchdog`/`stopWatchdog` 仍是空实现）

- [ ] **Step 3: 实现 watchdog**

在 `lock.go` 中，将 Task 3 的 `func (m *Mutex) startWatchdog() {}` 和 Task 4 的 `func (m *Mutex) stopWatchdog() {}` 占位整体替换为：

```go
var renewScript = goredis.NewScript(`
if redis.call('get', KEYS[1]) == ARGV[1] then
	return redis.call('pexpire', KEYS[1], ARGV[2])
else
	return 0
end
`)

// startWatchdog 启动后台续期 goroutine：按 ttl/3 周期执行 Lua 续期脚本，
// 直到 stopWatchdog 被调用或续期失败（锁已丢失）。
func (m *Mutex) startWatchdog() {
	stop := make(chan struct{})
	m.mu.Lock()
	m.stopCh = stop
	m.mu.Unlock()

	interval := m.ttl / 3
	if interval <= 0 {
		interval = time.Millisecond
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				m.mu.Lock()
				token := m.token
				m.mu.Unlock()
				if token == "" {
					return
				}

				res, err := renewScript.Run(context.Background(), m.client, []string{m.key}, token, m.ttl.Milliseconds()).Int64()
				if err != nil || res == 0 {
					return
				}
			}
		}
	}()
}

// stopWatchdog 停止续期 goroutine（幂等，可安全重复调用）。
func (m *Mutex) stopWatchdog() {
	m.mu.Lock()
	stop := m.stopCh
	m.stopCh = nil
	m.mu.Unlock()

	if stop != nil {
		close(stop)
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-middleware/redis/... -run TestMutex -v`
Expected: PASS（全部 Mutex 相关用例，包括 Task 2-6）

- [ ] **Step 5: Commit**

```bash
git add go-middleware/redis/lock.go go-middleware/redis/lock_test.go
git commit -m "feat(redis): implement Mutex watchdog renewal (#56)"
```

---

### Task 7: Limiter 骨架 + Options（构造器）

**Files:**
- Create: `go-middleware/redis/limiter.go`
- Create: `go-middleware/redis/limiter_test.go`

**Interfaces:**
- Consumes: `redis.UniversalClient`
- Produces：
  - `type Limiter struct { client redis.UniversalClient; key string; rate float64; burst int; waitPollInterval time.Duration }`
  - `func NewLimiter(client redis.UniversalClient, key string, r float64, burst int, opts ...LimiterOption) *Limiter`
  - `type LimiterOption func(*limiterOptions)`
  - `func WithWaitPollInterval(interval time.Duration) LimiterOption`
  - Task 8-9 在此文件追加 `Allow`/`AllowN`/`Wait` 方法

- [ ] **Step 1: 写失败测试**

创建 `go-middleware/redis/limiter_test.go`：

```go
package redis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewLimiter_Defaults(t *testing.T) {
	_, client := newTestRedisClient(t)

	l := NewLimiter(client, "limiter:test", 10, 5)

	assert.Equal(t, "limiter:test", l.key)
	assert.InDelta(t, 10.0, l.rate, 0.0001)
	assert.Equal(t, 5, l.burst)
	assert.Equal(t, defaultWaitPollInterval, l.waitPollInterval)
}

func TestNewLimiter_CustomOptions(t *testing.T) {
	_, client := newTestRedisClient(t)

	l := NewLimiter(client, "limiter:test", 10, 5, WithWaitPollInterval(200*time.Millisecond))

	assert.Equal(t, 200*time.Millisecond, l.waitPollInterval)
}

func TestLimiterOption_IgnoresInvalidValues(t *testing.T) {
	_, client := newTestRedisClient(t)

	l := NewLimiter(client, "limiter:test", 10, 5, WithWaitPollInterval(0))

	assert.Equal(t, defaultWaitPollInterval, l.waitPollInterval)
}
```

（`newTestRedisClient` 复用 Task 2 在 `lock_test.go` 中定义的 helper，同包内可直接调用。）

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-middleware/redis/... -run TestNewLimiter -v`
Expected: FAIL（`limiter.go` 不存在）

- [ ] **Step 3: 实现 Limiter 骨架**

创建 `go-middleware/redis/limiter.go`：

```go
package redis

import (
	"time"

	goredis "github.com/redis/go-redis/v9"
)

const defaultWaitPollInterval = 50 * time.Millisecond

// Limiter 基于 Redis 的分布式令牌桶限流器。
// 方法命名对齐 golang.org/x/time/rate.Limiter（Allow/AllowN/Wait），
// 但因跨网络调用 Redis，签名带 ctx 且返回 error。
type Limiter struct {
	client           goredis.UniversalClient
	key              string
	rate             float64
	burst            int
	waitPollInterval time.Duration
}

// LimiterOption 定义 Limiter 创建选项。
type LimiterOption func(*limiterOptions)

type limiterOptions struct {
	waitPollInterval time.Duration
}

// WithWaitPollInterval 设置 Wait 轮询重试间隔（必须 > 0，否则忽略）。
func WithWaitPollInterval(interval time.Duration) LimiterOption {
	return func(o *limiterOptions) {
		if interval > 0 {
			o.waitPollInterval = interval
		}
	}
}

// NewLimiter 创建分布式限流器。r 为每秒生成令牌数，burst 为桶容量。
// 默认配置：waitPollInterval=50ms。
func NewLimiter(client goredis.UniversalClient, key string, r float64, burst int, opts ...LimiterOption) *Limiter {
	o := &limiterOptions{waitPollInterval: defaultWaitPollInterval}
	for _, opt := range opts {
		opt(o)
	}
	return &Limiter{
		client:           client,
		key:              key,
		rate:             r,
		burst:            burst,
		waitPollInterval: o.waitPollInterval,
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-middleware/redis/... -run TestNewLimiter -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-middleware/redis/limiter.go go-middleware/redis/limiter_test.go
git commit -m "feat(redis): add Limiter constructor and options (#56)"
```

---

### Task 8: AllowN/Allow（令牌桶 Lua 脚本）

**Files:**
- Modify: `go-middleware/redis/limiter.go`
- Modify: `go-middleware/redis/limiter_test.go`

**Interfaces:**
- Consumes: Task 7 的 `Limiter` 字段、`ErrLimiterEval`（Task 1）
- Produces: `func (l *Limiter) AllowN(ctx context.Context, n int) (bool, error)`、`func (l *Limiter) Allow(ctx context.Context) (bool, error)`；内部 helper `func (l *Limiter) ttlMillis() int64`；包级变量 `var tokenBucketScript = goredis.NewScript(...)`。Task 9（Wait）依赖 `Allow`。

- [ ] **Step 1: 写失败测试**

在 `limiter_test.go` 追加：

```go
func TestLimiter_AllowN_WithinBurst(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	l := NewLimiter(client, "limiter:burst", 1, 3)

	for i := 0; i < 3; i++ {
		ok, err := l.Allow(ctx)
		require.NoError(t, err)
		assert.True(t, ok, "request %d within burst should be allowed", i)
	}

	ok, err := l.Allow(ctx)
	require.NoError(t, err)
	assert.False(t, ok, "request beyond burst should be rejected")
}

func TestLimiter_AllowN_RejectsWhenInsufficientTokens(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	l := NewLimiter(client, "limiter:allown", 1, 5)

	ok, err := l.AllowN(ctx, 3)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = l.AllowN(ctx, 3)
	require.NoError(t, err)
	assert.False(t, ok, "only 2 tokens remain, requesting 3 must fail")
}

func TestLimiter_Refill_OverTime(t *testing.T) {
	mr, client := newTestRedisClient(t)
	ctx := context.Background()
	l := NewLimiter(client, "limiter:refill", 10, 1) // 10 tokens/sec, burst=1

	ok, err := l.Allow(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = l.Allow(ctx)
	require.NoError(t, err)
	require.False(t, ok, "bucket should be empty immediately after consuming the single token")

	mr.FastForward(200 * time.Millisecond) // 10 tokens/sec * 0.2s = 2 tokens, capped at burst=1

	ok, err = l.Allow(ctx)
	require.NoError(t, err)
	assert.True(t, ok, "token should have refilled after 200ms at rate=10/s")
}
```

顶部 import 追加 `"context"` 和 `"github.com/stretchr/testify/require"`。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-middleware/redis/... -run TestLimiter -v`
Expected: FAIL（`AllowN`/`Allow` 未定义）

- [ ] **Step 3: 实现 AllowN/Allow**

在 `limiter.go` 顶部追加 import `"context"`，包级追加：

```go
var tokenBucketScript = goredis.NewScript(`
local tokens_key = KEYS[1]
local rate = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])
local ttl = tonumber(ARGV[5])

local data = redis.call('hmget', tokens_key, 'tokens', 'last_refill_ts')
local tokens = tonumber(data[1])
local last_refill_ts = tonumber(data[2])

if tokens == nil then
	tokens = burst
	last_refill_ts = now
end

local elapsed = math.max(0, now - last_refill_ts)
local refill = (elapsed / 1000) * rate
tokens = math.min(burst, tokens + refill)

local allowed = 0
if tokens >= requested then
	tokens = tokens - requested
	allowed = 1
end

redis.call('hmset', tokens_key, 'tokens', tokens, 'last_refill_ts', now)
redis.call('pexpire', tokens_key, ttl)

return allowed
`)

// Allow 等价于 AllowN(ctx, 1)。
func (l *Limiter) Allow(ctx context.Context) (bool, error) {
	return l.AllowN(ctx, 1)
}

// AllowN 尝试消耗 n 个令牌，返回是否成功。
func (l *Limiter) AllowN(ctx context.Context, n int) (bool, error) {
	now := time.Now().UnixMilli()
	res, err := tokenBucketScript.Run(ctx, l.client, []string{l.key}, l.rate, l.burst, now, n, l.ttlMillis()).Int64()
	if err != nil {
		return false, ErrLimiterEval.Wrap(err)
	}
	return res == 1, nil
}

// ttlMillis 计算 bucket key 的过期时间（约为完全补满所需时间的 2 倍，至少 1 秒），
// 避免闲置 key 常驻内存。
func (l *Limiter) ttlMillis() int64 {
	seconds := 2 * (float64(l.burst) / l.rate)
	if seconds < 1 {
		seconds = 1
	}
	return int64(seconds * 1000)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-middleware/redis/... -run TestLimiter -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-middleware/redis/limiter.go go-middleware/redis/limiter_test.go
git commit -m "feat(redis): implement Limiter token bucket via Lua script (#56)"
```

---

### Task 9: Wait（阻塞等待令牌）

**Files:**
- Modify: `go-middleware/redis/limiter.go`
- Modify: `go-middleware/redis/limiter_test.go`

**Interfaces:**
- Consumes: Task 8 的 `Allow`、`l.waitPollInterval`（Task 7）
- Produces: `func (l *Limiter) Wait(ctx context.Context) error`

- [ ] **Step 1: 写失败测试**

在 `limiter_test.go` 追加：

```go
func TestLimiter_Wait_SucceedsAfterRefill(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	l := NewLimiter(client, "limiter:wait", 100, 1, WithWaitPollInterval(5*time.Millisecond)) // fast refill for test speed

	ok, err := l.Allow(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	waitCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	err = l.Wait(waitCtx)

	assert.NoError(t, err, "Wait should succeed once tokens refill at rate=100/s")
}

func TestLimiter_Wait_ContextTimeout(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	l := NewLimiter(client, "limiter:wait-timeout", 0.001, 1, WithWaitPollInterval(5*time.Millisecond)) // effectively never refills within test window

	ok, err := l.Allow(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel()

	err = l.Wait(waitCtx)

	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-middleware/redis/... -run TestLimiter_Wait -v`
Expected: FAIL（`Wait` 未定义）

- [ ] **Step 3: 实现 Wait**

在 `limiter.go` 追加：

```go
// Wait 阻塞直到成功获取 1 个令牌，或 ctx 取消/超时返回其错误。
func (l *Limiter) Wait(ctx context.Context) error {
	for {
		ok, err := l.Allow(ctx)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(l.waitPollInterval):
		}
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-middleware/redis/... -run TestLimiter -v`
Expected: PASS（全部 Limiter 相关用例，包括 Task 7-9）

- [ ] **Step 5: Commit**

```bash
git add go-middleware/redis/limiter.go go-middleware/redis/limiter_test.go
git commit -m "feat(redis): implement Limiter.Wait (#56)"
```

---

### Task 10: 全量验证（build / vet / lint / test）

**Files:**
- None (verification only)

**Interfaces:**
- Consumes: Task 1-9 的全部产出
- Produces: 无新代码；确认整个 `go-middleware` 模块可构建、通过静态检查与测试

- [ ] **Step 1: 格式检查**

Run: `gofmt -l go-middleware/redis/`
Expected: 无输出（无需格式化的文件）

- [ ] **Step 2: 构建**

Run: `go build ./go-middleware/...`
Expected: 无错误退出

- [ ] **Step 3: vet**

Run: `go vet ./go-middleware/...`
Expected: 无错误退出

- [ ] **Step 4: lint**

Run: `golangci-lint run --timeout=5m ./go-middleware/...`
Expected: 无 issue（若报 `errcheck`/`revive` 等问题，按 `.claude/rules/go.md` §8 逐条修复后重跑本步骤）

- [ ] **Step 5: 完整测试**

Run: `go test ./go-middleware/... -count=1 -race`
Expected: 全部 PASS（含 `-race`，验证 watchdog goroutine 与 `sync.Mutex` 保护的并发安全性）

- [ ] **Step 6: Commit（若 lint/fix 产生改动）**

```bash
git add -A
git commit -m "chore(redis): fix lint findings for lock/limiter (#56)"
```

若 Step 1-5 全部无改动即通过，跳过本步骤（无空提交）。

---

## Self-Review Notes（编写计划时的自查）

- **Spec 覆盖**：设计文档中的 Mutex 结构/Options/加锁/释放/watchdog（Task 2-6）、Limiter 结构/Options/AllowN/Allow/Wait（Task 7-9）、错误码 20103-20105（Task 1）、测试覆盖点（并发竞争 Task 3、非持有者释放 Task 4、watchdog 续期 Task 6、令牌补充 Task 8、Wait 阻塞 Task 9）均有对应任务；README 因包内无独立 README，已在设计文档中明确以 godoc 注释为准（`lock.go`/`limiter.go` 包级类型注释已包含用法说明），不再单列任务。
- **类型一致性**：`Mutex`/`Limiter` 字段名、`MutexOption`/`LimiterOption`、`WithTTL`/`WithRetryInterval`/`WithWatchdog`/`WithWaitPollInterval` 在各任务间签名一致；`TryLock`/`Lock`/`Unlock`/`Allow`/`AllowN`/`Wait` 的参数与返回类型在跨任务引用处（Interfaces 小节）保持一致。
- **占位符扫描**：所有 Step 均给出完整可运行代码，无 TBD/TODO。
