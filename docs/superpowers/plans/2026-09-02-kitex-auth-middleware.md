# kitex 侧鉴权中间件 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `go-framework/kitex/middleware/auth/` 新增 JWT / Session / Device 三种鉴权中间件（各含 Server 校验 + Client 注入身份，对称实现）以及独立的 `Recovery()` panic 恢复中间件，补齐 kitex 侧与 hertz 侧的鉴权能力对称性（Issue #94）。

**Architecture:** 每种鉴权中间件直接依赖 `github.com/cloudwego/kitex/pkg/endpoint`（跟随 `observability/suite.go` 模式，不走 `accesslog.go` 的 compat 间接层）。身份信息通过 `github.com/bytedance/gopkg/cloud/metainfo` 的 `WithPersistentValue`/`GetPersistentValue` 在 Client→Server 之间传递，`WithPersistentValue` 语义天然支持多跳自动透传（`metainfo.node.transferForward()` 保留 `persistent` 字段不变），无需业务代码手动转发。鉴权失败统一通过 `go-framework/kitex/rpcerror.OopsStatusAdapter` 包装为 Kitex `BizStatusErrorIface` 返回。

**Tech Stack:** Go 1.26（workspace `go-framework` 模块，已有依赖：`github.com/cloudwego/kitex v0.16.2`、`github.com/bytedance/gopkg v0.1.4`、`github.com/byx-darwin/go-tools/go-auth v0.2.0`、`github.com/samber/oops v1.22.0`；均已在 `go-framework/go.mod` 中声明，本计划不需要新增依赖）。

**Spec:** `docs/superpowers/specs/2026-09-02-kitex-auth-middleware-design.md`

## Global Constraints

- 新包路径固定为 `go-framework/kitex/middleware/auth/`，包名 `auth`
- 中间件必须返回真正的 `endpoint.Middleware`（`github.com/cloudwego/kitex/pkg/endpoint`），不引入自定义 `Endpoint`/`Middleware` 类型
- 身份透传统一使用 `metainfo.WithPersistentValue` / `metainfo.GetPersistentValue`（不使用 `WithValue`/`GetValue`，避免仅单跳透传）
- 鉴权/panic 失败必须通过 `rpcerror.OopsStatusAdapter` 包装后返回，禁止裸 `return err`
- 复用 `go-auth/error`（`autherror.Err*`）与 `go-framework/error`（`frameworkerror.Err*`）已有错误码，不新增错误码
- 不修改 `go-framework/hertz/middleware/` 任何文件（含已知的 Session 中间件错误路由不一致问题）
- 不新增 `WithPanicHandler` 之类的自定义回调钩子
- 所有导出符号必须有 `// Name ...` 格式 godoc 注释（golangci-lint `revive` 规则）
- `gofmt` / `goimports`（标准库/第三方/本项目三组）/ `go vet` 必须通过

---

### Task 1: Context 辅助函数与 metainfo key 常量

**Files:**
- Create: `go-framework/kitex/middleware/auth/context.go`
- Test: `go-framework/kitex/middleware/auth/context_test.go`

**Interfaces:**
- Produces（后续 Task 2-4 依赖）:
  - `func SetClaims[T any](ctx context.Context, claims *T) context.Context`
  - `func GetClaims[T any](ctx context.Context) (*T, bool)`
  - `func SetJWTToken(ctx context.Context, token string) context.Context`
  - `func GetJWTToken(ctx context.Context) (string, bool)`
  - `func SetSession(ctx context.Context, s *session.Session) context.Context`
  - `func GetSession(ctx context.Context) (*session.Session, bool)`
  - `func SetSessionID(ctx context.Context, id string) context.Context`
  - `func GetSessionID(ctx context.Context) (string, bool)`
  - 未导出 metainfo key 常量：`metaKeyJWTToken`、`metaKeySessionID`、`metaKeyDeviceUserUUID`、`metaKeyDeviceID`、`metaKeyDeviceJTI`
  - 未导出错误包装辅助：`func bizError(err error) error`（包装为 `*rpcerror.OopsStatusAdapter`）

- [ ] **Step 1: Write the failing test**

```go
// go-framework/kitex/middleware/auth/context_test.go
package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/byx-darwin/go-tools/go-auth/session"
	"github.com/byx-darwin/go-tools/go-framework/kitex/rpcerror"
)

type testClaims struct {
	Subject string
}

func TestSetGetClaims(t *testing.T) {
	ctx := context.Background()

	got, ok := GetClaims[testClaims](ctx)
	assert.False(t, ok)
	assert.Nil(t, got)

	claims := &testClaims{Subject: "user-1"}
	ctx = SetClaims(ctx, claims)

	got, ok = GetClaims[testClaims](ctx)
	assert.True(t, ok)
	assert.Equal(t, claims, got)
}

func TestSetGetJWTToken(t *testing.T) {
	ctx := context.Background()

	got, ok := GetJWTToken(ctx)
	assert.False(t, ok)
	assert.Empty(t, got)

	ctx = SetJWTToken(ctx, "token-abc")

	got, ok = GetJWTToken(ctx)
	assert.True(t, ok)
	assert.Equal(t, "token-abc", got)
}

func TestSetGetSession(t *testing.T) {
	ctx := context.Background()

	got, ok := GetSession(ctx)
	assert.False(t, ok)
	assert.Nil(t, got)

	s := &session.Session{ID: "sess-1", UserUUID: "user-1", ExpiresAt: time.Now()}
	ctx = SetSession(ctx, s)

	got, ok = GetSession(ctx)
	assert.True(t, ok)
	assert.Equal(t, s, got)
}

func TestSetGetSessionID(t *testing.T) {
	ctx := context.Background()

	got, ok := GetSessionID(ctx)
	assert.False(t, ok)
	assert.Empty(t, got)

	ctx = SetSessionID(ctx, "sess-1")

	got, ok = GetSessionID(ctx)
	assert.True(t, ok)
	assert.Equal(t, "sess-1", got)
}

func TestBizError_WrapsAsOopsStatusAdapter(t *testing.T) {
	inner := errors.New("inner error")
	err := bizError(inner)

	adapter, ok := err.(*rpcerror.OopsStatusAdapter)
	assert.True(t, ok)
	assert.Equal(t, inner, adapter.Err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go-framework && go test ./kitex/middleware/auth/... -run TestSetGetClaims -v`
