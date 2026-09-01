# go-auth Token 撤销机制 + 错误码传导 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 go-auth 具备独立于 device 模型的 JWT 撤销能力，并让 go-framework 的 JWTAuth/DeviceAuth 中间件把 go-auth 已经精心设计好的错误码（TokenInvalid/TokenExpired/TokenRevoked/DeviceKicked）精确传导到 HTTP 响应，而不是统一压成 401。

**Architecture:** 新增 `go-auth/revocation` 契约包（Checker/Revoker 两个小接口）+ `go-middleware/auth` 的 Redis TTL 实现；`go-auth/jwt` 新增 `ExtractJTI` 反射辅助函数；`go-auth/error` 注册 HTTP 状态码映射；`go-framework/hertz/middleware` 的 JWTAuth/DeviceAuth 改用 Hertz 内建的 `AbortWithError` 记录错误并写出正确状态码（**不依赖 `go-framework/hertz` 包，规避与 `hertz → hertz/middleware` 既有依赖方向的循环 import**）；`go-framework/hertz/response.go` 的 `Responder.Middleware()` 在链路收尾时按需用完整的内容协商（JSON/Protobuf）+ i18n 重写响应体。

**Tech Stack:** Go 1.26, golang-jwt/jwt/v5, samber/oops, go-redis/v9, Hertz, testify, miniredis

**Spec:** `docs/superpowers/specs/2026-09-01-go-auth-token-revocation-design.md`（含循环依赖修正说明，见文档 3.1 节）

## Global Constraints

- go-auth 只能依赖 go-common，禁止反向依赖 go-framework/go-middleware
- go-middleware 可依赖 go-auth + go-common，禁止依赖 go-framework
- go-framework 可依赖 go-auth + go-common，禁止依赖 go-middleware
- **`go-framework/hertz/middleware` 禁止依赖 `go-framework/hertz`**（`hertz` 包已依赖 `hertz/middleware`，反向依赖会造成循环 import，编译失败）
- 3+ 构造参数或 5+ 配置字段的新代码必须用 Functional Options 模式（`.claude/rules/options-pattern.md`）
- 错误码范围：go-auth 40000-40099（本计划新增映射复用现有常量，不新增错误码）
- 所有导出符号必须有 `// Name ...` 格式 godoc 注释（revive 规则）
- 代码必须 gofmt 干净，import 分组：标准库 / 第三方 / 本项目三组（goimports）
- 每个子模块需单独跑 `golangci-lint run ./<module>/...`（golangci-lint v2）
- HTTP 状态码保持 401（Token 相关）/403（DeviceKicked）语义，不采用仓库默认的"业务错误一律 200"约定（本 issue 的既定决策）

---

## File Structure

```
go-auth/revocation/checker.go            (新增) Checker/Revoker 接口
go-auth/jwt/token.go                     (修改) 新增 ExtractJTI 导出函数
go-auth/jwt/token_test.go                (修改) 新增 ExtractJTI 测试
go-auth/error/error.go                   (修改) 新增 init() HTTP 状态码注册
go-auth/error/error_test.go              (修改) 新增 HTTP 状态码映射测试
go-middleware/auth/revocation_redis.go       (新增) RedisRevocationStore
go-middleware/auth/revocation_redis_test.go  (新增) 对应单测
go-framework/hertz/response.go                     (修改) Middleware() 收尾补齐 c.Errors 的内容协商重写
go-framework/hertz/response_integration_test.go    (修改) 新增收尾逻辑的集成测试
go-framework/hertz/middleware/jwt_auth.go       (修改) Option/writeAuthError（AbortWithError）/撤销检查
go-framework/hertz/middleware/jwt_auth_test.go  (修改) 新增撤销/错误码相关测试
go-framework/hertz/middleware/device_auth.go       (修改) writeAuthError 改造
go-framework/hertz/middleware/device_auth_test.go  (修改) 新增内部错误/错误码测试
```

---

### Task 1: `go-auth/revocation` 契约包

**Files:**
- Create: `go-auth/revocation/checker.go`

**Interfaces:**
- Produces: `revocation.Checker` (方法 `IsRevoked(ctx context.Context, jti string) (bool, error)`)，`revocation.Revoker` (方法 `Revoke(ctx context.Context, jti string, ttl time.Duration) error`)，`revocation.Store`（`Checker` + `Revoker`）。供 Task 4（Redis 实现）与 Task 6（JWTAuth 集成）使用。

这是纯接口声明，无行为可测，跳过 TDD 的 RED/GREEN 循环，直接写文件后用编译验证。

- [ ] **Step 1: 编写接口文件**

