# Issue #95: 中间件配置化 — CORS/SessionAuth 改为 Options 模式

## Context

`go-framework/hertz/middleware/` 包内多个中间件被下游项目复用。`Cors()` 目前无参数，
`Origin` 固定为 `*` 且同时设置 `Access-Control-Allow-Credentials: true`
（通配符 Origin + 携带凭证的组合在浏览器 CORS 规范下本就是无效/危险配置），
Header/Method/ExposeHeaders 列表也写死，业务方无法按项目/环境定制（如限制 Origin
白名单），只能弃用后完全自实现。

排查同包内其它中间件是否存在同类问题：

| 文件 | 现状 | 结论 |
|------|------|------|
| `auth.go` | 已有 `Option`（`WithTimestampWindow`） | 无需改动 |
| `jwt_auth.go` | 已有 `JWTAuthOption`（`WithRevocationChecker`/`WithVerifyOptions`） | 无需改动 |
| `accesslog.go` | 无业务硬编码参数 | 无需改动 |
| `session_auth.go` | `headerSessionID`/`cookieSessionID` 包级常量硬编码，无法定制 | **本次一并改造** |
| `device_auth.go` | 行为完全由调用方传入的 `extract` 函数决定，无硬编码 | 无需改动 |
| Recovery (`server.go` 用 Hertz 官方 `recovery.Recovery()`) | `IsRecovery` 已是可配置开关；非本仓库自实现中间件；issue 明确不涉及默认值变更 | 不改动 |

## Goal

- `Cors()` 支持 `...CorsOption`，暴露 Origin/Header/Method/ExposeHeaders/Credentials
  的可定制项，默认值与当前硬编码行为完全一致（向后兼容，`Cors()` 零参数调用不变）。
- `SessionAuth()` 支持 `...SessionAuthOption`，暴露 Header/Cookie 名称可定制项，
  默认值仍为 `X-Session-Id`/`session_id`。
- 不改变任何现有默认行为，纯新增可选能力。

## Design

### 命名约定

包内已有两种 Options 命名先例：`auth.go` 用包级 `Option`，`jwt_auth.go` 用
`JWTAuthOption`。因为 `Option` 名已被 `auth.go` 占用，新增中间件遵循 `jwt_auth.go`
先例，使用 `<Middleware>Option` 命名，避免包级标识符冲突。

### `cors.go`

```go
type corsConfig struct {
    allowOrigins     []string // 默认 ["*"]
    allowHeaders     []string // 默认原硬编码列表
    allowMethods     []string // 默认原硬编码列表
    exposeHeaders    []string // 默认原硬编码列表
    allowCredentials bool     // 默认 true（与现状一致）
}

type CorsOption func(*corsConfig)

func WithAllowOrigins(origins []string) CorsOption
func WithAllowHeaders(headers []string) CorsOption
func WithAllowMethods(methods []string) CorsOption
func WithExposeHeaders(headers []string) CorsOption
func WithAllowCredentials(allow bool) CorsOption

func Cors(opts ...CorsOption) app.HandlerFunc
```

行为：

- 默认配置（不传 opts）与当前硬编码输出完全一致。
- `allowOrigins` 为 `["*"]`（默认，未显式设置）时，`Access-Control-Allow-Origin`
  头固定输出 `*`，与现状一致。
- `allowOrigins` 被显式设置为非 `["*"]` 值时，按请求 `Origin` 头做白名单比对：
  命中则回显该 Origin，未命中则不设置该响应头（浏览器端请求会被拒绝）。
- 其余头（Headers/Methods/ExposeHeaders/Credentials）按配置值 `strings.Join(",")` 输出。
- 空 slice / 空字符串输入按 Options 模式规范忽略，不覆盖默认值。

### `session_auth.go`

```go
type sessionAuthConfig struct {
    headerName string // 默认 "X-Session-Id"
    cookieName string // 默认 "session_id"
}

type SessionAuthOption func(*sessionAuthConfig)

func WithSessionHeader(name string) SessionAuthOption
func WithSessionCookie(name string) SessionAuthOption

func SessionAuth(store session.Store, opts ...SessionAuthOption) app.HandlerFunc
```

行为：空字符串输入忽略，不覆盖默认值；`extractSessionID` 改为读取配置后的
header/cookie 名称，优先级不变（Header > Cookie）。

## Testing

- `cors_test.go`：默认行为不变（原有断言保留）；`WithAllowOrigins` 白名单命中/未命中；
  多个 Option 组合；空值输入不覆盖默认值。
- `session_auth_test.go`：默认行为不变；`WithSessionHeader`/`WithSessionCookie` 自定义
  名称下的 Header 优先 Cookie 提取逻辑；空值输入不覆盖默认值。

## Non-Goals

- 不改变 Recovery（`IsRecovery`）默认值。
- 不改造 `auth.go`/`jwt_auth.go`/`accesslog.go`/`device_auth.go`（已有 Options 或无硬编码问题）。
- 不引入跨模块依赖，改动范围限定在 `go-framework/hertz/middleware/` 包内。