Expected: FAIL — `package auth: no Go files` / undefined `SetClaims` 等（包尚未创建）

- [ ] **Step 3: Write minimal implementation**

```go
// go-framework/kitex/middleware/auth/context.go

// Package auth 提供 Kitex RPC 鉴权中间件：JWT / Session / Device 三种鉴权
// （各含 Server 端校验 + Client 端注入身份，对称实现）以及独立的 panic
// recovery 中间件。
//
// 身份信息通过 github.com/bytedance/gopkg/cloud/metainfo 的
// WithPersistentValue/GetPersistentValue 在 RPC 调用链路中传递，
// 天然支持多跳自动持久透传（无需业务代码在中间服务手动转发）。
//
// 鉴权失败统一通过 rpcerror.OopsStatusAdapter 包装为 Kitex
// BizStatusErrorIface 返回，错误码复用 go-auth/error 与 go-framework/error
// 已有的鉴权错误码。
package auth

import (
	"context"

	"github.com/byx-darwin/go-tools/go-auth/session"
	"github.com/byx-darwin/go-tools/go-framework/kitex/rpcerror"
)

// ctxKey 是本包 context value 的私有 key 类型，避免与其他包的 key 冲突。
type ctxKey string

const (
	ctxKeyJWTClaims ctxKey = "auth:jwt:claims"
	ctxKeyJWTToken  ctxKey = "auth:jwt:token"
	ctxKeySession   ctxKey = "auth:session"
	ctxKeySessionID ctxKey = "auth:session:id"
)

// metainfo persistent key 常量：用于在 RPC 调用链路中透传身份信息。
const (
	metaKeyJWTToken       = "auth-jwt-token"
	metaKeySessionID      = "auth-session-id"
	metaKeyDeviceUserUUID = "auth-device-user-uuid"
	metaKeyDeviceID       = "auth-device-id"
	metaKeyDeviceJTI      = "auth-device-jti"
)

// SetClaims 将 JWT claims 注入 ctx，返回携带该值的新 ctx。
func SetClaims[T any](ctx context.Context, claims *T) context.Context {
	return context.WithValue(ctx, ctxKeyJWTClaims, claims)
}

// GetClaims 从 ctx 提取 JWT claims。
func GetClaims[T any](ctx context.Context) (*T, bool) {
	v := ctx.Value(ctxKeyJWTClaims)
	if v == nil {
		return nil, false
	}
	claims, ok := v.(*T)
	return claims, ok
}

// SetJWTToken 将已校验通过的原始 JWT token 字符串注入 ctx，供
// JWTAuthClient 在同一调用链继续向下游透传时复用（避免重新签发）。
func SetJWTToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, ctxKeyJWTToken, token)
}

// GetJWTToken 从 ctx 提取已校验通过的原始 JWT token 字符串。
func GetJWTToken(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKeyJWTToken).(string)
	return v, ok
}

// SetSession 将 Session 注入 ctx，返回携带该值的新 ctx。
func SetSession(ctx context.Context, s *session.Session) context.Context {
	return context.WithValue(ctx, ctxKeySession, s)
}

// GetSession 从 ctx 提取 Session。
func GetSession(ctx context.Context) (*session.Session, bool) {
	v, ok := ctx.Value(ctxKeySession).(*session.Session)
	return v, ok
}

// SetSessionID 将已校验通过的 Session ID 注入 ctx，供 SessionAuthClient
// 在同一调用链继续向下游透传时复用。
func SetSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeySessionID, id)
}

// GetSessionID 从 ctx 提取已校验通过的 Session ID。
func GetSessionID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKeySessionID).(string)
	return v, ok
}

// bizError 将错误包装为 Kitex BizStatusErrorIface，使 oops 错误码/公开消息
// 能通过 Kitex BizStatus 机制正确返回给调用方。
func bizError(err error) error {
	return &rpcerror.OopsStatusAdapter{Err: err}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go-framework && go test ./kitex/middleware/auth/... -v`
Expected: PASS（全部 5 个测试）

- [ ] **Step 5: Commit**

```bash
git add go-framework/kitex/middleware/auth/context.go go-framework/kitex/middleware/auth/context_test.go
git commit -m "feat(kitex): add auth package context helpers and metainfo keys"
```

---

### Task 2: JWTAuthServer / JWTAuthClient

**Files:**
- Create: `go-framework/kitex/middleware/auth/jwt.go`
- Test: `go-framework/kitex/middleware/auth/jwt_test.go`

**Interfaces:**
- Consumes: Task 1 的 `SetClaims`/`GetClaims`/`SetJWTToken`/`GetJWTToken`/`bizError`/`metaKeyJWTToken`
- Produces:
  - `type JWTOption func(*jwtConfig)`
  - `func WithRevocationChecker(checker revocation.Checker) JWTOption`
  - `func WithVerifyOptions(opts ...gojwt.Option) JWTOption`
  - `func JWTAuthServer[T any](secret []byte, opts ...JWTOption) endpoint.Middleware`
  - `func JWTAuthClient[T any](tokenProvider func(ctx context.Context) (string, bool)) endpoint.Middleware`

