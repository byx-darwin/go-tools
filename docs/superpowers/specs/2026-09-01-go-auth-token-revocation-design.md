# go-auth Token 撤销机制 + 错误码传导设计

- Issue: [#72](https://github.com/byx-darwin/go-tools/issues/72)
- 状态：待实现
- 涉及模块：go-auth（新增）、go-middleware（新增）、go-framework（修改）

## 背景

`go-auth/error/error.go` 定义了 `CodeTokenRevoked`(40003) 和 `CodeDeviceKicked`(40004)，但全仓库除测试外无生产代码路径会产出这两个错误。`go-framework/hertz/middleware` 的 `JWTAuth`/`DeviceAuth` 鉴权失败时统一 `AbortWithStatus(401)`，不区分 token 过期/无效/撤销/设备被踢出。同时 go-auth 及下游都没有提供独立于 device 模型的 Token 撤销（黑名单）能力——唯一能使 token 失效的手段是 `device.Store` 的 `RemoveDevice`/`RemoveAllDevices`，只覆盖"设备登出"场景。

现状调研发现：`go-framework/hertz/response.go` 已有一套完整的 `Responder` + `ErrorRouter` 机制用于把 oops 错误路由为 HTTP 响应（含 Protobuf/JSON 内容协商、i18n），但当前完全未被认证中间件使用。

## 目标

1. 在 go-auth 新增独立的 `revocation` 契约包，不依赖 device 模型，支持任意场景下按 JTI 撤销单个 token。
2. go-middleware 提供 Redis TTL 版实现。
3. go-framework 的 `JWTAuth`/`DeviceAuth` 中间件改为区分具体错误原因，返回精确的 HTTP 状态码 + 业务码，而不是统一 401。

## 非目标（本 issue 不做的事）

- Refresh Token 轮换/复用检测——已拆分到独立 issue [#74](https://github.com/byx-darwin/go-tools/issues/74)。
- "用户主动登出但未指定设备"的业务封装（如 `Logout(ctx, jti)` 便捷函数）——本次只提供 `revocation.Store` 契约和 Redis 实现，具体业务封装留给调用方或后续 issue。
- `session` 包的撤销集成——Session 走独立的 `Delete`/`Exists` 语义，已经具备撤销能力，本次聚焦 JWT/Device。

## 架构

```
go-auth/revocation/          (新增，纯接口包)
  └── checker.go             Checker / Revoker 接口
go-auth/jwt/
  └── token.go                新增 ExtractJTI(claims any) (string, bool) 导出函数（复用现有反射逻辑）
go-auth/error/
  └── error.go                新增 init()，注册 401/403/500 HTTP 状态映射
go-middleware/auth/
  └── revocation_redis.go     (新增) RedisRevocationStore，实现 revocation.Checker + Revoker
go-framework/hertz/middleware/
  ├── jwt_auth.go              新增 WithRevocationChecker Option；错误响应改用 Responder.ErrorWithCode
  └── device_auth.go           错误响应改用 Responder.ErrorWithCode，区分 kicked/internal
```

**数据流**：请求 → `JWTAuth` 验证签名（复用 `jwt.Verify`）→ 若配置了 `WithRevocationChecker`，用 `jwt.ExtractJTI` 取出 jti → `checker.IsRevoked(ctx, jti)` → 命中返回 `ErrTokenRevoked`（401）→ 未命中放行，claims 注入 context → `DeviceAuth`（如启用）调用 `CheckDevice`，失败返回 `ErrDeviceKicked`（403）。

## 详细设计

### 1. `go-auth/revocation` 接口

```go
package revocation

// Checker 检查 JTI 是否已被撤销。
type Checker interface {
    IsRevoked(ctx context.Context, jti string) (bool, error)
}

// Revoker 撤销指定 JTI。
type Revoker interface {
    // Revoke 撤销 jti，ttl 应设置为该 token 的剩余有效期（过期后自动从存储清除）。
    Revoke(ctx context.Context, jti string, ttl time.Duration) error
}

// Store 同时具备撤销与检查能力的完整存储接口。
type Store interface {
    Checker
    Revoker
}
```

拆成 `Checker`/`Revoker` 两个小接口而非一个大接口：`middleware.WithRevocationChecker` 只依赖 `Checker`；未来若要加批量撤销等能力可以再加独立小接口，不破坏现有实现（optional interface pattern，呼应 [#79](https://github.com/byx-darwin/go-tools/issues/79) 的架构建议）。

### 2. HTTP 状态码映射（`go-auth/error` 新增 init）

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

保留现有 401/403 语义，不采用仓库默认的"业务错误一律 200"约定——避免破坏下游客户端现有的"401 触发刷新 token"逻辑。

### 3. 中间件改造

**jwt_auth.go**：

```go
type config struct {
    revocationChecker revocation.Checker
}
type Option func(*config)

// WithRevocationChecker 设置撤销检查器，验证签名成功后额外查询撤销表。
// 未设置时行为与现在完全一致。
func WithRevocationChecker(checker revocation.Checker) Option {
    return func(c *config) { c.revocationChecker = checker }
}

func JWTAuth[T any](secret []byte, opts ...Option) app.HandlerFunc {
    cfg := applyOptions(opts)
    return func(ctx context.Context, c *app.RequestContext) {
        token := extractBearerToken(c)
        if token == "" {
            writeAuthError(ctx, c, autherror.ErrTokenInvalid)
            return
        }
        claims, err := gojwt.Verify[T](token, secret)
        if err != nil {
            writeAuthError(ctx, c, err)
            return
        }
        if cfg.revocationChecker != nil {
            if jti, ok := gojwt.ExtractJTI(claims); ok {
                revoked, rerr := cfg.revocationChecker.IsRevoked(ctx, jti)
                if rerr != nil {
                    writeAuthError(ctx, c, oops.Code(autherror.CodeJWTVerifyFailed).Wrap(rerr))
                    return
                }
                if revoked {
                    writeAuthError(ctx, c, autherror.ErrTokenRevoked)
                    return
                }
            }
        }
        SetClaims(c, claims)
        c.Next(ctx)
    }
}

// writeAuthError 写鉴权错误响应，跳过 ErrorRouter，直接使用 err 对应的 httpCode/bizCode。
// 复用 Responder.ErrorWithCode 以保留内容协商（JSON/Protobuf）与 i18n 能力；
// 若上游未通过 Middleware() 注入 Responder，RespondFrom 退化为 defaultResponder，
// 仍能正确写出 httpCode/bizCode（ErrorWithCode 本身不依赖 ErrorRouter）。
func writeAuthError(ctx context.Context, c *app.RequestContext, err error) {
    code, msg := goerror.Extract(err)
    hertz.RespondFrom(c).ErrorWithCode(ctx, c, goerror.HTTPStatus(err), code, msg)
}
```

`JWTAuth[T any](secret []byte, opts ...Option)` 新增变参 `opts`，属于向后兼容的签名扩展（原调用 `JWTAuth[UserClaims](secret)` 不受影响）。

**device_auth.go**：`CheckDevice` 返回 `false` → `writeAuthError(ctx, c, autherror.ErrDeviceKicked)`；`CheckDevice` 返回 `err != nil` → 包一层 `CodeJWTVerifyFailed`（内部错误，非鉴权语义错误）后 `writeAuthError`。

### 4. go-middleware/auth 的 Redis 实现

并入现有 `go-middleware/auth` 包（与 `device_redis.go`/`session_redis.go` 同包），key 设计延续相同前缀约定：`{prefix}revoked:{jti}`，用 `SET key "" EX ttl` 写入，`EXISTS`/`GET` 判断撤销状态。遵循 Functional Options 模式：

```go
// NewRedisRevocationStore 创建 Redis 撤销存储。
// 默认配置：
//   - keyPrefix: ""
func NewRedisRevocationStore(client redis.UniversalClient, opts ...Option) *RedisRevocationStore
```

复用包内已有的 `Option`/`applyDefaults`（与 `RedisDeviceStore` 共享 `keyPrefix` 配置项）。

## 错误处理

- `Verify` 失败（签名无效/过期）→ 复用 `jwt.Verify` 已映射好的 `ErrTokenInvalid`/`ErrTokenExpired`。
- 撤销表查询失败（Redis 故障等基础设施错误）→ 包装为 `CodeJWTVerifyFailed`（500），不误判为"token 已撤销"。
- `CheckDevice` 查询失败 → 同样包装为 `CodeJWTVerifyFailed`（500）。
- 所有错误响应通过 `writeAuthError` 统一出口，保证 httpCode/bizCode/msg 的映射规则集中在一处。

## 测试计划

- `go-auth/jwt`：`ExtractJTI` 单测（直接嵌入 RegisteredClaims 场景、无 JTI 字段场景、nil claims 场景）。
- `go-auth/error`：HTTP 状态映射注册测试（覆盖 init 不重复 panic，验证映射结果正确）。
- `go-middleware/auth`：`RedisRevocationStore` 单测（miniredis）：撤销后 `IsRevoked` 返回 true、TTL 过期后自动清除、未撤销返回 false、并发撤销安全。
- `go-framework/hertz/middleware`：
  - `JWTAuth` 集成测试：未配置 `WithRevocationChecker` 时行为与现在完全一致（回归测试）；配置后撤销 token 返回 401 且响应体含 `code=40003`。
  - `DeviceAuth` 集成测试：设备被踢出返回 403 + `code=40004`；内部错误返回 500。

## 依赖影响

- `go-auth` 新增 `revocation` 包，不引入新的第三方依赖，不反向依赖 go-framework/go-middleware（符合模块边界约束）。
- `go-framework/hertz/middleware` 依赖 `go-auth/revocation`（已在依赖范围内）与 `go-framework/hertz`（同模块内部依赖，已存在）。
- `go-middleware/auth` 新文件复用现有 `redis.UniversalClient`/`samber/oops` 依赖，无新增。
