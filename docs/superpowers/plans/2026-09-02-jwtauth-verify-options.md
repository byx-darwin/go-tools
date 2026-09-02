# JWTAuth 中间件透传 Verify 选项 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `go-framework/hertz/middleware` 的 `JWTAuth[T]` 中间件能够透传 `go-auth/jwt` 的 Verify 选项（如 `WithExpectedIssuer`），使下游 Hertz 服务可以在中间件层启用 issuer 校验。

**Architecture:** 在 `jwtAuthConfig` 新增 `verifyOptions []gojwt.Option` 字段，新增对应的 `WithVerifyOptions(opts ...gojwt.Option) JWTAuthOption`（与既有 `WithRevocationChecker` 风格一致），并将其透传给内部 `gojwt.Verify[T]` 调用。这是通用透传设计（而非专用 `WithExpectedIssuer` 快捷方式），可覆盖当前及未来所有 `go-auth/jwt` Verify 选项。

**Tech Stack:** Go 1.25+，Hertz（`github.com/cloudwego/hertz`），`go-auth/jwt`（对外暴露为 `gojwt` 别名的包，注意与 `github.com/golang-jwt/jwt/v5` 的测试文件内 `gojwt` 别名指代不同包——见 Task 1 说明），testify（assert/require）。

**Spec:** `docs/superpowers/specs/2026-09-02-jwtauth-verify-options-design.md`

## Global Constraints

- 密钥参数变更、既有调用签名不受影响：`JWTAuth[T](secret []byte, opts ...JWTAuthOption)` 签名不变，新增能力通过新增 Option 暴露（非破坏性）。
- 所有导出符号（`WithVerifyOptions`）必须有 `// WithVerifyOptions ...` 格式的 godoc 注释（revive 规则）。
- 新增测试须使用真实 `authjwt.Sign`/`authjwt.Verify` 端到端验证，不 mock JWT 签发/校验本身（沿用 `jwt_auth_test.go` 现有风格）。
- 未配置 `WithVerifyOptions` 时行为必须与现状完全一致（回归保护，`jwt_auth_test.go` 中 `TestJWTAuth_NoRevocationChecker_BehaviorUnchanged` 已建立此类回归测试的先例，本次比照新增一个）。

---

### Task 1: `JWTAuth[T]` 新增 `WithVerifyOptions` 并透传给 `gojwt.Verify`

**Files:**
- Modify: `go-framework/hertz/middleware/jwt_auth.go`
- Test: `go-framework/hertz/middleware/jwt_auth_test.go`

**Interfaces:**
- Consumes：`gojwt.Verify[T](token string, secret any, opts ...gojwt.Option) (T, error)`（已存在于 `go-auth/jwt` 包，`jwt_auth.go` 中该包以别名 `gojwt "github.com/byx-darwin/go-tools/go-auth/jwt"` 导入 — 注意：这与测试文件 `jwt_auth_test.go` 中 `gojwt "github.com/golang-jwt/jwt/v5"` 是**不同的包**，仅别名相同，本任务全程在 `jwt_auth.go` 内操作，沿用其已有别名约定）；`gojwt.Option`（`go-auth/jwt` 包导出类型，`func(*config)`）；`gojwt.WithExpectedIssuer(issuer string) gojwt.Option`（已存在）。
- Produces：`WithVerifyOptions(opts ...gojwt.Option) JWTAuthOption`，供本任务测试及后续下游服务使用；`jwtAuthConfig.verifyOptions []gojwt.Option` 字段（包内私有，供 `JWTAuth[T]` 内部读取）。

- [ ] **Step 1: 写失败测试 — issuer 校验拒绝不匹配的 token**

在 `go-framework/hertz/middleware/jwt_auth_test.go` 末尾（`TestJWTAuth_RevocationChecker_NoJTI_FailOpen` 之后）追加：