- [ ] **Step 1: Write the failing test**

```go
// go-framework/kitex/middleware/auth/jwt_test.go
package auth

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gopkg/cloud/metainfo"
	gojwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
	gojwt "github.com/byx-darwin/go-tools/go-auth/jwt"
	"github.com/byx-darwin/go-tools/go-framework/kitex/rpcerror"
)

type jwtTestClaims struct {
	gojwtlib.RegisteredClaims
	UserID string
}

var jwtTestSecret = []byte("01234567890123456789012345678901") // 33 bytes, >= HS256 min

func signTestToken(t *testing.T, userID string) string {
	t.Helper()
	claims := jwtTestClaims{
		RegisteredClaims: gojwtlib.RegisteredClaims{
			ExpiresAt: gojwtlib.NewNumericDate(time.Now().Add(time.Hour)),
		},
		UserID: userID,
	}
	token, err := gojwt.Sign(claims, jwtTestSecret)
	require.NoError(t, err)
	return token
}

func noopEndpoint(gotCtx *context.Context) func(ctx context.Context, req, resp any) error {
	return func(ctx context.Context, req, resp any) error {
		if gotCtx != nil {
			*gotCtx = ctx
		}
		return nil
	}
}

func TestJWTAuthServer_Success(t *testing.T) {
	mw := JWTAuthServer[jwtTestClaims](jwtTestSecret)

	token := signTestToken(t, "user-1")
	ctx := metainfo.WithPersistentValue(context.Background(), metaKeyJWTToken, token)
	ctx = metainfo.TransferForward(ctx) // 模拟经过一次 RPC 传输后到达 server

	var gotCtx context.Context
	wrapped := mw(noopEndpoint(&gotCtx))
	err := wrapped(ctx, nil, nil)
	require.NoError(t, err)

	claims, ok := GetClaims[jwtTestClaims](gotCtx)
	require.True(t, ok)
	assert.Equal(t, "user-1", claims.UserID)

	gotToken, ok := GetJWTToken(gotCtx)
	require.True(t, ok)
	assert.Equal(t, token, gotToken)
}

func TestJWTAuthServer_MissingToken(t *testing.T) {
	mw := JWTAuthServer[jwtTestClaims](jwtTestSecret)
	wrapped := mw(noopEndpoint(nil))

	err := wrapped(context.Background(), nil, nil)
	require.Error(t, err)

	adapter, ok := err.(*rpcerror.OopsStatusAdapter)
	require.True(t, ok)
	assert.Equal(t, int32(10020), adapter.BizStatusCode()) // frameworkerror.CodeTokenMissing
}

func TestJWTAuthServer_InvalidToken(t *testing.T) {
	mw := JWTAuthServer[jwtTestClaims](jwtTestSecret)
	ctx := metainfo.WithPersistentValue(context.Background(), metaKeyJWTToken, "not-a-jwt")
	ctx = metainfo.TransferForward(ctx)

	wrapped := mw(noopEndpoint(nil))
	err := wrapped(ctx, nil, nil)
	require.Error(t, err)

	adapter, ok := err.(*rpcerror.OopsStatusAdapter)
	require.True(t, ok)
	assert.Equal(t, int32(autherror.CodeTokenInvalid), adapter.BizStatusCode())
}

func TestJWTAuthClient_UsesTokenProvider(t *testing.T) {
	provider := func(ctx context.Context) (string, bool) { return "provided-token", true }
	mw := JWTAuthClient[jwtTestClaims](provider)

	var gotCtx context.Context
	wrapped := mw(noopEndpoint(&gotCtx))
	err := wrapped(context.Background(), nil, nil)
	require.NoError(t, err)

	// 经过一次模拟传输后，下游应能读到 persistent token。
	forwarded := metainfo.TransferForward(gotCtx)
	token, ok := metainfo.GetPersistentValue(forwarded, metaKeyJWTToken)
	require.True(t, ok)
	assert.Equal(t, "provided-token", token)
}

func TestJWTAuthClient_ReusesCtxTokenOverProvider(t *testing.T) {
	called := false
	provider := func(ctx context.Context) (string, bool) {
		called = true
		return "provider-token", true
	}
	mw := JWTAuthClient[jwtTestClaims](provider)

	ctx := SetJWTToken(context.Background(), "ctx-token")
	var gotCtx context.Context
	wrapped := mw(noopEndpoint(&gotCtx))
	err := wrapped(ctx, nil, nil)
	require.NoError(t, err)
	assert.False(t, called, "provider must not be called when ctx already has a verified token")

	forwarded := metainfo.TransferForward(gotCtx)
	token, ok := metainfo.GetPersistentValue(forwarded, metaKeyJWTToken)
	require.True(t, ok)
	assert.Equal(t, "ctx-token", token)
}

func TestJWTAuthClient_NoTokenAvailable(t *testing.T) {
	mw := JWTAuthClient[jwtTestClaims](nil)
	wrapped := mw(noopEndpoint(nil))

	err := wrapped(context.Background(), nil, nil)
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go-framework && go test ./kitex/middleware/auth/... -run TestJWTAuth -v`
Expected: FAIL — undefined `JWTAuthServer`/`JWTAuthClient`

- [ ] **Step 3: Write minimal implementation**