```go
// Package revocation 定义 JWT 撤销的存储契约。
//
// Checker/Revoker 拆成两个小接口而非一个大接口：中间件只依赖 Checker，
// 未来新增能力（如批量撤销）可以再加独立小接口，不破坏现有实现
// （optional interface pattern）。
package revocation

import (
	"context"
	"time"
)

// Checker 检查 JTI 是否已被撤销。
type Checker interface {
	// IsRevoked 返回 jti 对应的 token 是否已被撤销。
	IsRevoked(ctx context.Context, jti string) (bool, error)
}

// Revoker 撤销指定 JTI。
type Revoker interface {
	// Revoke 撤销 jti，ttl 应设置为该 token 的剩余有效期，
	// 过期后实现方应自动清除撤销记录，避免撤销表无限增长。
	Revoke(ctx context.Context, jti string, ttl time.Duration) error
}

// Store 同时具备撤销与检查能力的完整存储接口。
type Store interface {
	Checker
	Revoker
}
```

- [ ] **Step 2: 验证编译通过**

Run: `cd go-auth && go build ./...`
Expected: 无错误退出

- [ ] **Step 3: gofmt 检查**

Run: `gofmt -l go-auth/revocation/checker.go`
Expected: 无输出（已格式化）

- [ ] **Step 4: Commit**

```bash
git add go-auth/revocation/checker.go
git commit -m "feat(auth): 新增 revocation 撤销契约包"
```

---

### Task 2: `go-auth/jwt` 新增 `ExtractJTI`

**Files:**
- Modify: `go-auth/jwt/token.go`
- Test: `go-auth/jwt/token_test.go`

**Interfaces:**
- Consumes: 内部 `extractRegisteredClaims(claims gojwt.Claims) *gojwt.RegisteredClaims`（已存在于 `token.go`）
- Produces: `ExtractJTI(claims any) (jti string, ok bool)`。供 Task 6（`go-framework/hertz/middleware/jwt_auth.go`）在撤销检查前提取 JTI 使用。

- [ ] **Step 1: 编写失败的测试**

在 `go-auth/jwt/token_test.go` 末尾追加：

```go
type extractJTIClaims struct {
	gojwt.RegisteredClaims
}

func TestExtractJTI(t *testing.T) {
	t.Run("direct embed with ID", func(t *testing.T) {
		claims := &extractJTIClaims{RegisteredClaims: gojwt.RegisteredClaims{ID: "jti-123"}}
		jti, ok := ExtractJTI(claims)
		assert.True(t, ok)
		assert.Equal(t, "jti-123", jti)
	})

	t.Run("no ID set", func(t *testing.T) {
		claims := &extractJTIClaims{}
		jti, ok := ExtractJTI(claims)
		assert.False(t, ok)
		assert.Empty(t, jti)
	})

	t.Run("not a claims type", func(t *testing.T) {
		jti, ok := ExtractJTI("not-claims")
		assert.False(t, ok)
		assert.Empty(t, jti)
	})

	t.Run("nil claims", func(t *testing.T) {
		jti, ok := ExtractJTI(nil)
		assert.False(t, ok)
		assert.Empty(t, jti)
	})

	t.Run("MapClaims not supported", func(t *testing.T) {
		mc := gojwt.MapClaims{"jti": "abc"}
		jti, ok := ExtractJTI(&mc)
		assert.False(t, ok)
		assert.Empty(t, jti)
	})
}
```

确认文件顶部已 import `"github.com/stretchr/testify/assert"`（`token_test.go` 中应已存在，若无则补充）。

- [ ] **Step 2: 运行测试，确认失败**

Run: `cd go-auth && go test ./jwt/... -run TestExtractJTI -v`
Expected: FAIL，`undefined: ExtractJTI`

- [ ] **Step 3: 实现 `ExtractJTI`**

在 `go-auth/jwt/token.go` 的 `extractRegisteredClaims` 函数后追加：

