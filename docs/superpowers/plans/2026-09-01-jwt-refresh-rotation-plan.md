# Refresh Token 轮换与复用检测 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `jwt.Refresh` 在成功刷新后撤销旧 JTI，并在同一旧 JTI 被再次用于 Refresh 时判定为复用攻击并拒绝签发新 token。

**Architecture:** `jwt.Refresh` 新增 `ctx context.Context` 与必选 `store revocation.Store` 参数；成功路径下检查/撤销旧 JTI 并为新 token 生成新 JTI（`google/uuid`）。`go-middleware/auth` 新增 `MemoryRevocationStore`（此前只有 Redis 实现），供 example 与测试使用。example 的 `jwtRefreshHandler` 与 `main.go` 接线更新以配合新签名。

**Tech Stack:** Go 1.25 workspace（`go.work`）、`golang-jwt/jwt/v5`、`samber/oops`、`samber/hot`、`google/uuid`、`testify`。

**Spec:** `docs/superpowers/specs/2026-09-01-jwt-refresh-rotation-design.md`

## Global Constraints

- `go-auth` 模块只能 import `go-common`，不得 import `go-middleware`/`go-framework`（`.claude/rules/go.md` §4）。`google/uuid` 是叶子依赖，不违反该约束。
- `go-middleware` 可以 import `go-auth` + `go-common`，不得 import `go-framework`。
- 所有导出符号必须有 `// Name ...` 格式的 godoc 注释（revive linter，`.claude/rules/go.md` §8.3）。
- 错误返回值必须处理：检查、`_ =` 显式忽略、或 `require.NoError`（errcheck，`.claude/rules/go.md` §8.4）。
- 使用 `any` 而非 `interface{}`（`.claude/rules/go.md` §8.5）。
- 保持现有代码风格：`oops.With(...).Code(...).Wrap(err)` 错误包装模式、`gofmt` 干净、模块内 import 分三组（标准库/第三方/本项目）。

---

## Task 1: go-auth 新增 `google/uuid` 依赖并实现 Refresh 轮换与复用检测

**Files:**
- Modify: `go-auth/go.mod`（新增直接依赖）
- Modify: `go-auth/jwt/token.go:160-177`（`Refresh` 函数签名与实现）
- Modify: `go-auth/jwt/options.go:22`（包注释用法示例）
- Modify: `go-auth/jwt/token_test.go`（更新现有 `Refresh` 调用点 + 新增复用检测测试）

**Interfaces:**
- Consumes: `revocation.Store`（已存在，`go-auth/revocation/checker.go`）：
  ```go
  type Checker interface {
      IsRevoked(ctx context.Context, jti string) (bool, error)
  }
  type Revoker interface {
      Revoke(ctx context.Context, jti string, ttl time.Duration) error
  }
  type Store interface {
      Checker
      Revoker
  }
  ```
  `ExtractJTI(claims any) (string, bool)`、`extractRegisteredClaims(claims gojwt.Claims) *gojwt.RegisteredClaims`（包内已有，无需新建）。
  `autherror.ErrTokenRevoked`、`autherror.CodeJWTRefreshFailed`（`go-auth/error/error.go`，已存在）。
- Produces: 新的公开签名
  ```go
  func Refresh[T any](ctx context.Context, tokenStr string, secret any, store revocation.Store, opts ...Option) (string, error)
  ```
  供 Task 3（example 接线）使用。

- [ ] **Step 1: 添加 `google/uuid` 依赖**

```bash
cd go-auth && go get github.com/google/uuid@v1.6.0 && cd ..
```

- [ ] **Step 2: 写失败测试 — 复用检测**

在 `go-auth/jwt/token_test.go` 文件末尾追加以下内容（新增 fake store + 测试用例）：