```go
// go-framework/kitex/middleware/auth/jwt.go

package auth

import (
	"context"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/kitex/pkg/endpoint"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
	gojwt "github.com/byx-darwin/go-tools/go-auth/jwt"
	"github.com/byx-darwin/go-tools/go-auth/revocation"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
)

// jwtConfig 存储 JWTAuthServer 配置选项。
type jwtConfig struct {
	revocationChecker revocation.Checker
	verifyOptions     []gojwt.Option
}

// JWTOption 定义 JWTAuthServer 配置选项函数。
type JWTOption func(*jwtConfig)

// WithRevocationChecker 设置撤销检查器。验证签名成功后，若配置了
// checker，会额外查询撤销表；命中则返回 autherror.ErrTokenRevoked。
// 未设置时行为与不启用撤销检查完全一致。语义与
// go-framework/hertz/middleware.WithRevocationChecker 一致。
func WithRevocationChecker(checker revocation.Checker) JWTOption {
	return func(c *jwtConfig) { c.revocationChecker = checker }
}

// WithVerifyOptions 透传 go-auth/jwt 的 Verify 选项（如
// gojwt.WithExpectedIssuer）给内部 gojwt.Verify 调用。语义与
// go-framework/hertz/middleware.WithVerifyOptions 一致。
func WithVerifyOptions(opts ...gojwt.Option) JWTOption {
	return func(c *jwtConfig) { c.verifyOptions = append(c.verifyOptions, opts...) }
}

func applyJWTOptions(opts []JWTOption) jwtConfig {
	var cfg jwtConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// JWTAuthServer 返回 Kitex Server 端 JWT 鉴权中间件。
// 从 incoming metainfo（persistent key）读取 token，验证签名后将 claims
// 与原始 token 注入 ctx，供业务代码与 JWTAuthClient（透传到下游）使用。
// T 必须嵌入 jwt.RegisteredClaims。
//
// 使用方式：
//
//	server.WithMiddleware(auth.JWTAuthServer[UserClaims](secret))
//	claims, ok := auth.GetClaims[UserClaims](ctx)
func JWTAuthServer[T any](secret []byte, opts ...JWTOption) endpoint.Middleware {
	cfg := applyJWTOptions(opts)

	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			token, ok := metainfo.GetPersistentValue(ctx, metaKeyJWTToken)
			if !ok || token == "" {
				return bizError(frameworkerror.ErrTokenMissing.Errorf("missing jwt token in metainfo"))
			}

			claims, err := gojwt.Verify[T](token, secret, cfg.verifyOptions...)
			if err != nil {
				return bizError(err)
			}

			if cfg.revocationChecker != nil {
				if jti, ok := gojwt.ExtractJTI(claims); ok {
					revoked, rerr := cfg.revocationChecker.IsRevoked(ctx, jti)
					if rerr != nil {
						return bizError(autherror.ErrJWTVerifyFailed.Wrap(rerr))
					}
					if revoked {
						return bizError(autherror.ErrTokenRevoked.Errorf("token revoked"))
					}
				}
			}

			ctx = SetClaims(ctx, claims)
			ctx = SetJWTToken(ctx, token)
			return next(ctx, req, resp)
		}
	}
}

// JWTAuthClient 返回 Kitex Client 端 JWT 身份注入中间件。
// 优先复用 ctx 中已由 JWTAuthServer 校验通过的原始 token（多跳场景，
// 如 B 调 C 复用 A 传给 B 的身份，不重新签发）；ctx 中没有时调用
// tokenProvider 获取 token。token 通过 metainfo.WithPersistentValue
// 写入 outgoing metadata，随调用链自动持久透传到所有下游，无需业务代码
// 在中间服务手动转发。
//
// tokenProvider 允许为 nil（例如纯透传场景、ctx 中必然已有 token）；
// ctx 与 provider 均未提供 token 时返回错误。
//
// 使用方式：
//
//	client.WithMiddleware(auth.JWTAuthClient[UserClaims](func(ctx context.Context) (string, bool) {
//	    return getTokenFromSomewhere(ctx)
//	}))
func JWTAuthClient[T any](tokenProvider func(ctx context.Context) (string, bool)) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			token, ok := GetJWTToken(ctx)
			if !ok || token == "" {
				if tokenProvider == nil {
					return bizError(frameworkerror.ErrTokenMissing.Errorf("no token in context and no tokenProvider configured"))
				}
				token, ok = tokenProvider(ctx)
				if !ok || token == "" {
					return bizError(frameworkerror.ErrTokenMissing.Errorf("tokenProvider returned no token"))
				}
			}

			ctx = metainfo.WithPersistentValue(ctx, metaKeyJWTToken, token)
			return next(ctx, req, resp)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go-framework && go test ./kitex/middleware/auth/... -v`
Expected: PASS（Task 1 + Task 2 全部测试）

- [ ] **Step 5: Commit**

```bash
git add go-framework/kitex/middleware/auth/jwt.go go-framework/kitex/middleware/auth/jwt_test.go
git commit -m "feat(kitex): add JWTAuthServer/JWTAuthClient middleware"
```

---

### Task 3: SessionAuthServer / SessionAuthClient

**Files:**
- Create: `go-framework/kitex/middleware/auth/session.go`
- Test: `go-framework/kitex/middleware/auth/session_test.go`

**Interfaces:**
- Consumes: Task 1 的 `SetSession`/`GetSession`/`SetSessionID`/`GetSessionID`/`bizError`/`metaKeySessionID`
- Produces:
  - `func SessionAuthServer(store session.Store) endpoint.Middleware`
  - `func SessionAuthClient(sessionIDProvider func(ctx context.Context) (string, bool)) endpoint.Middleware`

- [ ] **Step 1: Write the failing test**