```go
// ExtractJTI 从已验证的 Claims 中提取 JWT ID（jti）。
// claims 通常是 Verify 返回的 *T 指针（或任何实现 jwt.Claims 且嵌入了
// gojwt.RegisteredClaims 的结构体指针）。未找到 RegisteredClaims 或
// ID 为空时返回 ("", false)。
func ExtractJTI(claims any) (string, bool) {
	jwtClaims, ok := claims.(gojwt.Claims)
	if !ok {
		return "", false
	}

	rc := extractRegisteredClaims(jwtClaims)
	if rc == nil || rc.ID == "" {
		return "", false
	}

	return rc.ID, true
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `cd go-auth && go test ./jwt/... -run TestExtractJTI -v`
Expected: PASS（5 个子测试全部通过）

- [ ] **Step 5: 运行整个 jwt 包测试防止回归**

Run: `cd go-auth && go test ./jwt/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add go-auth/jwt/token.go go-auth/jwt/token_test.go
git commit -m "feat(auth): jwt 包新增 ExtractJTI 导出函数"
```

---

### Task 3: `go-auth/error` 注册 HTTP 状态码映射

**Files:**
- Modify: `go-auth/error/error.go`
- Test: `go-auth/error/error_test.go`

**Interfaces:**
- Consumes: `goerror.RegisterHTTPStatuses(map[int]int)`、`goerror.HTTPStatus(err error) int`（已存在于 `go-common/error`）
- Produces: 包 `init()` 副作用——任何 import 了 `go-auth/error` 的代码，`goerror.HTTPStatus()` 对 auth 错误码返回 401/403/500 而非默认的 200/500 兜底。供 Task 5/6/7（`AbortWithError`、`Responder.Middleware()` 收尾逻辑）依赖此映射。

- [ ] **Step 1: 编写失败的测试**

在 `go-auth/error/error_test.go` 末尾追加：

```go
// TestHTTPStatusRegistration 验证 init() 注册的 HTTP 状态码映射。
func TestHTTPStatusRegistration(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"token invalid", ErrTokenInvalid.Wrap(errors.New("x")), 401},
		{"token expired", ErrTokenExpired.Wrap(errors.New("x")), 401},
		{"token revoked", ErrTokenRevoked.Wrap(errors.New("x")), 401},
		{"device kicked", ErrDeviceKicked.Wrap(errors.New("x")), 403},
		{"session invalid", ErrSessionInvalid.Wrap(errors.New("x")), 401},
		{"session expired", ErrSessionExpired.Wrap(errors.New("x")), 401},
		{"jwt sign failed", ErrJWTSignFailed.Wrap(errors.New("x")), 500},
		{"jwt verify failed", ErrJWTVerifyFailed.Wrap(errors.New("x")), 500},
		{"jwt refresh failed", ErrJWTRefreshFailed.Wrap(errors.New("x")), 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, goerror.HTTPStatus(tt.err))
		})
	}
}
```

（`errors`、`goerror`、`assert` 均已在文件顶部 import，无需新增。）

- [ ] **Step 2: 运行测试，确认失败**

Run: `cd go-auth && go test ./error/... -run TestHTTPStatusRegistration -v`
Expected: FAIL（未注册时走范围兜底，`ErrTokenInvalid`(40001) ≥ AuthCodeMin(40000) → 期望值 401 但实际得到 200；`ErrJWTSignFailed`(40007) 走 `code > 0` 分支同样得到 200 而非 500，与期望值不符）

- [ ] **Step 3: 新增 `init()`**

在 `go-auth/error/error.go` 文件末尾追加：

```go
func init() {
	goerror.RegisterHTTPStatuses(map[int]int{
		CodeTokenInvalid:     401,
		CodeTokenExpired:     401,
		CodeTokenRevoked:     401,
		CodeDeviceKicked:     403,
		CodeSessionInvalid:   401,
		CodeSessionExpired:   401,
		CodeJWTSignFailed:    500,
		CodeJWTVerifyFailed:  500,
		CodeJWTRefreshFailed: 500,
	})
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `cd go-auth && go test ./error/... -v`
Expected: PASS（含原有 `TestCodeConstants`/`TestPredefinedErrors` 等）

- [ ] **Step 5: Commit**

```bash
git add go-auth/error/error.go go-auth/error/error_test.go
git commit -m "feat(auth): 注册认证错误码到 HTTP 状态码映射"
```

---

### Task 4: `go-middleware/auth` 新增 `RedisRevocationStore`

**Files:**
- Create: `go-middleware/auth/revocation_redis.go`
- Test: `go-middleware/auth/revocation_redis_test.go`

**Interfaces:**
- Consumes: `revocation.Store`（Task 1）、包内已有的 `Option`/`applyDefaults`/`WithKeyPrefix`（`go-middleware/auth/options.go`，无需修改）
- Produces: `NewRedisRevocationStore(client redis.UniversalClient, opts ...Option) *RedisRevocationStore`，实现 `revocation.Store`。供后续调用方（业务代码）在配置 `middleware.WithRevocationChecker` 时传入。

- [ ] **Step 1: 编写失败的测试**

创建 `go-middleware/auth/revocation_redis_test.go`：

```go
package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/byx-darwin/go-tools/go-auth/revocation"
)

func newTestRevocationRedisClient(t *testing.T) (*miniredis.Miniredis, redis.UniversalClient) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, client
}

func TestRedisRevocationStore_ImplementsInterface(t *testing.T) {
	var _ revocation.Store = (*RedisRevocationStore)(nil)
}

func TestRedisRevocationStore_RevokeAndCheck(t *testing.T) {
	_, client := newTestRevocationRedisClient(t)
	ctx := context.Background()
	s := NewRedisRevocationStore(client)

	revoked, err := s.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	assert.False(t, revoked)

	require.NoError(t, s.Revoke(ctx, "jti-1", time.Hour))

	revoked, err = s.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	assert.True(t, revoked)
}

func TestRedisRevocationStore_TTLExpiry(t *testing.T) {
	mr, client := newTestRevocationRedisClient(t)
	ctx := context.Background()
	s := NewRedisRevocationStore(client)

	require.NoError(t, s.Revoke(ctx, "jti-1", 5*time.Second))

	revoked, err := s.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	assert.True(t, revoked)

	mr.FastForward(6 * time.Second)

	revoked, err = s.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	assert.False(t, revoked, "revocation record should expire after TTL")
}

func TestRedisRevocationStore_ZeroTTLNoop(t *testing.T) {
	_, client := newTestRevocationRedisClient(t)
	ctx := context.Background()
	s := NewRedisRevocationStore(client)

	require.NoError(t, s.Revoke(ctx, "jti-1", 0))

	revoked, err := s.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	assert.False(t, revoked)
}

func TestRedisRevocationStore_WithKeyPrefix(t *testing.T) {
	mr, client := newTestRevocationRedisClient(t)
	ctx := context.Background()
	s := NewRedisRevocationStore(client, WithKeyPrefix("app:"))

	require.NoError(t, s.Revoke(ctx, "jti-1", time.Hour))

	keys := mr.Keys()
	assert.Contains(t, keys, "app:revoked:jti-1")
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `cd go-middleware && go test ./auth/... -run TestRedisRevocationStore -v`
Expected: FAIL，`undefined: RedisRevocationStore` / `undefined: NewRedisRevocationStore`

- [ ] **Step 3: 实现 `RedisRevocationStore`**

创建 `go-middleware/auth/revocation_redis.go`：

```go
package auth

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/samber/oops"

	"github.com/byx-darwin/go-tools/go-auth/revocation"
)