```go
// ── Refresh 轮换与复用检测 ──

// fakeRevocationStore 是 revocation.Store 的内存测试替身，避免 go-auth 反向依赖
// go-middleware。
type fakeRevocationStore struct {
	revoked map[string]bool

	isRevokedErr error
	revokeErr    error
}

func newFakeRevocationStore() *fakeRevocationStore {
	return &fakeRevocationStore{revoked: make(map[string]bool)}
}

func (s *fakeRevocationStore) IsRevoked(_ context.Context, jti string) (bool, error) {
	if s.isRevokedErr != nil {
		return false, s.isRevokedErr
	}
	return s.revoked[jti], nil
}

func (s *fakeRevocationStore) Revoke(_ context.Context, jti string, _ time.Duration) error {
	if s.revokeErr != nil {
		return s.revokeErr
	}
	s.revoked[jti] = true
	return nil
}

type refreshRotationClaims struct {
	UserUUID string `json:"user_uuid"`
	gojwt.RegisteredClaims
}

func TestRefreshRotatesJTI(t *testing.T) {
	store := newFakeRevocationStore()
	claims := refreshRotationClaims{
		UserUUID:         "user-rotate",
		RegisteredClaims: gojwt.RegisteredClaims{ID: "jti-original"},
	}

	token, err := Sign(claims, testSecret, WithExpiration(30*time.Minute))
	require.NoError(t, err)

	newToken, err := Refresh[refreshRotationClaims](context.Background(), token, testSecret, store, WithExpiration(time.Hour))
	require.NoError(t, err)
	assert.NotEmpty(t, newToken)

	parsed, err := Verify[refreshRotationClaims](newToken, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "user-rotate", parsed.UserUUID)
	assert.NotEqual(t, "jti-original", parsed.ID, "刷新后必须生成新 JTI")
	assert.True(t, store.revoked["jti-original"], "旧 JTI 必须被标记为已撤销")
}

func TestRefreshReuseDetection(t *testing.T) {
	store := newFakeRevocationStore()
	claims := refreshRotationClaims{
		UserUUID:         "user-reuse",
		RegisteredClaims: gojwt.RegisteredClaims{ID: "jti-reuse"},
	}

	token, err := Sign(claims, testSecret, WithExpiration(30*time.Minute))
	require.NoError(t, err)

	_, err = Refresh[refreshRotationClaims](context.Background(), token, testSecret, store, WithExpiration(time.Hour))
	require.NoError(t, err)

	// 用同一个旧 token 再次 Refresh：旧 JTI 已被撤销，必须判定为复用攻击。
	_, err = Refresh[refreshRotationClaims](context.Background(), token, testSecret, store, WithExpiration(time.Hour))
	require.Error(t, err)

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeTokenRevoked, code)
}

func TestRefreshWithoutJTISkipsRotation(t *testing.T) {
	// Claims 未携带 JTI 时，行为与变更前一致：不触碰 store，不报错。
	store := newFakeRevocationStore()
	claims := UserClaims{UserUUID: "user-no-jti"}

	token, err := Sign(claims, testSecret, WithExpiration(30*time.Minute))
	require.NoError(t, err)

	newToken, err := Refresh[UserClaims](context.Background(), token, testSecret, store, WithExpiration(time.Hour))
	require.NoError(t, err)
	assert.NotEmpty(t, newToken)
	assert.Empty(t, store.revoked)
}

func TestRefreshIsRevokedError(t *testing.T) {
	store := newFakeRevocationStore()
	store.isRevokedErr = assert.AnError
	claims := refreshRotationClaims{
		UserUUID:         "user-isrevoked-err",
		RegisteredClaims: gojwt.RegisteredClaims{ID: "jti-isrevoked-err"},
	}

	token, err := Sign(claims, testSecret, WithExpiration(30*time.Minute))
	require.NoError(t, err)

	_, err = Refresh[refreshRotationClaims](context.Background(), token, testSecret, store, WithExpiration(time.Hour))
	require.Error(t, err, "IsRevoked 出错必须 fail-closed，拒绝刷新")
}

func TestRefreshRevokeError(t *testing.T) {
	store := newFakeRevocationStore()
	store.revokeErr = assert.AnError
	claims := refreshRotationClaims{
		UserUUID:         "user-revoke-err",
		RegisteredClaims: gojwt.RegisteredClaims{ID: "jti-revoke-err"},
	}

	token, err := Sign(claims, testSecret, WithExpiration(30*time.Minute))
	require.NoError(t, err)

	_, err = Refresh[refreshRotationClaims](context.Background(), token, testSecret, store, WithExpiration(time.Hour))
	require.Error(t, err, "Revoke 出错必须 fail-closed，拒绝刷新")
}
```