```go
// go-framework/kitex/middleware/auth/session_test.go
package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/byx-darwin/go-tools/go-auth/session"
)

// memorySessionStore 是仅用于测试的最小 session.Store 实现。
type memorySessionStore struct {
	sessions map[string]*session.Session
	getErr   error
}

func (m *memorySessionStore) Get(_ context.Context, sessionID string) (*session.Session, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.sessions[sessionID], nil
}
func (m *memorySessionStore) Save(_ context.Context, s *session.Session) error {
	m.sessions[s.ID] = s
	return nil
}
func (m *memorySessionStore) Delete(_ context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}
func (m *memorySessionStore) Exists(_ context.Context, sessionID string) (bool, error) {
	_, ok := m.sessions[sessionID]
	return ok, nil
}

func TestSessionAuthServer_Success(t *testing.T) {
	store := &memorySessionStore{sessions: map[string]*session.Session{
		"sess-1": {ID: "sess-1", UserUUID: "user-1", ExpiresAt: time.Now().Add(time.Hour)},
	}}
	mw := SessionAuthServer(store)

	ctx := metainfo.WithPersistentValue(context.Background(), metaKeySessionID, "sess-1")
	ctx = metainfo.TransferForward(ctx)

	var gotCtx context.Context
	wrapped := mw(noopEndpoint(&gotCtx))
	err := wrapped(ctx, nil, nil)
	require.NoError(t, err)

	s, ok := GetSession(gotCtx)
	require.True(t, ok)
	assert.Equal(t, "user-1", s.UserUUID)
}

func TestSessionAuthServer_MissingSessionID(t *testing.T) {
	store := &memorySessionStore{sessions: map[string]*session.Session{}}
	mw := SessionAuthServer(store)

	err := mw(noopEndpoint(nil))(context.Background(), nil, nil)
	require.Error(t, err)
}

func TestSessionAuthServer_SessionNotFound(t *testing.T) {
	store := &memorySessionStore{sessions: map[string]*session.Session{}}
	mw := SessionAuthServer(store)

	ctx := metainfo.WithPersistentValue(context.Background(), metaKeySessionID, "missing")
	ctx = metainfo.TransferForward(ctx)

	err := mw(noopEndpoint(nil))(ctx, nil, nil)
	require.Error(t, err)
}

func TestSessionAuthServer_StoreError(t *testing.T) {
	store := &memorySessionStore{sessions: map[string]*session.Session{}, getErr: errors.New("store down")}
	mw := SessionAuthServer(store)

	ctx := metainfo.WithPersistentValue(context.Background(), metaKeySessionID, "sess-1")
	ctx = metainfo.TransferForward(ctx)

	err := mw(noopEndpoint(nil))(ctx, nil, nil)
	require.Error(t, err)
}

func TestSessionAuthClient_UsesProviderAndCtxReuse(t *testing.T) {
	provider := func(ctx context.Context) (string, bool) { return "provided-sess", true }
	mw := SessionAuthClient(provider)

	var gotCtx context.Context
	err := mw(noopEndpoint(&gotCtx))(context.Background(), nil, nil)
	require.NoError(t, err)

	forwarded := metainfo.TransferForward(gotCtx)
	id, ok := metainfo.GetPersistentValue(forwarded, metaKeySessionID)
	require.True(t, ok)
	assert.Equal(t, "provided-sess", id)

	// ctx 已有 session id 时不再调用 provider。
	called := false
	provider2 := func(ctx context.Context) (string, bool) { called = true; return "x", true }
	mw2 := SessionAuthClient(provider2)
	ctx := SetSessionID(context.Background(), "ctx-sess")
	err = mw2(noopEndpoint(nil))(ctx, nil, nil)
	require.NoError(t, err)
	assert.False(t, called)
}

func TestSessionAuthClient_NoSessionAvailable(t *testing.T) {
	mw := SessionAuthClient(nil)
	err := mw(noopEndpoint(nil))(context.Background(), nil, nil)
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go-framework && go test ./kitex/middleware/auth/... -run TestSessionAuth -v`
Expected: FAIL — undefined `SessionAuthServer`/`SessionAuthClient`

- [ ] **Step 3: Write minimal implementation**

```go
// go-framework/kitex/middleware/auth/session.go

package auth

import (
	"context"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/kitex/pkg/endpoint"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
	"github.com/byx-darwin/go-tools/go-auth/session"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
)

// SessionAuthServer 返回 Kitex Server 端 Session 鉴权中间件。
// 从 incoming metainfo（persistent key）读取 Session ID，校验通过后将
// Session 与 Session ID 注入 ctx，供业务代码与 SessionAuthClient
// （透传到下游）使用。
//
// 使用方式：
//
//	server.WithMiddleware(auth.SessionAuthServer(sessionStore))
//	s, ok := auth.GetSession(ctx)
func SessionAuthServer(store session.Store) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			sessionID, ok := metainfo.GetPersistentValue(ctx, metaKeySessionID)
			if !ok || sessionID == "" {
				return bizError(autherror.ErrSessionInvalid.Errorf("missing session id in metainfo"))
			}

			s, err := store.Get(ctx, sessionID)
			if err != nil {
				return bizError(frameworkerror.ErrSystem.Wrap(err))
			}
			if s == nil {
				return bizError(autherror.ErrSessionInvalid.Errorf("session not found"))
			}

			ctx = SetSession(ctx, s)
			ctx = SetSessionID(ctx, sessionID)
			return next(ctx, req, resp)
		}
	}
}

// SessionAuthClient 返回 Kitex Client 端 Session 身份注入中间件。
// 优先复用 ctx 中已由 SessionAuthServer 校验通过的 Session ID；ctx 中
// 没有时调用 sessionIDProvider 获取。Session ID 通过
// metainfo.WithPersistentValue 写入，随调用链自动持久透传到所有下游。
//
// sessionIDProvider 允许为 nil；ctx 与 provider 均未提供时返回错误。
func SessionAuthClient(sessionIDProvider func(ctx context.Context) (string, bool)) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			sessionID, ok := GetSessionID(ctx)
			if !ok || sessionID == "" {
				if sessionIDProvider == nil {
					return bizError(autherror.ErrSessionInvalid.Errorf("no session id in context and no sessionIDProvider configured"))
				}
				sessionID, ok = sessionIDProvider(ctx)
				if !ok || sessionID == "" {
					return bizError(autherror.ErrSessionInvalid.Errorf("sessionIDProvider returned no session id"))
				}
			}

			ctx = metainfo.WithPersistentValue(ctx, metaKeySessionID, sessionID)
			return next(ctx, req, resp)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go-framework && go test ./kitex/middleware/auth/... -v`