// compile-time interface check.
var _ revocation.Store = (*RedisRevocationStore)(nil)

// RedisRevocationStore 基于 Redis 的 Token 撤销存储实现。
//
// 使用 Redis String 存储撤销标记：
//   - Key: {prefix}revoked:{jti}
//   - Value: ""（存在即表示已撤销）
//
// 撤销记录的 TTL 由调用方在 Revoke 时显式传入（应设置为该 token 的
// 剩余有效期），过期后 Redis 自动清除，避免撤销表无限增长。
type RedisRevocationStore struct {
	client redis.UniversalClient
	prefix string
}

// NewRedisRevocationStore 创建 Redis 撤销存储。
//
// 默认配置：
//   - keyPrefix: ""
func NewRedisRevocationStore(client redis.UniversalClient, opts ...Option) *RedisRevocationStore {
	cfg := applyDefaults(opts)
	return &RedisRevocationStore{
		client: client,
		prefix: cfg.keyPrefix,
	}
}

// revokedKey 构建 Redis key。
func (s *RedisRevocationStore) revokedKey(jti string) string {
	return s.prefix + "revoked:" + jti
}

// Revoke 撤销指定 jti，ttl 应设置为该 token 的剩余有效期。
// ttl <= 0 视为无效撤销请求（token 已过期，撤销无意义），不写入，返回 nil。
func (s *RedisRevocationStore) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}

	if err := s.client.Set(ctx, s.revokedKey(jti), "", ttl).Err(); err != nil {
		return oops.Wrapf(err, "revocation revoke")
	}

	return nil
}