- [ ] **Step 3: 更新 `token_test.go` 顶部 import，加入 `context`**

`go-auth/jwt/token_test.go` 的 import 块新增 `"context"`（标准库分组内，按字母序放在 `"crypto/ecdsa"` 之前）：

```go
import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)
```

- [ ] **Step 4: 更新 `token_test.go` 中所有既有 `Refresh` 调用点**

以下 7 个既有测试函数的 `Refresh[...]` 调用都必须补上 `context.Background()` 与一个 `newFakeRevocationStore()` 实例（这些测试用的 Claims 都不带 JTI，store 实际不会被触碰，但签名要求非 nil 传参以体现真实调用契约）：

- `TestRefresh`（`go-auth/jwt/token_test.go:183`）：
  ```go
  newToken, err := Refresh[UserClaims](context.Background(), token, testSecret, newFakeRevocationStore(), WithExpiration(24*time.Hour))
  ```
- `TestRefreshExpiredToken`（`:207`）：
  ```go
  _, err = Refresh[UserClaims](context.Background(), token, testSecret, newFakeRevocationStore(), WithExpiration(time.Hour))
  ```
- `TestRefreshWithIssuer`（`:219`）：
  ```go
  newToken, err := Refresh[UserClaims](context.Background(), token, testSecret, newFakeRevocationStore(),
  	WithExpiration(24*time.Hour),
  	WithIssuer("new-issuer"),
  )
  ```
- `TestRefreshCarriesDefaultExpiration`（`:238`）：
  ```go
  newToken, err := Refresh[UserClaims](context.Background(), token, testSecret, newFakeRevocationStore())
  ```
- `TestRefreshExplicitExpirationOverridesDefault`（`:255`）：
  ```go
  newToken, err := Refresh[UserClaims](context.Background(), token, testSecret, newFakeRevocationStore(), WithExpiration(24*time.Hour))
  ```
- `TestRefreshWithSigningMethod`（`:329`）：
  ```go
  newToken, err := Refresh[UserClaims](context.Background(), token, testSecret, newFakeRevocationStore(),
  	WithExpiration(24*time.Hour),
  	WithSigningMethod(gojwt.SigningMethodHS512),
  )
  ```

（`refreshRotationClaims`/`fakeRevocationStore` 定义在 Step 2 追加的代码块中，位于文件末尾，Go 不要求声明顺序在使用之前，因此这些既有测试函数可以直接引用。）

- [ ] **Step 5: 运行测试确认失败（编译错误，签名不匹配）**

```bash
go test ./go-auth/jwt/... -run TestRefresh -v
```

Expected: 编译失败，`not enough arguments in call to Refresh` 或类似签名不匹配错误（因为 `token.go` 尚未修改）。

- [ ] **Step 6: 修改 `go-auth/jwt/token.go` 的 `Refresh` 实现**

在文件顶部 import 块加入 `"context"` 和 `"github.com/google/uuid"`，以及本地包 `"github.com/byx-darwin/go-tools/go-auth/revocation"`：

```go
import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"errors"
	"fmt"
	"reflect"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/samber/oops"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
	"github.com/byx-darwin/go-tools/go-auth/revocation"
)
```

将 `Refresh` 函数（`token.go:160-177`）替换为：