Expected: PASS（Task 1-3 全部测试）

- [ ] **Step 5: Commit**

```bash
git add go-framework/kitex/middleware/auth/session.go go-framework/kitex/middleware/auth/session_test.go
git commit -m "feat(kitex): add SessionAuthServer/SessionAuthClient middleware"
```

---

### Task 4: DeviceAuthServer / DeviceAuthClient

**Files:**
- Create: `go-framework/kitex/middleware/auth/device.go`
- Test: `go-framework/kitex/middleware/auth/device_test.go`

**Interfaces:**
- Consumes: Task 1 的 `bizError`/`metaKeyDeviceUserUUID`/`metaKeyDeviceID`/`metaKeyDeviceJTI`
- Produces:
  - `type DeviceClaimsProvider func(ctx context.Context) (userUUID, deviceID, jti string, ok bool)`
  - `func DeviceAuthServer(store device.Store) endpoint.Middleware`
  - `func DeviceAuthClient(extract DeviceClaimsProvider) endpoint.Middleware`

- [ ] **Step 1: Write the failing test**

```go
// go-framework/kitex/middleware/auth/device_test.go
package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/byx-darwin/go-tools/go-auth/device"
)

// memoryDeviceStore 是仅用于测试的最小 device.Store 实现。
type memoryDeviceStore struct {
	valid     bool
	checkErr  error
	lastCheck [3]string // userUUID, deviceID, jti
}

func (m *memoryDeviceStore) AddDevice(_ context.Context, _, _, _ string, _ int) ([]device.Device, error) {
	return nil, nil
}
func (m *memoryDeviceStore) CheckDevice(_ context.Context, userUUID, deviceID, jti string) (bool, error) {
	m.lastCheck = [3]string{userUUID, deviceID, jti}
	if m.checkErr != nil {
		return false, m.checkErr
	}
	return m.valid, nil
}
func (m *memoryDeviceStore) RemoveDevice(_ context.Context, _, _ string) error       { return nil }
func (m *memoryDeviceStore) RemoveAllDevices(_ context.Context, _ string) error      { return nil }
func (m *memoryDeviceStore) ListDevices(_ context.Context, _ string) ([]device.Device, error) {
	return nil, nil
}

func withDeviceMetainfo(ctx context.Context, userUUID, deviceID, jti string) context.Context {
	if userUUID != "" {
		ctx = metainfo.WithPersistentValue(ctx, metaKeyDeviceUserUUID, userUUID)
	}
	if deviceID != "" {
		ctx = metainfo.WithPersistentValue(ctx, metaKeyDeviceID, deviceID)
	}
	if jti != "" {
		ctx = metainfo.WithPersistentValue(ctx, metaKeyDeviceJTI, jti)
	}
	return metainfo.TransferForward(ctx)
}

func TestDeviceAuthServer_Success(t *testing.T) {
	store := &memoryDeviceStore{valid: true}
	mw := DeviceAuthServer(store)

	ctx := withDeviceMetainfo(context.Background(), "user-1", "device-1", "jti-1")
	err := mw(noopEndpoint(nil))(ctx, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, [3]string{"user-1", "device-1", "jti-1"}, store.lastCheck)
}

func TestDeviceAuthServer_IncompleteIdentity(t *testing.T) {
	store := &memoryDeviceStore{valid: true}
	mw := DeviceAuthServer(store)

	ctx := withDeviceMetainfo(context.Background(), "user-1", "", "jti-1")
	err := mw(noopEndpoint(nil))(ctx, nil, nil)
	require.Error(t, err)
}

func TestDeviceAuthServer_Kicked(t *testing.T) {
	store := &memoryDeviceStore{valid: false}
	mw := DeviceAuthServer(store)

	ctx := withDeviceMetainfo(context.Background(), "user-1", "device-1", "jti-1")
	err := mw(noopEndpoint(nil))(ctx, nil, nil)
	require.Error(t, err)
}

func TestDeviceAuthServer_StoreError(t *testing.T) {
	store := &memoryDeviceStore{checkErr: errors.New("store down")}
	mw := DeviceAuthServer(store)

	ctx := withDeviceMetainfo(context.Background(), "user-1", "device-1", "jti-1")
	err := mw(noopEndpoint(nil))(ctx, nil, nil)
	require.Error(t, err)
}

func TestDeviceAuthClient_InjectsMetainfo(t *testing.T) {
	extract := func(ctx context.Context) (string, string, string, bool) {
		return "user-1", "device-1", "jti-1", true
	}
	mw := DeviceAuthClient(extract)

	var gotCtx context.Context
	err := mw(noopEndpoint(&gotCtx))(context.Background(), nil, nil)
	require.NoError(t, err)

	forwarded := metainfo.TransferForward(gotCtx)
	userUUID, ok := metainfo.GetPersistentValue(forwarded, metaKeyDeviceUserUUID)
	require.True(t, ok)
	assert.Equal(t, "user-1", userUUID)
	deviceID, ok := metainfo.GetPersistentValue(forwarded, metaKeyDeviceID)
	require.True(t, ok)
	assert.Equal(t, "device-1", deviceID)
	jti, ok := metainfo.GetPersistentValue(forwarded, metaKeyDeviceJTI)
	require.True(t, ok)
	assert.Equal(t, "jti-1", jti)
}

func TestDeviceAuthClient_NilExtractor(t *testing.T) {
	mw := DeviceAuthClient(nil)
	err := mw(noopEndpoint(nil))(context.Background(), nil, nil)
	require.Error(t, err)
}

func TestDeviceAuthClient_ExtractorIncomplete(t *testing.T) {
	extract := func(ctx context.Context) (string, string, string, bool) {
		return "user-1", "", "", true
	}
	mw := DeviceAuthClient(extract)
	err := mw(noopEndpoint(nil))(context.Background(), nil, nil)
	require.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go-framework && go test ./kitex/middleware/auth/... -run TestDeviceAuth -v`
