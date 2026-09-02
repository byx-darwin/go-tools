# JWTAuth 中间件透传 Verify 选项 — 设计（Bounded）

**Issue:** #85
**分类:** Bounded（单文件改动，模式与既有 `WithRevocationChecker` 一致）

## 背景

`go-framework/hertz/middleware/jwt_auth.go` 的 `JWTAuth[T]` 中间件硬编码调用
`gojwt.Verify[T](token, secret)`，不接受也不透传任何 `jwt.Option`，导致
`go-auth/jwt/options.go` 中已存在的 `WithExpectedIssuer` 等 Verify 选项无法在
中间件层启用，使用 `JWTAuth` 的下游 Hertz 服务无法开启 issuer 校验。

## API 设计

采用**通用选项透传**（而非专用 `WithExpectedIssuer` 快捷方式）：

- `jwtAuthConfig` 新增字段 `verifyOptions []gojwt.Option`
- 新增 `WithVerifyOptions(opts ...gojwt.Option) JWTAuthOption`，追加到
  `cfg.verifyOptions`，与现有 `WithRevocationChecker` 并列，风格一致
- `JWTAuth[T]` 内部调用改为
  `gojwt.Verify[T](token, secret, cfg.verifyOptions...)`
- godoc 补充用例：
  `middleware.JWTAuth[UserClaims](secret, middleware.WithVerifyOptions(gojwt.WithExpectedIssuer("myapp")))`

选择通用透传而非专用快捷方式的理由：覆盖当前 `WithExpectedIssuer` 及未来所有
`go-auth/jwt` 新增的 Verify 相关选项，无需每次新增选项都在中间件层重复封装。

## 测试

新增单元测试：配置 `WithVerifyOptions(gojwt.WithExpectedIssuer(...))` 后，
issuer 不匹配的 token 被中间件以 401 拒绝；未配置时行为不变（回归保护）。
使用真实 `gojwt.Sign` 签发 token 做端到端验证，不 mock。

## 涉及文件

- `go-framework/hertz/middleware/jwt_auth.go`（实现 + godoc）
- `go-framework/hertz/middleware/jwt_auth_test.go`（新增测试用例）
- `example/handler/auth_jwt.go`（可选同步，非强制）

## 状态

设计已在对话中获用户批准（2026-09-02）。