```go
// Refresh 刷新 JWT（延长过期时间，保留原有 Claims 数据），并对 refresh token 做
// 一次性轮换与复用检测：
//   - 若 Claims 携带 JTI（jti），成功刷新后旧 JTI 会通过 store.Revoke 标记为已
//     撤销，新 Token 携带全新生成的 JTI；同一旧 JTI 被再次用于 Refresh 时视为
//     复用攻击，返回 autherror.ErrTokenRevoked，且不签发新 Token。
//   - 若 Claims 未携带 JTI（ExtractJTI 返回 false），跳过轮换与复用检测，行为
//     与未启用该机制前完全一致（向后兼容）。
//   - store 的 IsRevoked/Revoke 调用失败时按 fail-closed 处理：直接返回错误，
//     不签发新 Token，避免存储故障导致复用检测被绕过。
//   - 检测到复用后是否触发全设备登出由调用方决定（例如收到 ErrTokenRevoked 后
//     调用 device.Store.RemoveAllDevices），本函数不感知 device 包。
//
// secret 的类型要求与 Sign/Verify 一致，由当前签名算法决定。
// 原 Claims 中的 ExpiresAt、Issuer 等会被 opts 中的值覆盖；
// 未显式指定 WithExpiration 时，使用默认 2 小时过期。
func Refresh[T any](ctx context.Context, tokenStr string, secret any, store revocation.Store, opts ...Option) (string, error) {
	// 先验证原 Token，提取 Claims。opts 透传给 Verify 以复用签名算法校验。
	claims, err := Verify[T](tokenStr, secret, opts...)
	if err != nil {
		return "", oops.With("jwt.Refresh").
			Code(autherror.CodeJWTRefreshFailed).
			Wrap(err)
	}

	if jti, ok := ExtractJTI(claims); ok {
		revoked, err := store.IsRevoked(ctx, jti)
		if err != nil {
			return "", oops.With("jwt.Refresh").
				Code(autherror.CodeJWTRefreshFailed).
				Wrap(err)
		}
		if revoked {
			return "", autherror.ErrTokenRevoked.Errorf("jti %s already used for refresh (reuse detected)", jti)
		}

		rc := extractRegisteredClaims(any(claims).(gojwt.Claims))
		if err := store.Revoke(ctx, jti, time.Until(rc.ExpiresAt.Time)); err != nil {
			return "", oops.With("jwt.Refresh").
				Code(autherror.CodeJWTRefreshFailed).
				Wrap(err)
		}

		rc.ID = uuid.NewString()
	}

	// Refresh 语义：新 Token 不复用旧 Token 的剩余有效期，强制刷新 ExpiresAt。
	signOpts := append([]Option{withIgnoreClaimsExpiration()}, opts...)
	return Sign(*claims, secret, signOpts...)
}
```

- [ ] **Step 7: 运行测试确认通过**

```bash
go test ./go-auth/... -count=1 -v -run 'TestRefresh|TestSign|TestVerify|TestExtractJTI'
```

Expected: PASS（包括 Step 2 新增的 5 个测试与 Step 4 更新的既有测试）。

- [ ] **Step 8: 更新 `go-auth/jwt/options.go` 包注释用法示例**

将 `options.go:22` 一行：

```go
//	newToken, err := jwt.Refresh[UserClaims](token, secret, jwt.WithExpiration(24*time.Hour))
```

替换为：

```go
//	newToken, err := jwt.Refresh[UserClaims](ctx, token, secret, revocationStore, jwt.WithExpiration(24*time.Hour))
```

- [ ] **Step 9: 模块内完整验证**

```bash
gofmt -l go-auth/
go vet ./go-auth/...
go test ./go-auth/... -count=1
```

Expected: `gofmt -l` 无输出；`go vet`/`go test` 全部通过。

- [ ] **Step 10: Commit**

```bash
git add go-auth/go.mod go-auth/go.sum go-auth/jwt/token.go go-auth/jwt/options.go go-auth/jwt/token_test.go
git commit -m "feat(go-auth): rotate refresh token JTI and detect reuse in jwt.Refresh"
```

---

## Task 2: go-middleware/auth 新增 `MemoryRevocationStore`

**Files:**
- Create: `go-middleware/auth/revocation_memory.go`
- Create: `go-middleware/auth/revocation_memory_test.go`