Expected: FAIL — undefined `DeviceAuthServer`/`DeviceAuthClient`

- [ ] **Step 3: Write minimal implementation**

```go
// go-framework/kitex/middleware/auth/device.go

package auth

import (
	"context"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/kitex/pkg/endpoint"

	"github.com/byx-darwin/go-tools/go-auth/device"
	autherror "github.com/byx-darwin/go-tools/go-auth/error"
)

// DeviceClaimsProvider 从 ctx 提取设备身份三元组（userUUID、deviceID、
// jti），用于 DeviceAuthClient 向下游透传。实现方通常从
// auth.GetClaims 取出本地已验证的 JWT claims 中提取这三个字段。
type DeviceClaimsProvider func(ctx context.Context) (userUUID, deviceID, jti string, ok bool)

// DeviceAuthServer 返回 Kitex Server 端设备会话校验中间件。
// 从 incoming metainfo（persistent key）读取 userUUID/deviceID/jti，
// 调用 device.Store.CheckDevice 验证设备会话是否有效。
//
// 使用方式：
//
//	server.WithMiddleware(auth.DeviceAuthServer(deviceStore))
func DeviceAuthServer(store device.Store) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			userUUID, ok1 := metainfo.GetPersistentValue(ctx, metaKeyDeviceUserUUID)
			deviceID, ok2 := metainfo.GetPersistentValue(ctx, metaKeyDeviceID)
			jti, ok3 := metainfo.GetPersistentValue(ctx, metaKeyDeviceJTI)
			if !ok1 || !ok2 || !ok3 || userUUID == "" || deviceID == "" || jti == "" {
				return bizError(autherror.ErrTokenInvalid.Errorf("incomplete device identity in metainfo"))
			}

			valid, err := store.CheckDevice(ctx, userUUID, deviceID, jti)
			if err != nil {
				return bizError(autherror.ErrJWTVerifyFailed.Wrap(err))
			}
			if !valid {
				return bizError(autherror.ErrDeviceKicked.Errorf("device kicked"))
			}

			return next(ctx, req, resp)
		}
	}
}

// DeviceAuthClient 返回 Kitex Client 端设备身份注入中间件。
// 调用 extract 从 ctx 提取设备身份三元组，通过
// metainfo.WithPersistentValue 写入，随调用链自动持久透传到所有下游。
//
// extract 不允许为 nil；提取结果不完整时返回错误。
func DeviceAuthClient(extract DeviceClaimsProvider) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			if extract == nil {
				return bizError(autherror.ErrTokenInvalid.Errorf("no DeviceClaimsProvider configured"))
			}

			userUUID, deviceID, jti, ok := extract(ctx)
			if !ok || userUUID == "" || deviceID == "" || jti == "" {
				return bizError(autherror.ErrTokenInvalid.Errorf("incomplete device claims from provider"))
			}

			ctx = metainfo.WithPersistentValue(ctx, metaKeyDeviceUserUUID, userUUID)
			ctx = metainfo.WithPersistentValue(ctx, metaKeyDeviceID, deviceID)
			ctx = metainfo.WithPersistentValue(ctx, metaKeyDeviceJTI, jti)
			return next(ctx, req, resp)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go-framework && go test ./kitex/middleware/auth/... -v`
Expected: PASS（Task 1-4 全部测试）

- [ ] **Step 5: Commit**

```bash
git add go-framework/kitex/middleware/auth/device.go go-framework/kitex/middleware/auth/device_test.go
git commit -m "feat(kitex): add DeviceAuthServer/DeviceAuthClient middleware"
```

---

### Task 5: Recovery panic 恢复中间件

**Files:**
- Create: `go-framework/kitex/middleware/auth/recovery.go`
- Test: `go-framework/kitex/middleware/auth/recovery_test.go`

**Interfaces:**
- Consumes: Task 1 的 `bizError`
- Produces: `func Recovery() endpoint.Middleware`

- [ ] **Step 1: Write the failing test**

```go
// go-framework/kitex/middleware/auth/recovery_test.go
package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/byx-darwin/go-tools/go-framework/kitex/rpcerror"
)

func TestRecovery_CatchesPanic(t *testing.T) {
	mw := Recovery()
	panicking := func(ctx context.Context, req, resp any) error {
		panic("boom")
	}

	wrapped := mw(panicking)

	var err error
	assert.NotPanics(t, func() {
		err = wrapped(context.Background(), nil, nil)
	})
	require.Error(t, err)

	adapter, ok := err.(*rpcerror.OopsStatusAdapter)
	require.True(t, ok)
	assert.Contains(t, adapter.Error(), "boom")
}

func TestRecovery_PassesThroughSuccess(t *testing.T) {
	mw := Recovery()
	ok := func(ctx context.Context, req, resp any) error { return nil }

	wrapped := mw(ok)
	err := wrapped(context.Background(), nil, nil)
	require.NoError(t, err)
}

func TestRecovery_PassesThroughError(t *testing.T) {
	mw := Recovery()
	failing := func(ctx context.Context, req, resp any) error { return assert.AnError }

	wrapped := mw(failing)
	err := wrapped(context.Background(), nil, nil)
	require.Error(t, err)
	assert.Equal(t, assert.AnError, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go-framework && go test ./kitex/middleware/auth/... -run TestRecovery -v`
