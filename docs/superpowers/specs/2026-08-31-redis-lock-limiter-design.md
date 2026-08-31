# Redis 分布式锁与限流器封装 — 设计文档

- Issue: [#56](https://github.com/byx-darwin/go-tools/issues/56)
- 模块：`go-middleware/redis`
- 日期：2026-08-31

## 背景

`go-middleware/redis` 目前只提供裸的 `redis.UniversalClient` 连接层封装（`NewUniversalClient`/`NewClient`，支持单节点/哨兵/集群、TLS、OTel 追踪）。作为微服务脚手架的中间件基础库，缺少两类最常见的业务级用法封装：分布式锁、限流器。业务项目各自实现容易出现不一致甚至有 bug 的版本（锁的可重入性/续期、限流的原子性都需要 Lua 脚本或恰当的 Redis 命令组合）。

## 目标

在 `go-middleware/redis` 包内新增分布式锁（`Mutex`）和限流器（`Limiter`）封装，作为基于 `redis.UniversalClient` 的上层工具。

## 分布式锁（Mutex）

**文件**：`go-middleware/redis/lock.go`

### 选型结论

- **单实例 `SET NX PX` + Lua 释放脚本**，不做 Redlock（多实例）。理由：Redlock 的正确性在社区一直有争议（Martin Kleppmann 的批评），微服务场景下单实例锁配合哨兵/集群的高可用已经足够，多实例方案的实现复杂度和运维成本收益有限。
- **需要 watchdog 续期**：后台 goroutine 按 `ttl/3` 周期执行 Lua 续期脚本（校验持有者 token 后 `PEXPIRE`），防止业务处理时间超过锁 TTL 导致锁提前释放。`Unlock` 或 `ctx` 取消时停止续期。
- **API 形态**：返回 `Mutex` 对象（`Lock`/`TryLock`/`Unlock`），不提供函数式 `WithLock` 包装（YAGNI，调用方可自行用 `defer` 包装）。

### 结构

```go
// Mutex 基于 Redis 的分布式互斥锁，不支持可重入。
type Mutex struct {
    client        redis.UniversalClient
    key           string
    ttl           time.Duration
    retryInterval time.Duration
    watchdog      bool

    mu     sync.Mutex // 保护 token/stopCh 等内部可变状态
    token  string     // 每次 Lock 成功后生成的随机持有者标识
    stopCh chan struct{}
}

// MutexOption 定义 Mutex 创建选项。
type MutexOption func(*mutexOptions)

// NewMutex 创建分布式锁实例。
// 默认配置：ttl=10s，retryInterval=100ms，watchdog=true。
func NewMutex(client redis.UniversalClient, key string, opts ...MutexOption) *Mutex

// WithTTL 设置锁的过期时间（必须 > 0）。
func WithTTL(ttl time.Duration) MutexOption

// WithRetryInterval 设置 Lock 阻塞重试的轮询间隔（必须 > 0）。
func WithRetryInterval(interval time.Duration) MutexOption

// WithWatchdog 设置是否启用续期 watchdog（默认 true）。
func WithWatchdog(enabled bool) MutexOption

// Lock 阻塞获取锁，直到成功或 ctx 取消。成功后自动启动 watchdog（若启用）。
func (m *Mutex) Lock(ctx context.Context) error

// TryLock 单次非阻塞尝试获取锁。
func (m *Mutex) TryLock(ctx context.Context) (bool, error)

// Unlock 释放锁：停止 watchdog，Lua 脚本校验 token 匹配后删除 key。
// 若锁已过期或被其他持有者持有，返回 ErrLockRelease。
func (m *Mutex) Unlock(ctx context.Context) error
```

### 机制细节

- **加锁**：`SET key token NX PX <ttl_ms>`；`token` 用 `crypto/rand` 生成随机字符串（每次 `Lock`/`TryLock` 成功时重新生成）。
- **释放**（Lua，原子）：
  ```lua
  if redis.call('get', KEYS[1]) == ARGV[1] then
      return redis.call('del', KEYS[1])
  else
      return 0
  end
  ```
  返回 0 表示未持有或已过期，映射为 `ErrLockRelease`。
- **续期**（Lua，原子，watchdog goroutine 周期调用）：
  ```lua
  if redis.call('get', KEYS[1]) == ARGV[1] then
      return redis.call('pexpire', KEYS[1], ARGV[2])
  else
      return 0
  end
  ```
  续期失败（返回 0，说明锁已丢失）时 watchdog 自行退出，不重试。
- **`Lock` 重试**：轮询 `TryLock`，按 `retryInterval` 间隔重试，直到成功或 `ctx.Done()`。
- 不支持可重入：一个 `Mutex` 实例代表一次加锁会话，重复调用 `Lock` 前需先 `Unlock`。

## 限流器（Limiter）

**文件**：`go-middleware/redis/limiter.go`

### 选型结论

- **令牌桶算法**，不做滑动窗口。理由：令牌桶允许突发流量（burst）且平滑限制平均速率，实现开销（Redis Hash 存两个字段）远低于滑动窗口（每请求一条记录），且语义与 Go 标准库 `golang.org/x/time/rate` 一致，便于业务从本地限流器迁移到分布式限流器。
- **API 对齐 `golang.org/x/time/rate.Limiter`** 的方法命名（`Allow`/`AllowN`/`Wait`），但因跨网络调用 Redis，签名带 `ctx` 且返回 `error`（本地版本没有）。

### 结构

```go
// Limiter 基于 Redis 的分布式令牌桶限流器。
type Limiter struct {
    client           redis.UniversalClient
    key              string
    rate             float64 // 每秒生成令牌数
    burst            int     // 桶容量
    waitPollInterval time.Duration
}

// LimiterOption 定义 Limiter 创建选项。
type LimiterOption func(*limiterOptions)

// NewLimiter 创建分布式限流器。r 为每秒生成令牌数，burst 为桶容量。
// 默认配置：waitPollInterval=50ms。
func NewLimiter(client redis.UniversalClient, key string, r float64, burst int, opts ...LimiterOption) *Limiter

// WithWaitPollInterval 设置 Wait 轮询重试间隔（必须 > 0）。
func WithWaitPollInterval(interval time.Duration) LimiterOption

// Allow 等价于 AllowN(ctx, 1)。
func (l *Limiter) Allow(ctx context.Context) (bool, error)

// AllowN 尝试消耗 n 个令牌，返回是否成功。
func (l *Limiter) AllowN(ctx context.Context, n int) (bool, error)

// Wait 阻塞直到成功获取 1 个令牌或 ctx 取消/超时。
func (l *Limiter) Wait(ctx context.Context) error
```

### 机制细节

- **状态存储**：Redis Hash（`key`）存 `tokens`（当前令牌数）、`last_refill_ts`（毫秒时间戳）。
- **Lua 脚本**（原子）：
  1. 读取 `tokens`/`last_refill_ts`，若不存在则初始化为 `tokens=burst, last_refill_ts=now`
  2. `elapsed = now - last_refill_ts`；`refill = elapsed/1000 * rate`；`tokens = min(burst, tokens + refill)`
  3. 若 `tokens >= n`：`tokens -= n`，写回并 `PEXPIRE key <ttl>`，返回 1（成功）
  4. 否则：写回补充后的 `tokens`（不写回则下次计算 elapsed 会重复补充，需持久化），`PEXPIRE`，返回 0（失败）
- **Key TTL**：每次调用后设置为 `2 * (burst / rate) * 1000` 毫秒（向上取整，至少 1s），避免闲置 key 常驻内存。
- **`Wait`**：内部循环调用 `AllowN(ctx, 1)`，失败则 `time.Sleep(waitPollInterval)`（响应 `ctx.Done()`），直到成功或 ctx 取消返回其错误。

## 错误码

延续 `go-middleware/redis/errors.go` 现有编号（20101-20102 已用于连接/埋点），新增：

```go
const (
    CodeLockAcquire = 20103 // 加锁失败（Lock 因 ctx 取消退出）
    CodeLockRelease = 20104 // Unlock 时未持有锁（token 不匹配/已过期）或 Redis 操作失败
    CodeLimiterEval = 20105 // 限流器 Lua 脚本执行失败
)

var (
    ErrLockAcquire = goerror.Code(CodeLockAcquire).Public("redis_lock_acquire_error")
    ErrLockRelease = goerror.Code(CodeLockRelease).Public("redis_lock_release_error")
    ErrLimiterEval = goerror.Code(CodeLimiterEval).Public("redis_limiter_eval_error")
)
```

HTTP 状态码映射沿用 `init()` 中 `RegisterHTTPStatuses` 模式（503/500，与现有 `CodeConnect`/`CodeInstrument` 一致的量级）。

## 测试

`go-middleware/redis/lock_test.go`、`go-middleware/redis/limiter_test.go`，复用 `go-middleware/auth` 的 `miniredis.RunT` 测试模式：

**Mutex**
- `TryLock`/`Lock` 单实例加锁成功、`Unlock` 成功释放
- 并发竞争：两个 `Mutex` 实例对同一 key，只有一方 `TryLock` 成功
- 非持有者 `Unlock` 返回 `ErrLockRelease`
- watchdog 续期：`miniredis.FastForward` 超过初始 TTL 后锁仍被持有（验证续期生效）；`WithWatchdog(false)` 时锁会按 TTL 过期
- `Lock` 在锁被占用时阻塞重试，锁释放后成功获取；`ctx` 取消时 `Lock` 及时返回

**Limiter**
- `Allow`/`AllowN` 在 burst 范围内成功，超出后返回 `false`
- `miniredis.FastForward` 模拟时间流逝后令牌按 `rate` 补充
- `Wait` 在令牌不足时阻塞，令牌可用后返回；`ctx` 超时时返回其错误

## 文档

`go-middleware/redis` 目前无独立 README，用法说明以 `lock.go`/`limiter.go` 包级 godoc 注释（含用法示例）为准，风格与 `client.go` 一致。

## 范围外（Out of Scope）

- Redlock（多实例锁）— 已决策不做
- 滑动窗口/固定窗口限流算法 — 已决策不做，仅实现令牌桶
- 可重入锁语义