// IsRevoked 检查指定 jti 是否已被撤销。
func (s *RedisRevocationStore) IsRevoked(ctx context.Context, jti string) (bool, error) {
	_, err := s.client.Get(ctx, s.revokedKey(jti)).Result()
	if err != nil {
		if err == redis.Nil { //nolint:errorlint // go-redis 约定 redis.Nil 为哨兵值，用 == 比较
			return false, nil
		}
		return false, oops.Wrapf(err, "revocation check")
	}

	return true, nil
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `cd go-middleware && go test ./auth/... -run TestRedisRevocationStore -v`
Expected: PASS（5 个测试全部通过）

- [ ] **Step 5: 运行整个 auth 包测试防止回归**

Run: `cd go-middleware && go test ./auth/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add go-middleware/auth/revocation_redis.go go-middleware/auth/revocation_redis_test.go
git commit -m "feat(middleware): 新增 RedisRevocationStore 撤销存储实现"
```

---

### Task 5: `go-framework/hertz/response.go` — `Responder.Middleware()` 收尾补齐内容协商

**Files:**
- Modify: `go-framework/hertz/response.go`
- Test: `go-framework/hertz/response_integration_test.go`

**Interfaces:**
- Consumes: `goerror.Extract`/`goerror.HTTPStatus`（已存在，`response.go` 已 import `goerror`）、Hertz `app.RequestContext.Errors`（框架自带字段，`[]*errors.Error`，`Errors.Last() *errors.Error`）
- Produces: `Responder.Middleware()` 行为变更（签名不变）——`c.Next(ctx)` 返回后，若 `c.Errors` 非空，用 `ErrorWithCode` 重写响应体。供 Task 6/7（`middleware.JWTAuth`/`DeviceAuth` 通过 `c.AbortWithError` 记录的错误）在配置了 `Responder.Middleware()` 时获得完整内容协商。

此任务与 Task 6/7 顺序无关（`response.go` 的改动不依赖 `middleware` 包的任何符号），可先做，为后续任务的集成测试提供正确的收尾行为。

- [ ] **Step 1: 编写失败的测试**

在 `go-framework/hertz/response_integration_test.go` 的 `setupHertzEngine` 函数中新增一个路由（在 `error-with-code` 路由之后插入）：

```go
	engine.GET("/error-via-context", func(ctx context.Context, c *app.RequestContext) {
		err := frameworkerror.ErrParamInvalid.Wrap(errors.New("via context"))
		c.AbortWithError(http.StatusBadRequest, err)
	})
```

并在文件末尾追加测试：

```go
func TestResponder_Middleware_RewritesContextError(t *testing.T) {
	r := NewResponder()
	engine := setupHertzEngine(t, r)

	w := ut.PerformRequest(engine, http.MethodGet, "/error-via-context", nil)

	// AbortWithError 已经写出正确状态码；Middleware() 收尾后 body 应变成
	// 完整协商后的 JSON（而不是空 body）。
	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(frameworkerror.CodeParamInvalid), resp["code"])
	assert.Equal(t, "param_invalid", resp["msg"])
}

func TestResponder_Middleware_NoErrorsNoOverwrite(t *testing.T) {
	r := NewResponder()
	engine := setupHertzEngine(t, r)

	w := ut.PerformRequest(engine, http.MethodGet, "/success", nil)

	// 成功路径 c.Errors 为空，收尾逻辑不应有任何影响。
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "ok", resp["msg"])
}
```

`frameworkerror.ErrParamInvalid` 的 `Public()` 固定串是 `"param_invalid"`（见 `go-framework/error/error.go:77`），与 `Wrap` 时传入的内部 `err.Error()`（"via context"）不同——`goerror.Extract` 返回的 `msg` 取自 `Public()`，不是 `err.Error()`。

- [ ] **Step 2: 运行测试，确认失败**

Run: `cd go-framework && go test ./hertz/... -run TestResponder_Middleware_RewritesContextError -v`
Expected: FAIL，响应体为空（`json.Unmarshal` 对空 body 报错，或 `resp["code"]` 与期望不符），因为 `Middleware()` 尚未读取 `c.Errors`

- [ ] **Step 3: 实现收尾逻辑**

修改 `go-framework/hertz/response.go` 的 `Middleware()` 方法（`// ── Middleware ──` 一节），在 `c.Next(ctx)` 之后追加：

```go
// Middleware 返回 Hertz 中间件处理函数。
// 中间件职责：
//  1. 提取/生成 Request ID → 设置响应头 + 注入 ctx
//  2. 提取语言偏好 → 注入 ctx
//  3. 注入 Responder 实例 → 注入 ctx
//  4. 链路收尾：若下游通过 c.Error/c.AbortWithError 记录了错误但只写了
//     空 body（如认证中间件），用完整的内容协商（JSON/Protobuf）+ i18n
//     重写响应体
func (r *Responder) Middleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// 1. 提取 Request ID
		reqID := r.extractRequestID(ctx, c)
		if reqID != "" && r.reqIDHeader != "" {
			c.Response.Header.Set(r.reqIDHeader, reqID)
		}
		c.Set(string(ctxKeyRequestID), reqID)

		// 2. 提取语言偏好
		lang := r.extractLang(c)
		c.Set(string(ctxKeyLang), lang)

		// 3. 注入 Responder
		c.Set(string(ctxKeyResponder), r)

		// 4. recover 保护：handler panic 时返回结构化错误响应
		defer func() {
			if rec := recover(); rec != nil {
				err := fmt.Errorf("panic: %v", rec)
				r.Error(ctx, c, err, "internal server error")
			}
		}()

		c.Next(ctx)

		// 下游中间件（如认证中间件）可能只调用了 AbortWithError 记录错误、
		// 写了空 body。这里统一补齐：状态码计算方式与下游 baseline 一致
		// （都是 goerror.HTTPStatus），不依赖 ErrorRouter 是否配置。
		if len(c.Errors) > 0 {
			last := c.Errors.Last()
			if last != nil {
				code, msg := goerror.Extract(last.Err)
				r.ErrorWithCode(ctx, c, goerror.HTTPStatus(last.Err), code, msg)
			}
		}
	}
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `cd go-framework && go test ./hertz/... -run TestResponder_Middleware -v`
Expected: PASS

- [ ] **Step 5: 运行整个 hertz 包测试防止回归**

Run: `cd go-framework && go test ./hertz/... -v`
Expected: PASS（含既有的全部 `TestResponder_*`/`TestRespondFrom_*` 用例）

- [ ] **Step 6: Commit**

```bash
git add go-framework/hertz/response.go go-framework/hertz/response_integration_test.go
git commit -m "feat(framework): Responder.Middleware() 收尾补齐 c.Errors 的内容协商"
```

---

### Task 6: `go-framework/hertz/middleware/jwt_auth.go` 改造

**Files:**
- Modify: `go-framework/hertz/middleware/jwt_auth.go`
- Test: `go-framework/hertz/middleware/jwt_auth_test.go`

**Interfaces:**
- Consumes: `gojwt.ExtractJTI`（Task 2）、`goerror.HTTPStatus`（已存在，来自 `go-common/error`，**不是** `go-framework/hertz`）、Hertz `app.RequestContext.AbortWithError`（框架自带）、`revocation.Checker`（Task 1）
- Produces: `JWTAuth[T any](secret []byte, opts ...Option) app.HandlerFunc`（新增变参 `opts`，向后兼容）、`WithRevocationChecker(checker revocation.Checker) Option`、包内可见的 `writeAuthError(c *app.RequestContext, err error)`（Task 7 的 `device_auth.go` 会复用）

- [ ] **Step 1: 编写失败的测试**

在 `go-framework/hertz/middleware/jwt_auth_test.go` 顶部 import 块中新增 `"errors"`，并在文件末尾追加：

```go
type revocationTestClaims struct {
	gojwt.RegisteredClaims
}

func issueTestTokenWithJTI(t *testing.T, secret []byte, jti string) string {
	t.Helper()
	claims := revocationTestClaims{
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   "user-1",
			ID:        jti,
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token, err := authjwt.Sign(claims, secret)
	require.NoError(t, err)
	return token
}

type mockRevocationChecker struct {
	revoked map[string]bool
	err     error
}

func (m *mockRevocationChecker) IsRevoked(_ context.Context, jti string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.revoked[jti], nil
}

func TestJWTAuth_RevocationChecker_Revoked(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	token := issueTestTokenWithJTI(t, secret, "jti-revoked")
	checker := &mockRevocationChecker{revoked: map[string]bool{"jti-revoked": true}}

	engine := newTestEngine()
	engine.Use(JWTAuth[revocationTestClaims](secret, WithRevocationChecker(checker)))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode())
}

func TestJWTAuth_RevocationChecker_NotRevoked(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	token := issueTestTokenWithJTI(t, secret, "jti-active")
	checker := &mockRevocationChecker{revoked: map[string]bool{}}

	engine := newTestEngine()
	engine.Use(JWTAuth[revocationTestClaims](secret, WithRevocationChecker(checker)))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
}

func TestJWTAuth_RevocationChecker_QueryError(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	token := issueTestTokenWithJTI(t, secret, "jti-x")
	checker := &mockRevocationChecker{err: errors.New("redis down")}

	engine := newTestEngine()
	engine.Use(JWTAuth[revocationTestClaims](secret, WithRevocationChecker(checker)))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	res := w.Result()
	assert.Equal(t, http.StatusInternalServerError, res.StatusCode())
}

func TestJWTAuth_NoRevocationChecker_BehaviorUnchanged(t *testing.T) {
	// 未配置 WithRevocationChecker 时，行为应与旧版完全一致（回归测试）。
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	token := issueTestToken(t, secret, "user-123", time.Now().Add(time.Hour))

	engine := newTestEngine()
	engine.Use(JWTAuth[testClaims](secret))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		claims, ok := GetClaims[testClaims](c)
		assert.True(t, ok)
		c.JSON(http.StatusOK, map[string]string{"user": claims.UserUUID})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
	assert.Contains(t, string(res.Body()), "user-123")
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `cd go-framework && go test ./hertz/middleware/... -run TestJWTAuth -v`
Expected: FAIL，`undefined: WithRevocationChecker`（包编译失败，新符号缺失）

- [ ] **Step 3: 实现改造**

用以下内容替换 `go-framework/hertz/middleware/jwt_auth.go` 全文：

```go
package middleware

import (
	"context"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/samber/oops"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
	gojwt "github.com/byx-darwin/go-tools/go-auth/jwt"
	"github.com/byx-darwin/go-tools/go-auth/revocation"
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// config 存储 JWTAuth 配置选项。
type config struct {
	revocationChecker revocation.Checker
}

// Option 定义 JWTAuth 配置选项函数。
type Option func(*config)

// WithRevocationChecker 设置撤销检查器。
// 验证签名成功后，若配置了 checker，会额外查询撤销表；命中则返回 ErrTokenRevoked。
// 未设置时行为与不启用撤销检查完全一致。
func WithRevocationChecker(checker revocation.Checker) Option {
	return func(c *config) { c.revocationChecker = checker }
}

// applyOptions 应用选项并返回配置快照。
func applyOptions(opts []Option) config {
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// JWTAuth 返回 JWT 认证中间件。
// 从 Authorization Bearer 头解析 token，验证签名，将 claims 注入 RequestContext。
// T 必须嵌入 jwt.RegisteredClaims。可通过 WithRevocationChecker 启用撤销检查。
//
// 使用方式：
//
//	engine.Use(middleware.JWTAuth[UserClaims](secret))
//	claims, ok := middleware.GetClaims[UserClaims](c)
func JWTAuth[T any](secret []byte, opts ...Option) app.HandlerFunc {
	cfg := applyOptions(opts)

	return func(ctx context.Context, c *app.RequestContext) {
		token := extractBearerToken(c)
		if token == "" {
			writeAuthError(c, autherror.ErrTokenInvalid.Errorf("missing bearer token"))
			return
		}

		claims, err := gojwt.Verify[T](token, secret)
		if err != nil {
			writeAuthError(c, err)
			return
		}

		if cfg.revocationChecker != nil {
			if jti, ok := gojwt.ExtractJTI(claims); ok {
				revoked, rerr := cfg.revocationChecker.IsRevoked(ctx, jti)
				if rerr != nil {
					writeAuthError(c, oops.Code(autherror.CodeJWTVerifyFailed).Wrap(rerr))
					return
				}
				if revoked {
					writeAuthError(c, autherror.ErrTokenRevoked.Errorf("token revoked"))
					return
				}
			}
		}

		SetClaims(c, claims)
		c.Next(ctx)
	}
}

// writeAuthError 中断请求并记录错误。不依赖 go-framework/hertz 包——
// hertz 包（server.go）已经 import 了 hertz/middleware（挂载 middleware.Cors()），
// 若 middleware 反过来 import hertz 会形成循环 import，编译失败。
// AbortWithError 是 Hertz 框架自带方法，立即写出正确的 HTTP 状态码
// （goerror.HTTPStatus(err)）并把 err 压入 c.Errors；未配置
// hertz.Responder.Middleware() 时 body 为空，状态码已经正确（与当前
// AbortWithStatus 行为等价，无回归）；配置了的话，Responder.Middleware()
// 会在链路收尾时用完整的内容协商重写响应体。
func writeAuthError(c *app.RequestContext, err error) {
	c.AbortWithError(goerror.HTTPStatus(err), err)
}

// extractBearerToken 从 Authorization 头提取 Bearer token。
func extractBearerToken(c *app.RequestContext) string {
	auth := string(c.Request.Header.Peek("Authorization"))
	if auth == "" {
		return ""
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}

	token := strings.TrimSpace(auth[len(prefix):])
	if token == "" {
		return ""
	}

	return token
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `cd go-framework && go test ./hertz/middleware/... -run TestJWTAuth -v`
Expected: PASS（含既有的 `TestJWTAuth_Success`/`MissingHeader`/`InvalidToken`/`WrongSecret`/`ExpiredToken`/`NonBearerPrefix` 与新增用例）

- [ ] **Step 5: 运行整个 middleware 包测试防止回归**

Run: `cd go-framework && go test ./hertz/middleware/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add go-framework/hertz/middleware/jwt_auth.go go-framework/hertz/middleware/jwt_auth_test.go
git commit -m "feat(framework): JWTAuth 支持撤销检查，错误响应传导精确状态码"
```

---

### Task 7: `go-framework/hertz/middleware/device_auth.go` 改造

**Files:**
- Modify: `go-framework/hertz/middleware/device_auth.go`
- Test: `go-framework/hertz/middleware/device_auth_test.go`

**Interfaces:**
- Consumes: `writeAuthError`（Task 6，同包内）、`autherror.ErrDeviceKicked`/`autherror.ErrTokenInvalid`（已存在）
- Produces: `DeviceAuth` 行为变更（签名不变），失败时状态码精确区分踢出（403）与内部错误（500）

- [ ] **Step 1: 编写失败的测试**

`DeviceAuth` 改造后，"设备被踢出/JTI 不匹配"的状态码从 401 变为 403（`ErrDeviceKicked` 映射）。先修改两个既有测试的断言，把 `assert.Equal(t, http.StatusUnauthorized, res.StatusCode())` 改成 `assert.Equal(t, http.StatusForbidden, res.StatusCode())`：

- `TestDeviceAuth_DeviceNotFound`
- `TestDeviceAuth_JTIMismatch`

（`TestDeviceAuth_NoClaimsInContext`、`TestDeviceAuth_ExtractReturnsEmptyFields` 对应 `ErrTokenInvalid`，仍是 401，不需要改。）

在 `go-framework/hertz/middleware/device_auth_test.go` 顶部 import 块新增 `"errors"`，并在文件末尾追加：

```go
type errorDeviceStore struct{}

func (errorDeviceStore) AddDevice(context.Context, string, string, string, int) ([]device.Device, error) {
	return nil, nil
}

func (errorDeviceStore) CheckDevice(context.Context, string, string, string) (bool, error) {
	return false, errors.New("redis down")
}

func (errorDeviceStore) RemoveDevice(context.Context, string, string) error { return nil }

func (errorDeviceStore) RemoveAllDevices(context.Context, string) error { return nil }

func (errorDeviceStore) ListDevices(context.Context, string) ([]device.Device, error) {
	return nil, nil
}

func TestDeviceAuth_StoreError(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	store := errorDeviceStore{}
	token := issueDeviceToken(t, secret, "user-1", "dev-1", "jti-1")

	extract := func(claims any) (string, string, string) {
		c := claims.(*deviceTestClaims)
		return c.Subject, c.DeviceID, c.JTI
	}

	engine := newDeviceTestEngine()
	engine.Use(JWTAuth[deviceTestClaims](secret))
	engine.Use(DeviceAuth(store, extract))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	res := w.Result()
	assert.Equal(t, http.StatusInternalServerError, res.StatusCode())
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `cd go-framework && go test ./hertz/middleware/... -run TestDeviceAuth -v`
Expected: FAIL，`TestDeviceAuth_StoreError`、`TestDeviceAuth_DeviceNotFound`、`TestDeviceAuth_JTIMismatch` 均 FAIL（当前实现统一返回 401，不区分内部错误 500 与踢出 403）

- [ ] **Step 3: 实现改造**

用以下内容替换 `go-framework/hertz/middleware/device_auth.go` 全文：

```go
package middleware

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/samber/oops"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
	"github.com/byx-darwin/go-tools/go-auth/device"
)

// DeviceClaims 提取函数类型。
// 从用户自定义的 Claims 中提取 DeviceAuth 所需的字段。
// userUUID 通常对应 JWT RegisteredClaims.Subject。
type DeviceClaims func(claims any) (userUUID, deviceID, jti string)

// DeviceAuth 返回设备会话检查中间件。
// 需配合 JWTAuth 使用。通过 extract 函数从用户 Claims 中提取 user_uuid、device_id、jti，
// 然后调用 device.Store.CheckDevice 验证设备会话是否有效。
//
// extract 函数的 claims 参数是 JWTAuth 注入的 *T 指针。
// 用户需提供提取逻辑，例如：
//
//	extract := func(claims any) (string, string, string) {
//	    c := claims.(*UserClaims)
//	    return c.Subject, c.DeviceID, c.JTI
//	}
//	engine.Use(middleware.DeviceAuth(deviceStore, extract))
func DeviceAuth(store device.Store, extract DeviceClaims) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		claims, ok := c.Get(string(ctxKeyClaims))
		if !ok || extract == nil {
			writeAuthError(c, autherror.ErrTokenInvalid.Errorf("missing claims in context"))
			return
		}

		userUUID, deviceID, jti := extract(claims)
		if userUUID == "" || deviceID == "" || jti == "" {
			writeAuthError(c, autherror.ErrTokenInvalid.Errorf("incomplete device claims"))
			return
		}

		valid, err := store.CheckDevice(ctx, userUUID, deviceID, jti)
		if err != nil {
			writeAuthError(c, oops.Code(autherror.CodeJWTVerifyFailed).Wrap(err))
			return
		}
		if !valid {
			writeAuthError(c, autherror.ErrDeviceKicked.Errorf("device kicked"))
			return
		}

		c.Next(ctx)
	}
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `cd go-framework && go test ./hertz/middleware/... -run TestDeviceAuth -v`
Expected: PASS（含既有的 `TestDeviceAuth_Success`/`DeviceNotFound`（已改为 403 断言）/`JTIMismatch`（已改为 403 断言）/`NoClaimsInContext`/`ExtractReturnsEmptyFields` 与新增的 `StoreError`）

- [ ] **Step 5: Commit**

```bash
git add go-framework/hertz/middleware/device_auth.go go-framework/hertz/middleware/device_auth_test.go
git commit -m "feat(framework): DeviceAuth 错误响应传导精确状态码，区分内部错误与踢出"
```

---

### Task 8: 全量验证

**Files:** 无新增/修改，仅验证

- [ ] **Step 1: gofmt 全仓库检查**

Run: `gofmt -l $(find go-auth go-middleware go-framework -name '*.go' -not -path '*/vendor/*')`
Expected: 无输出

- [ ] **Step 2: 构建全部相关模块**

Run: `go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`
Expected: 无错误退出（重点验证 `go-framework/hertz` 与 `go-framework/hertz/middleware` 之间没有循环 import）

- [ ] **Step 3: go vet**

Run: `go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`
Expected: 无错误退出

- [ ] **Step 4: golangci-lint（逐 module）**

Run:
```bash
for m in go-common go-auth go-middleware go-framework; do
  golangci-lint run --timeout=5m ./$m/...
done
```
Expected: 每个 module 均无 lint 报错

- [ ] **Step 5: 全量测试**

Run: `go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1`
Expected: PASS，无失败用例

- [ ] **Step 6: Commit（如验证步骤本身无代码变更，此步跳过；若 lint 修复产生变更，按修复内容提交）**

```bash
git status --porcelain
# 若为空则无需提交；若有 lint 自动修复的变更，按内容单独 commit
```

---

## Acceptance Criteria 对照

| Issue #72 验收标准 | 对应 Task |
|---|---|
| 新增 revocation 接口契约（go-auth）+ Redis 实现（go-middleware） | Task 1, Task 4 |
| JWTAuth 验证签名后追加撤销表查询，命中则返回 ErrTokenRevoked | Task 6 |
| jwt_auth.go / device_auth.go 中间件按错误 code 区分响应，不再统一压成 401 | Task 6, Task 7 |
| 补充撤销场景的单元测试与集成测试 | Task 4, Task 5, Task 6, Task 7 |
| 未配置 WithRevocationChecker 时，JWTAuth 行为与改造前完全一致（回归测试） | Task 6（`TestJWTAuth_NoRevocationChecker_BehaviorUnchanged`） |
| 撤销表/设备存储查询失败时返回 500，不误判为撤销/踢出 | Task 6（`TestJWTAuth_RevocationChecker_QueryError`）、Task 7（`TestDeviceAuth_StoreError`） |
| HTTP 状态码符合设计文档映射 | Task 3, Task 5, Task 6, Task 7, Task 8 |