Expected: FAIL — undefined `Recovery`

- [ ] **Step 3: Write minimal implementation**

```go
// go-framework/kitex/middleware/auth/recovery.go

package auth

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/cloudwego/kitex/pkg/endpoint"

	"github.com/byx-darwin/go-tools/go-common/log"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
)

// Recovery 返回 Kitex Server 端 panic 恢复中间件。
// 捕获 handler 链路中的 panic，结构化记录日志（含 stack），转换为
// frameworkerror.ErrSystem 并通过 bizError 包装返回，避免进程崩溃。
// 仅提供 Server 端实现：Client 端是 RPC 发起方，panic 通常发生在业务
// handler 内部（Server 侧），Client 端无需 recovery。
//
// 使用方式：
//
//	server.WithMiddleware(auth.Recovery())
func Recovery() endpoint.Middleware {
	rpcLog := log.L().WithCategory(log.CategoryRPC)

	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) (err error) {
			defer func() {
				if r := recover(); r != nil {
					stack := debug.Stack()
					rpcLog.ErrorContext(ctx, "rpc panic recovered", fmt.Errorf("%v", r),
						"panic", fmt.Sprintf("%v", r),
						"stack", string(stack),
					)
					err = bizError(frameworkerror.ErrSystem.Errorf("rpc panic recovered: %v", r))
				}
			}()

			return next(ctx, req, resp)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go-framework && go test ./kitex/middleware/auth/... -v`
Expected: PASS（Task 1-5 全部测试）

- [ ] **Step 5: Commit**

```bash
git add go-framework/kitex/middleware/auth/recovery.go go-framework/kitex/middleware/auth/recovery_test.go
git commit -m "feat(kitex): add Recovery panic-handling middleware"
```

---

### Task 6: 包级文档与全量验证

**Files:**
- Create: `go-framework/kitex/middleware/auth/doc.go`

**Interfaces:**
- Consumes: Task 1-5 全部导出符号（仅用于文档示例，不新增代码接口）

- [ ] **Step 1: Write package doc**

```go
// go-framework/kitex/middleware/auth/doc.go

// Package auth 提供 Kitex RPC 鉴权中间件。
//
// # 快速开始
//
// JWT 鉴权（Server 端校验 + Client 端注入）：
//
//	// 服务端（被调用方）
//	svr := xxxsvr.NewServer(handler,
//	    server.WithMiddleware(auth.JWTAuthServer[UserClaims](secret)),
//	)
//	claims, ok := auth.GetClaims[UserClaims](ctx)
//
//	// 客户端（调用方）
//	cli, _ := xxxcli.NewClient("target-service",
//	    client.WithMiddleware(auth.JWTAuthClient[UserClaims](func(ctx context.Context) (string, bool) {
//	        return getTokenFromSomewhere(ctx)
//	    })),
//	)
//
// Session / Device 鉴权用法对称，分别见 SessionAuthServer/SessionAuthClient
// 与 DeviceAuthServer/DeviceAuthClient。
//
// Panic 恢复：
//
//	server.WithMiddleware(auth.Recovery())
//
// # 身份透传
//
// 三种鉴权中间件均使用 github.com/bytedance/gopkg/cloud/metainfo 的
// WithPersistentValue/GetPersistentValue 在 RPC 调用链路中传递身份。
// Persistent 语义保证 A→B→C 调用链中，B 收到 A 传来的身份后，B 调 C 时
// 该身份自动继续透传，无需业务代码在 B 中手动转发。
//
// # 错误处理
//
// 鉴权失败统一通过 go-framework/kitex/rpcerror.OopsStatusAdapter 包装为
// Kitex BizStatusErrorIface 返回，错误码复用 go-auth/error（Token/Session/
// Device 相关）与 go-framework/error（CodeTokenMissing 等）。
package auth
```

- [ ] **Step 2: Run full package build + test + vet**

Run: `cd go-framework && go build ./kitex/middleware/auth/... && go vet ./kitex/middleware/auth/... && go test ./kitex/middleware/auth/... -v -count=1`
Expected: 全部 PASS，无编译错误

- [ ] **Step 3: Run golangci-lint for go-framework module**

Run: `golangci-lint run --timeout=5m ./go-framework/...`
Expected: 无新增 lint 问题（若发现 revive godoc / errcheck 等问题，回到对应 Task 文件修复后重跑本命令）

- [ ] **Step 4: Run full workspace build + vet + test（回归其他模块）**

Run: `go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... && go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... && go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1`
Expected: 全部 PASS，无回归

- [ ] **Step 5: Commit**

```bash
git add go-framework/kitex/middleware/auth/doc.go
git commit -m "docs(kitex): add auth package overview doc"
```

---

## Self-Review Notes（写计划时已完成，供执行者参考）

- **Spec coverage**：Spec 六项决策（范围/挂载方向/错误路由/Recovery 设计/包结构/透传语义）分别对应 Task 2-5（JWT/Session/Device/Recovery）与 Task 1（透传基础设施）、Task 6（包结构文档化）。无遗漏。
- **Type consistency**：`GetClaims[T]`/`SetClaims[T]`（Task 1）与 `JWTAuthServer[T]`（Task 2）的类型参数 T 一致；`metaKeyJWTToken` 等 5 个 metainfo key 常量定义于 Task 1，Task 2-4 均直接引用，无重复定义。`bizError` 统一由 Task 1 提供，Task 2-5 均复用。
- **Placeholder scan**：无 TBD / "add error handling" 类占位描述，所有 Step 均含完整可运行代码。