**Interfaces:**
- Consumes: `revocation.Store`（`go-auth/revocation/checker.go`，Task 1 未改动此接口）；`hot.NewHotCache[K,V]`（`samber/hot`，已在 `device_memory.go` 中使用，见该文件签名 `hot.NewHotCache[deviceKey, *device.Device](hot.LRU, cfg.cacheSize).Build()`）；`applyDefaults`/`Option`/`WithCacheSize`（`go-middleware/auth/options.go`，已存在，无需新建）。
- Produces:
  ```go
  func NewMemoryRevocationStore(opts ...Option) *MemoryRevocationStore
  func (s *MemoryRevocationStore) Revoke(ctx context.Context, jti string, ttl time.Duration) error
  func (s *MemoryRevocationStore) IsRevoked(ctx context.Context, jti string) (bool, error)
  ```
  供 Task 3（example 接线）使用。

- [ ] **Step 1: 写失败测试**

创建 `go-middleware/auth/revocation_memory_test.go`：

```go
package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/byx-darwin/go-tools/go-auth/revocation"
)

func TestMemoryRevocationStoreInterface(t *testing.T) {
	var _ revocation.Store = NewMemoryRevocationStore()
}

func TestMemoryRevocationStoreRevokeAndCheck(t *testing.T) {
	store := NewMemoryRevocationStore()
	ctx := context.Background()

	revoked, err := store.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	assert.False(t, revoked, "未撤销的 jti 应返回 false")

	require.NoError(t, store.Revoke(ctx, "jti-1", time.Hour))

	revoked, err = store.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	assert.True(t, revoked, "已撤销的 jti 应返回 true")
}

func TestMemoryRevocationStoreNonPositiveTTLNoop(t *testing.T) {
	store := NewMemoryRevocationStore()
	ctx := context.Background()

	require.NoError(t, store.Revoke(ctx, "jti-expired", 0))
	require.NoError(t, store.Revoke(ctx, "jti-negative", -time.Second))

	revoked, err := store.IsRevoked(ctx, "jti-expired")
	require.NoError(t, err)
	assert.False(t, revoked, "ttl<=0 视为无效撤销请求，不应写入")

	revoked, err = store.IsRevoked(ctx, "jti-negative")
	require.NoError(t, err)
	assert.False(t, revoked)
}

func TestMemoryRevocationStoreTTLExpiry(t *testing.T) {
	store := NewMemoryRevocationStore()
	ctx := context.Background()

	require.NoError(t, store.Revoke(ctx, "jti-short", 20*time.Millisecond))

	revoked, err := store.IsRevoked(ctx, "jti-short")
	require.NoError(t, err)
	assert.True(t, revoked)

	time.Sleep(60 * time.Millisecond)

	revoked, err = store.IsRevoked(ctx, "jti-short")
	require.NoError(t, err)
	assert.False(t, revoked, "TTL 过期后应自动清除撤销记录")
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./go-middleware/auth/... -run TestMemoryRevocationStore -v
```

Expected: FAIL（`undefined: NewMemoryRevocationStore`）。

- [ ] **Step 3: 实现 `MemoryRevocationStore`**

创建 `go-middleware/auth/revocation_memory.go`：

```go
package auth

import (
	"context"
	"time"

	"github.com/samber/hot"
	"github.com/samber/oops"

	"github.com/byx-darwin/go-tools/go-auth/revocation"
)

// compile-time interface check.
var _ revocation.Store = (*MemoryRevocationStore)(nil)

// MemoryRevocationStore 基于内存的 Token 撤销存储实现。
//
// 使用 samber/hot 缓存存储已撤销的 JTI，TTL 由调用方在 Revoke 时显式传入
// （应设置为该 token 的剩余有效期），过期后自动从缓存中清除，避免撤销表无限
// 增长。适用于开发和测试环境，不适合生产使用。
type MemoryRevocationStore struct {
	cache *hot.HotCache[string, struct{}]
}

// NewMemoryRevocationStore 创建内存撤销存储。
//
// 默认配置：
//   - cacheSize: 1024
func NewMemoryRevocationStore(opts ...Option) *MemoryRevocationStore {
	cfg := applyDefaults(opts)
	return &MemoryRevocationStore{
		cache: hot.NewHotCache[string, struct{}](hot.LRU, cfg.cacheSize).Build(),
	}
}

// Revoke 撤销指定 jti，ttl 应设置为该 token 的剩余有效期。
// ttl <= 0 视为无效撤销请求（token 已过期，撤销无意义），不写入，返回 nil。
func (s *MemoryRevocationStore) Revoke(_ context.Context, jti string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	s.cache.SetWithTTL(jti, struct{}{}, ttl)
	return nil
}

// IsRevoked 检查指定 jti 是否已被撤销。
func (s *MemoryRevocationStore) IsRevoked(_ context.Context, jti string) (bool, error) {
	_, ok, err := s.cache.Get(jti)
	if err != nil {
		return false, oops.Wrapf(err, "revocation check")
	}
	return ok, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./go-middleware/auth/... -count=1 -run TestMemoryRevocationStore -v
```