```go
// ── Verify options passthrough ──

func issueTestTokenWithIssuer(t *testing.T, secret []byte, issuer string) string {
	t.Helper()
	claims := testClaims{
		UserUUID: "user-123",
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   "user-123",
			Issuer:    issuer,
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token, err := authjwt.Sign(claims, secret)
	require.NoError(t, err)
	return token
}

func TestJWTAuth_VerifyOptions_ExpectedIssuer_Mismatch(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	token := issueTestTokenWithIssuer(t, secret, "wrong-issuer")

	engine := newTestEngine()
	engine.Use(JWTAuth[testClaims](secret, WithVerifyOptions(authjwt.WithExpectedIssuer("expected-issuer"))))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode())
}

func TestJWTAuth_VerifyOptions_ExpectedIssuer_Match(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	token := issueTestTokenWithIssuer(t, secret, "expected-issuer")

	engine := newTestEngine()
	engine.Use(JWTAuth[testClaims](secret, WithVerifyOptions(authjwt.WithExpectedIssuer("expected-issuer"))))
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

func TestJWTAuth_NoVerifyOptions_BehaviorUnchanged(t *testing.T) {
	// 未配置 WithVerifyOptions 时，行为应与旧版完全一致（回归测试）。
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	token := issueTestTokenWithIssuer(t, secret, "any-issuer")

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

- [ ] **Step 2: 运行测试确认失败**

Run: `cd go-framework && go test ./hertz/middleware/... -run TestJWTAuth_VerifyOptions -v`
Expected: FAIL — `WithVerifyOptions` 未定义（编译错误：`undefined: WithVerifyOptions`）

- [ ] **Step 3: 实现 `WithVerifyOptions` 并透传**

修改 `go-framework/hertz/middleware/jwt_auth.go`：

1. `jwtAuthConfig` 结构体新增字段：

```go
// jwtAuthConfig 存储 JWTAuth 配置选项。
type jwtAuthConfig struct {
	revocationChecker revocation.Checker
	verifyOptions     []gojwt.Option
}
```

2. 在 `WithRevocationChecker` 之后新增：

```go
// WithVerifyOptions 透传 go-auth/jwt 的 Verify 选项（如 WithExpectedIssuer）
// 给内部 gojwt.Verify 调用，使下游服务可在中间件层启用 issuer 校验等能力。
// 可多次调用或一次传入多个选项，按顺序追加，语义与直接调用
// gojwt.Verify(token, secret, opts...) 完全一致。
//
// 用例：
//
//	engine.Use(middleware.JWTAuth[UserClaims](secret,
//	    middleware.WithVerifyOptions(gojwt.WithExpectedIssuer("myapp"))))
func WithVerifyOptions(opts ...gojwt.Option) JWTAuthOption {
	return func(c *jwtAuthConfig) { c.verifyOptions = append(c.verifyOptions, opts...) }
}
```

3. `JWTAuth[T]` 内部调用处，将：

```go
		claims, err := gojwt.Verify[T](token, secret)
```

改为：

```go
		claims, err := gojwt.Verify[T](token, secret, cfg.verifyOptions...)
```

4. 更新 `JWTAuth` 函数 godoc（在现有 godoc "可通过 WithRevocationChecker 启用撤销检查。" 之后追加一句）：

```go
// JWTAuth 返回 JWT 认证中间件。
// 从 Authorization Bearer 头解析 token，验证签名，将 claims 注入 RequestContext。
// T 必须嵌入 jwt.RegisteredClaims。可通过 WithRevocationChecker 启用撤销检查，
// 通过 WithVerifyOptions 透传 go-auth/jwt 的 Verify 选项（如 issuer 校验）。
//
// 使用方式：
//
//	engine.Use(middleware.JWTAuth[UserClaims](secret))
//	claims, ok := middleware.GetClaims[UserClaims](c)
//
//	// 启用 issuer 校验：
//	engine.Use(middleware.JWTAuth[UserClaims](secret,
//	    middleware.WithVerifyOptions(gojwt.WithExpectedIssuer("myapp"))))
func JWTAuth[T any](secret []byte, opts ...JWTAuthOption) app.HandlerFunc {
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd go-framework && go test ./hertz/middleware/... -run TestJWTAuth -v`
Expected: PASS（含新增 3 个用例及全部既有用例，全部 PASS）

- [ ] **Step 5: 运行完整模块测试与静态检查**

Run:
```bash
go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...
go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...
gofmt -l go-framework/hertz/middleware/jwt_auth.go go-framework/hertz/middleware/jwt_auth_test.go
golangci-lint run --timeout=5m ./go-framework/...
go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1
```
Expected: 全部通过，`gofmt -l` 无输出，`golangci-lint` 0 issues

- [ ] **Step 6: Commit**

```bash
git add go-framework/hertz/middleware/jwt_auth.go go-framework/hertz/middleware/jwt_auth_test.go
git commit -m "feat(hertz): JWTAuth 中间件支持透传 Verify 选项 (WithVerifyOptions)

Closes #85"
```

---

## Self-Review Notes

- **Spec coverage**：设计文档三项要求（`WithVerifyOptions` 通用透传、单元测试覆盖 issuer 不匹配拒绝、godoc 更新）均由 Task 1 覆盖；`example/handler/auth_jwt.go` 未使用 `middleware.JWTAuth`（直接调用 `jwt.Verify`），验收标准该项非强制且不适用，不建任务。
- **Placeholder scan**：无 TBD/TODO，所有测试代码均为完整可运行代码。
- **Type consistency**：`WithVerifyOptions(opts ...gojwt.Option) JWTAuthOption` 与 `jwtAuthConfig.verifyOptions []gojwt.Option` 类型一致；测试中 `authjwt.WithExpectedIssuer` 对应 `go-auth/jwt` 包导出函数，与 `jwt_auth.go` 内部 `gojwt` 别名指向同一包（`go-auth/jwt`），仅测试文件因需要构造真实 token 而同时导入了 `github.com/golang-jwt/jwt/v5`（也别名为 `gojwt`）——两个 `gojwt` 别名在不同文件中指代不同包，均沿用各自文件既有约定，未引入新的命名冲突。