Expected: PASS（4 个测试全部通过）。

- [ ] **Step 5: 模块内完整验证**

```bash
gofmt -l go-middleware/
go vet ./go-middleware/...
go test ./go-middleware/... -count=1
```

Expected: `gofmt -l` 无输出；`go vet`/`go test` 全部通过（含既有 `RedisRevocationStore`/`MemoryDeviceStore` 等测试不受影响）。

- [ ] **Step 6: Commit**

```bash
git add go-middleware/auth/revocation_memory.go go-middleware/auth/revocation_memory_test.go
git commit -m "feat(go-middleware): add MemoryRevocationStore implementation"
```

---

## Task 3: example 接线（复用 Task 1 + Task 2 的成果）

**Files:**
- Modify: `example/handler/auth_jwt.go`
- Modify: `example/main.go`

**Interfaces:**
- Consumes: `jwt.Refresh[T any](ctx context.Context, tokenStr string, secret any, store revocation.Store, opts ...Option) (string, error)`（Task 1 产出）；`mwauth.NewMemoryRevocationStore(opts ...Option) *MemoryRevocationStore`（Task 2 产出）；`revocation.Store` 接口类型（`go-auth/revocation`）。
- Produces: `handler.SetRevocationStore(s revocation.Store)`（供 `main.go` 调用，与既有 `SetSessionStore`/`SetDeviceStore` 同模式）。

- [ ] **Step 1: 修改 `example/handler/auth_jwt.go`**

在 import 块新增 `"github.com/byx-darwin/go-tools/go-auth/revocation"`：

```go
import (
	"context"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/byx-darwin/go-tools/go-auth/jwt"
	"github.com/byx-darwin/go-tools/go-auth/revocation"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)
```

在现有 `var ( jwtSecret ... )` 块下方新增包级变量与注入函数（紧跟 `SetJWTConfig` 函数之后）：

```go
// revocationStore Token 撤销存储实例（由 main 通过 SetRevocationStore 注入）。
var revocationStore revocation.Store

// SetRevocationStore 注入撤销存储（在 main 中调用）。
func SetRevocationStore(s revocation.Store) {
	revocationStore = s
}
```

将 `jwtRefreshHandler` 中的调用（原 `auth_jwt.go:134`）：

```go
	// Refresh 内部先验证原 Token，再以新过期时间重新签发。
	newToken, err := jwt.Refresh[AppClaims](req.RefreshToken, jwtSecret,
		jwt.WithIssuer(jwtIssuer),
		jwt.WithExpiration(jwtAccessExpiry),
	)
```

替换为：

```go
	// Refresh 内部先验证原 Token，再以新过期时间重新签发；同时对携带 JTI 的
	// Claims 做一次性轮换与复用检测（见 revocationStore）。
	newToken, err := jwt.Refresh[AppClaims](ctx, req.RefreshToken, jwtSecret, revocationStore,
		jwt.WithIssuer(jwtIssuer),
		jwt.WithExpiration(jwtAccessExpiry),
	)
```

- [ ] **Step 2: 修改 `example/main.go`**

在 import 块新增 `"github.com/byx-darwin/go-tools/go-auth/revocation"`（与 `"github.com/byx-darwin/go-tools/go-auth/device"`、`"github.com/byx-darwin/go-tools/go-auth/session"` 同组，按字母序排列）：

```go
	"github.com/byx-darwin/go-tools/go-auth/device"
	"github.com/byx-darwin/go-tools/go-auth/revocation"
	"github.com/byx-darwin/go-tools/go-auth/session"
```

在 `Deps` 结构体（`main.go:117-132`）中，`DeviceStore device.Store` 字段下方新增：

```go
	// RevocationStore Token 撤销存储（内存或 Redis 实现）。
	RevocationStore revocation.Store
```

在 `initDeps` 函数中，`deps.DeviceStore = mwauth.NewMemoryDeviceStore()`（`main.go:147`）下方新增：

```go
		deps.RevocationStore = mwauth.NewMemoryRevocationStore()
```

在 `handler.SetDeviceStore(deps.DeviceStore)`（`main.go:152`）下方新增：

```go
	handler.SetRevocationStore(deps.RevocationStore)
```

- [ ] **Step 3: 编译验证**

```bash
go build ./example/...
```

Expected: 编译成功，无错误。

- [ ] **Step 4: 模块内完整验证**

```bash
gofmt -l example/
go vet ./example/...
```

Expected: `gofmt -l` 无输出；`go vet` 通过。

- [ ] **Step 5: Commit**

```bash
git add example/handler/auth_jwt.go example/main.go
git commit -m "feat(example): wire MemoryRevocationStore into jwt refresh handler"
```

---

## Task 4: 全仓库最终验证

**Files:** 无新增/修改文件，仅运行验证命令。

**Interfaces:** 无。

- [ ] **Step 1: 全量 build**

```bash
go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... ./example/...
```

Expected: 编译成功。

- [ ] **Step 2: 全量 vet**

```bash
go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... ./example/...
```

Expected: 无输出（通过）。

- [ ] **Step 3: gofmt 检查**

```bash
gofmt -l $(find . -name '*.go' -not -path '*/vendor/*' -not -path './.git/*')
```

Expected: 无输出。

- [ ] **Step 4: 逐 module golangci-lint**

```bash
for m in go-common go-auth go-middleware go-framework; do golangci-lint run --timeout=5m ./$m/...; done
```

Expected: 每个 module 均无 lint 报错。

- [ ] **Step 5: 全量测试**

```bash
go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1
```

Expected: 全部 PASS。

- [ ] **Step 6: Commit（仅在验证过程中发现并修复了问题时才需要）**

```bash
git add -A
git commit -m "chore: fix lint/vet findings from refresh token rotation work"
```

若 Step 1-5 全部一次性通过，无需此提交。

---

## Self-Review Notes（写计划时已核对，无需执行者重复）

- **Spec coverage：** 设计文档中的 API 变更、核心逻辑（无 JTI 跳过 / fail-closed / 新 JTI 生成）、`google/uuid` 依赖、`MemoryRevocationStore`、example 接线、测试、文档 7 个部分均对应 Task 1-3 中的具体 Step；"范围外"两项（`WithReuseHandler`、`Sign`/`Verify` 不加 ctx）未在任何 Task 中出现，符合设计。
- **既有调用点：** 已核实全仓库仅 `example/handler/auth_jwt.go:134` 一处业务调用点（Task 3 已覆盖），`go-auth/jwt/options.go:22` 为文档示例（Task 1 Step 8 已覆盖），`go-auth/jwt/token_test.go` 中 7 处既有测试调用点（Task 1 Step 4 已逐一列出）。
- **类型一致性：** `Refresh` 新签名 `func Refresh[T any](ctx context.Context, tokenStr string, secret any, store revocation.Store, opts ...Option) (string, error)` 在 Task 1 Step 6（实现）与 Task 3 Step 1（调用点）中一致；`MemoryRevocationStore` 的 `Revoke`/`IsRevoked` 方法签名与 `revocation.Store` 接口、`RedisRevocationStore` 保持一致。
