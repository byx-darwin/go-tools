# Hertz Response 重构设计文档

**日期**: 2026-06-25
**状态**: 设计中
**作者**: Claude + User
**模块**: `go-framework/hertz`

---

## 1. 背景与目标

### 1.1 现状问题

当前 `go-framework/hertz/response.go` 功能简单：
- 始终输出 JSON，无内容协商
- 无 Protobuf 支持
- 无 i18n 国际化
- 无 Debug 模式（生产/开发环境错误信息无区分）
- 无 RPC 错误类型路由
- 无 Request ID 透传

项目方（如 go-beniofit）不得不在业务代码中自行实现这些能力（参见 `bff/beniofit/internal/pkg/utils/reply.go`），导致重复代码和 inconsistent 行为。

### 1.2 设计目标

将 reply.go 中的项目特定实现**泛化**为库级别 API，提供：

1. **内容协商** — 根据 Accept 头自动选择 JSON / Protobuf
2. **Debug 模式** — 开发环境返回详细错误，生产环境隐藏内部细节
3. **RPC 错误路由** — 基于 `kitex/rpcerror` 的错误类型自动映射到 HTTP 状态码
4. **i18n 国际化** — 可插拔 Translator 接口
5. **Request ID** — 多级 fallback（OTel → Header → UUID），回写响应头 + 日志 context
6. **全新 API** — 废弃旧 `OK`/`Err`/`Result`，采用 Responder 对象 + 中间件模式

### 1.3 非目标

- 不绑定具体 Protobuf 类型（如 `base.BaseReply`）—— 由项目方定义
- 不内置具体 i18n 实现 —— 项目方实现 `Translator` 接口
- 不替代 OTel 追踪中间件 —— 与之协同，不重复

---

## 2. 架构设计

### 2.1 方案选择：Responder 对象 + 中间件注入

采用**方案 A**，理由：
- 完全可测试（每个测试用例独立 Responder）
- 不同路由组可有不同配置
- 符合 Hertz 中间件惯用法
- 支持 per-request 状态（如从 header 提取 lang）

### 2.2 组件关系

```
┌─────────────────────────────────────────────────────────┐
│                  Responder.Middleware()                   │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────┐  │
│  │ RequestID   │  │ Logger       │  │ Responder      │  │
│  │ Extractor   │  │ Injector     │  │ Context Inject │  │
│  └─────────────┘  └──────────────┘  └────────────────┘  │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
              ┌────────────────────────┐
              │  ctx with:             │
              │  - request_id          │
              │  - logger (with reqID) │
              │  - *Responder          │
              └────────────────────────┘
                           │
                           ▼
              ┌────────────────────────┐
              │  Handler:              │
              │  resp := RespondFrom(c)│
              │  resp.Success(data)    │
              │  resp.Error(err, msg)  │
              └────────────────────────┘
                           │
                           ▼
              ┌────────────────────────┐
              │  Response Pipeline:    │
              │  1. Error Routing      │
              │  2. I18n Translation   │
              │  3. Debug Filter       │
              │  4. Content Negotiation│
              │  5. Write Response     │
              └────────────────────────┘
```

### 2.3 文件结构

```
go-framework/hertz/
├── response.go            ← 全新重写（核心类型 + Responder）
├── response_test.go       ← 全新测试
├── server.go              ← 不变
├── middleware/            ← 不变
└── observability/         ← 不变
```

---

## 3. 核心类型设计

### 3.1 Response 结构体

统一响应体，同时支持 JSON 和 Protobuf 序列化：

```go
// Response 统一响应体。
// 支持 JSON 和 Protobuf 双格式序列化。
type Response struct {
    Code int    `json:"code"       protobuf:"varint,1,opt,name=code"`
    Msg  string `json:"msg"        protobuf:"bytes,2,opt,name=msg"`
    Data any    `json:"data,omitempty" protobuf:"bytes,3,opt,name=data"`
}
```

**设计说明**:
- `Code`: 业务码，200 = 成功
- `Msg`: 用户可见消息（经 i18n 翻译后）
- `Data`: 业务数据，可选
- Request ID 通过响应头 `X-Request-ID` 返回，不放响应体

**注意**: `Data` 字段类型为 `any`，Protobuf 序列化时需要项目方自行定义具体 message 类型并转换。库提供 `Response` 作为 JSON 默认结构，Protobuf 场景下项目方可定义自己的结构体传入 `Reply()`。

### 3.2 Translator 接口

i18n 抽象，项目方实现：

```go
// Translator 国际化翻译器接口。
// 项目方实现此接口接入自己的 i18n 系统。
type Translator interface {
    // Translate 翻译消息 key 为指定语言文本。
    // lang 从请求上下文提取（见 WithLangHeader）。
    // key 为翻译 key 或原始消息。
    Translate(ctx context.Context, lang, key string) string
}
```

**设计说明**:
- 接口极简，单一方法
- `ctx` 参数允许 Translator 从 context 获取额外信息（如用户偏好）
- `lang` 由 Responder 从请求 header 提取并传入
- 若未配置 Translator，消息原样返回

### 3.3 ErrorRouter 接口

RPC 错误路由抽象：

```go
// ErrorRoute 错误路由结果。
type ErrorRoute struct {
    HTTPCode int    // HTTP 状态码（如 200, 400, 500）
    BizCode  int    // 业务码（响应体中的 code 字段）
    Override string // 覆盖消息（非空时替代 publicMsg）
}

// ErrorRouter RPC 错误路由器接口。
// 将 RPC 错误映射为 HTTP 响应参数。
type ErrorRouter interface {
    // Route 分析错误，返回路由结果。
    // 返回 ok=false 表示不识别此错误，走默认路由。
    Route(ctx context.Context, err error) (ErrorRoute, bool)
}
```

**设计说明**:
- `ErrorRouter` 不直接依赖 `rpcerror` 包 —— 项目方可实现任意路由逻辑
- go-framework 提供一个默认的 `RPCErrorRouter` 实现（基于 `kitex/rpcerror`）
- `Override` 字段允许路由器覆盖用户消息（如错误类型对应固定提示文案）

### 3.4 默认 RPCErrorRouter

基于 `go-common/error` 的默认实现，直接复用现有错误码体系：

```go
// RPCErrorRouter 基于 go-common/error 的默认路由器。
// 从 oops 错误中提取错误码（10001, 10002 等），
// 使用 goerror.HTTPStatus() 映射 HTTP 状态码。
//
// 错误码对齐 go-common/error 定义：
//   - 10001 CodeParamInvalid → HTTP 400
//   - 10002 CodeAuthFailed   → HTTP 401
//   - 10010 CodeRPCUnavailable → HTTP 503
//   - 10011 CodeRPCTimeout   → HTTP 504
//   - 10000 CodeSystem       → HTTP 500
//   - 其他 → 提取错误码，HTTP 500（兜底）
type RPCErrorRouter struct{}
```

**工作原理**：

```go
func (r *RPCErrorRouter) Route(ctx context.Context, err error) (ErrorRoute, bool) {
    code, public := goerror.Extract(err)
    if code == 0 {
        return ErrorRoute{}, false // 非 oops 错误，不识别
    }
    httpCode := goerror.HTTPStatus(err)
    return ErrorRoute{
        HTTPCode: httpCode,
        BizCode:  code,    // 直接使用错误码（10001, 10002 等）
        Override: public,  // 使用 oops 的 public 消息
    }, true
}
```

**错误码与 HTTP 状态码映射**（来自 `go-common/error`）：

| 错误码 | 常量名 | HTTP Code | 场景 |
|--------|--------|-----------|------|
| 10000 | `CodeSystem` | 500 | 系统内部错误（兜底） |
| 10001 | `CodeParamInvalid` | 400 | 参数无效 |
| 10002 | `CodeAuthFailed` | 401 | 鉴权失败 |
| 10003 | `CodeConfigNotFound` | 500 | 配置未找到 |
| 10004 | `CodeConfigInvalid` | 500 | 配置无效 |
| 10010 | `CodeRPCUnavailable` | 503 | RPC 服务不可用 |
| 10011 | `CodeRPCTimeout` | 504 | RPC 超时 |
| 10012 | `CodeRPCDecodeError` | 500 | RPC 解码错误 |
| 10013 | `CodeRPCEncodeError` | 500 | RPC 编码错误 |
| 20001-20005 | Redis 错误 | 500/503 | Redis 连接/操作 |
| 20101-20105 | Kafka 错误 | 500/503 | Kafka 连接/发送 |
| 20201-20204 | DB 错误 | 500/503 | 数据库连接/查询 |
| 40010-40012 | 数据错误 | 200 | 数据不存在/重复/冲突 |
| 40110-40113 | 认证错误 | 200 | 登录失败/凭证问题 |
| 40210-40212 | 限制错误 | 200 | 频率限制/配额用尽 |
| 40310-40314 | 业务状态 | 200 | 账户禁用/余额不足 |

**注意**：业务错误码（40000-59999）通常返回 HTTP 200，因为 RPC 调用成功，只是业务逻辑失败。

---

## 4. Responder 设计

### 4.1 结构体

```go
// Responder 统一响应处理器。
// 持有配置，提供 Success/Error/Reply 方法。
// 通过 Middleware 注入请求上下文。
type Responder struct {
    debug        bool              // Debug 模式
    translator   Translator        // i18n 翻译器（可选）
    errorRouter  ErrorRouter       // 错误路由器（可选）
    reqIDHeader  string            // Request ID 响应头名
    reqIDGen     func() string     // Request ID 生成器
    langHeader   string            // 语言检测请求头名
    defaultCodes map[string]int    // 默认业务码配置
}
```

### 4.2 Functional Options

```go
// Option 定义 Responder 配置选项。
type Option func(*Responder)

// WithDebug 启用 Debug 模式。
// Debug 模式下错误响应包含内部错误详情，生产环境应关闭。
func WithDebug(debug bool) Option

// WithTranslator 设置 i18n 翻译器。
func WithTranslator(t Translator) Option

// WithErrorRouter 设置错误路由器。
// 传入 nil 清除路由器。
func WithErrorRouter(r ErrorRouter) Option

// WithRequestIDHeader 设置 Request ID 响应头名称。
// 默认 "X-Request-ID"。空字符串禁用 Request ID 功能。
func WithRequestIDHeader(name string) Option

// WithRequestIDGenerator 设置 Request ID 生成函数。
// 默认使用 UUID v4。
func WithRequestIDGenerator(fn func() string) Option

// WithLangHeader 设置语言检测请求头名称。
// 默认 "Accept-Language"。空字符串禁用 i18n 语言检测。
func WithLangHeader(name string) Option

// WithDefaultBizCode 设置默认成功/失败业务码。
// 默认 successCode=200, failCode=500。
func WithDefaultBizCode(successCode, failCode int) Option
```

### 4.3 构造函数

```go
// NewResponder 创建 Responder 实例。
// 默认配置：
//   - debug: false
//   - reqIDHeader: "X-Request-ID"
//   - langHeader: "Accept-Language"
//   - successCode: 200
//   - failCode: 500
func NewResponder(opts ...Option) *Responder
```

---

## 5. API 设计

### 5.1 Responder 方法

```go
// Success 成功响应。
// HTTP 200，业务码为 successCode（默认 200），msg 为 "ok"。
// data 为 nil 时响应体不含 data 字段。
func (r *Responder) Success(ctx *app.RequestContext, data any)

// SuccessWithMsg 带自定义消息的成功响应。
// msg 会经过 Translator 翻译（若已配置）。
func (r *Responder) SuccessWithMsg(ctx *app.RequestContext, data any, msg string)

// Error 错误响应。
// 根据错误类型自动路由：
//   1. 若配置了 ErrorRouter 且识别错误 → 使用路由结果
//   2. 否则 → HTTP 500 + failCode
// publicMsg 为用户可见消息（经 Translator 翻译）。
// Debug 模式下，err.Error() 附加到 msg 末尾。
func (r *Responder) Error(ctx *app.RequestContext, err error, publicMsg string)

// ErrorWithCode 指定业务码的错误响应。
// 跳过 ErrorRouter，直接使用指定 code。
func (r *Responder) ErrorWithCode(ctx *app.RequestContext, httpCode, bizCode int, msg string)

// Reply 原始响应写入。
// 直接写入指定 HTTP 状态码和对象，用于自定义响应结构。
// 支持内容协商（JSON/Protobuf）。
func (r *Responder) Reply(ctx *app.RequestContext, httpCode int, obj any)
```

### 5.2 包级便捷函数

内部使用默认 Responder（无 Debug、无 Translator、无 ErrorRouter）：

```go
// Success 成功响应（使用默认 Responder）。
func Success(ctx *app.RequestContext, data any)

// Error 错误响应（使用默认 Responder）。
func Error(ctx *app.RequestContext, err error, publicMsg string)

// Reply 原始响应写入（使用默认 Responder）。
func Reply(ctx *app.RequestContext, httpCode int, obj any)
```

### 5.3 Context 辅助函数

```go
// RespondFrom 从请求上下文获取 Responder。
// 若未通过 Middleware 注入，返回默认 Responder。
func RespondFrom(ctx *app.RequestContext) *Responder

// RequestIDFrom 从请求上下文获取当前 Request ID。
// 若未通过 Middleware 提取，返回空字符串。
func RequestIDFrom(ctx *app.RequestContext) string
```

---

## 6. 中间件设计

### 6.1 Middleware 方法

```go
// Middleware 返回 Hertz 中间件处理函数。
// 中间件职责：
//   1. 提取/生成 Request ID → 设置响应头 + 注入 ctx
//   2. 提取语言偏好 → 注入 ctx
//   3. 注入增强 Logger（含 request_id）→ 替换 ctx 中的 logger
//   4. 注入 Responder 实例 → 注入 ctx
func (r *Responder) Middleware() app.HandlerFunc
```

### 6.2 中间件流程

```
请求进入
  │
  ├─ 1. 提取 Request ID
  │     a. 尝试 OTel trace-id（hex 格式）
  │     b. 尝试 X-Request-ID 请求头
  │     c. 生成 UUID v4
  │     → 设置响应头 X-Request-ID
  │     → 存入 ctx（key: "request_id"）
  │
  ├─ 2. 提取语言偏好
  │     → 从 Accept-Language（或配置的 header）提取
  │     → 存入 ctx（key: "lang"）
  │
  ├─ 3. 增强 Logger
  │     → 创建子 logger，添加 "request_id" 字段
  │     → 替换 ctx 中的 hlog.CtxLogger
  │
  ├─ 4. 注入 Responder
  │     → 将 *Responder 存入 ctx
  │
  └─ 5. c.Next(ctx)
```

### 6.3 Context Key 定义

```go
// context key 类型（不导出，防止冲突）
type ctxKey int

const (
    ctxKeyRequestID ctxKey = iota
    ctxKeyLang
    ctxKeyResponder
)
```

---

## 7. Request ID 详细设计

### 7.1 提取链

```go
func (r *Responder) extractRequestID(ctx *app.RequestContext) string {
    // 1. OTel trace-id
    if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
        return span.SpanContext().TraceID().String()
    }
    // 2. X-Request-ID header
    if id := string(ctx.Request.Header.Peek(r.reqIDHeader)); id != "" {
        return id
    }
    // 3. 生成 UUID v4
    if r.reqIDGen != nil {
        return r.reqIDGen()
    }
    return uuid.NewString()
}
```

### 7.2 OTel 依赖处理

`go-framework` 已有 `hertz/observability` 包依赖 OTel。`response.go` 中获取 trace-id 使用 `trace.SpanFromContext(ctx)`，这是 OTel 标准 API，不会引入额外重依赖。

若项目未使用 OTel，`SpanFromContext` 返回 no-op span，`IsValid()` 为 false，自动跳过。

### 7.3 UUID 生成

默认使用 `github.com/google/uuid`。若项目不想引入此依赖，可通过 `WithRequestIDGenerator` 替换：

```go
responder := hertz.NewResponder(
    hertz.WithRequestIDGenerator(func() string {
        return ksuid.New().String() // 或其他 ID 生成库
    }),
)
```

---

## 8. 内容协商详细设计

### 8.1 Accept 头解析

```go
func negotiateContentType(ctx *app.RequestContext) string {
    accept := strings.ToLower(string(ctx.Request.Header.Peek(consts.HeaderAccept)))
    switch {
    case strings.Contains(accept, consts.MIMEPROTOBUF):
        return consts.MIMEPROTOBUF
    default:
        return consts.MIMEJSONUTF8
    }
}
```

支持的值：
- `application/json` → JSON
- `application/json; charset=utf-8` → JSON
- `application/protobuf` → Protobuf
- `application/x-protobuf` → Protobuf
- 其他/缺失 → JSON（默认）

### 8.2 序列化

```go
func (r *Responder) writeResponse(ctx *app.RequestContext, httpCode int, obj any) {
    switch negotiateContentType(ctx) {
    case consts.MIMEPROTOBUF:
        ctx.ProtoBuf(httpCode, obj)
    default:
        ctx.JSON(httpCode, obj)
    }
}
```

**注意**: Protobuf 序列化要求 `obj` 实现 `proto.Message` 接口。若传入非 Protobuf 对象（如 `Response` 结构体），会回退到 JSON 并记录警告日志。

---

## 9. 错误路由详细设计

### 9.1 路由流程

```go
func (r *Responder) routeError(ctx *app.RequestContext, err error, publicMsg string) (httpCode, bizCode int, finalMsg string) {
    // 1. 尝试自定义 ErrorRouter
    if r.errorRouter != nil {
        if route, ok := r.errorRouter.Route(ctx, err); ok {
            msg := publicMsg
            if route.Override != "" {
                msg = route.Override
            }
            return route.HTTPCode, route.BizCode, r.translate(ctx, msg)
        }
    }
    // 2. 尝试默认 RPCErrorRouter（若 rpcerror 包可用）
    if route, ok := defaultRPCRoute(ctx, err); ok {
        return route.HTTPCode, route.BizCode, r.translate(ctx, publicMsg)
    }
    // 3. 兜底
    return http.StatusInternalServerError, r.failCode, r.translate(ctx, publicMsg)
}
```

### 9.2 Debug 模式处理

```go
func (r *Responder) applyDebugFilter(httpCode int, msg string, err error) string {
    if !r.debug {
        return msg // 生产模式：只返回翻译后的公开消息
    }
    // Debug 模式：附加内部错误详情
    if err != nil {
        return fmt.Sprintf("%s | internal: %s", msg, err.Error())
    }
    return msg
}
```

**安全警告**: Debug 模式绝不能在正式生产环境启用。`err.Error()` 可能包含敏感信息（SQL 语句、堆栈路径等）。

---

## 10. I18n 集成详细设计

### 10.1 语言提取

```go
func (r *Responder) extractLang(ctx *app.RequestContext) string {
    // 1. 从 ctx 获取（中间件已提取）
    if lang, ok := ctx.Value(ctxKeyLang).(string); ok && lang != "" {
        return lang
    }
    // 2. 直接从 header 读取
    if lang := string(ctx.Request.Header.Peek(r.langHeader)); lang != "" {
        return parseLang(lang) // 解析 "zh-CN,zh;q=0.9" → "zh"
    }
    return "zh" // 默认语言
}
```

### 10.2 翻译流程

```go
func (r *Responder) translate(ctx *app.RequestContext, msg string) string {
    if r.translator == nil || msg == "" {
        return msg
    }
    lang := r.extractLang(ctx)
    return r.translator.Translate(ctx, lang, msg)
}
```

### 10.3 项目方集成示例

```go
// 项目方实现 Translator
type MyI18n struct {
    // ... 项目方 i18n 系统
}

func (m *MyI18n) Translate(ctx context.Context, lang, key string) string {
    return m.i18n.Msg(lang, key)
}

// 初始化
responder := hertz.NewResponder(
    hertz.WithTranslator(&MyI18n{...}),
)
r.Use(responder.Middleware())
```

---

## 11. 完整使用示例

### 11.1 项目初始化

```go
package main

import (
    "github.com/bytedance/hertz/pkg/app/server"
    gohertz "github.com/byx-darwin/go-tools/go-framework/hertz"
)

func main() {
    // 创建 Responder
    responder := gohertz.NewResponder(
        gohertz.WithDebug(false),
        gohertz.WithTranslator(myI18nTranslator),
        gohertz.WithErrorRouter(&gohertz.RPCErrorRouter{}), // 使用 go-common/error 错误码体系
        gohertz.WithRequestIDHeader("X-Request-ID"),
        gohertz.WithLangHeader("Accept-Language"),
    )

    // 创建 Hertz 引擎
    h := server.Default()

    // 注册中间件
    h.Use(responder.Middleware())

    // 注册路由
    registerRoutes(h)

    h.Spin()
}
```

### 11.2 Handler 使用

```go
func HandleGetUser(ctx *app.RequestContext) {
    resp := gohertz.RespondFrom(ctx)

    // 调用 RPC 服务
    user, err := rpcClient.GetUser(ctx, &req)
    if err != nil {
        // 自动路由：ErrorTypeParamInvalid → 400, ErrorTypeSystemInternal → 500
        resp.Error(ctx, err, "获取用户失败")
        return
    }

    // 成功响应
    resp.Success(ctx, user)
}

func HandleCreateOrder(ctx *app.RequestContext) {
    resp := gohertz.RespondFrom(ctx)

    // 调用 RPC 服务
    _, err := rpcClient.CreateOrder(ctx, &req)
    if err != nil {
        // 指定业务码，跳过 ErrorRouter
        resp.ErrorWithCode(ctx, 200, 20001, "库存不足")
        return
    }

    resp.SuccessWithMsg(ctx, nil, "下单成功")
}
```

### 11.3 响应输出示例

**成功 (JSON)**:
```json
HTTP/1.1 200 OK
X-Request-ID: 4f3a2b1c-8d9e-4f7a-bc1d-2e3f4a5b6c7d
Content-Type: application/json; charset=utf-8

{
    "code": 200,
    "msg": "ok",
    "data": { "id": 123, "name": "Alice" }
}
```

**参数错误 (Debug 关闭)**:
```json
HTTP/1.1 400 Bad Request
X-Request-ID: 4f3a2b1c-8d9e-4f7a-bc1d-2e3f4a5b6c7d
Content-Type: application/json; charset=utf-8

{
    "code": 10001,
    "msg": "参数无效"
}
```

**参数错误 (Debug 开启)**:
```json
HTTP/1.1 400 Bad Request
X-Request-ID: 4f3a2b1c-8d9e-4f7a-bc1d-2e3f4a5b6c7d
Content-Type: application/json; charset=utf-8

{
    "code": 10001,
    "msg": "参数无效 | internal: field 'email' format invalid"
}
```

---

## 12. 向后兼容

### 12.1 废弃策略

旧 API 标记为 `Deprecated`，保留一个版本周期：

```go
// Deprecated: 使用 NewResponder + Responder.Success 替代。
func OK(c *app.RequestContext, data any) { ... }

// Deprecated: 使用 NewResponder + Responder.Error 替代。
func Err(c *app.RequestContext, err error) { ... }

// Deprecated: 使用 NewResponder + Responder.ErrorWithCode 替代。
func ErrWithCode(c *app.RequestContext, httpCode, bizCode int, msg string) { ... }

// Deprecated: 使用 NewResponder + Responder.Reply 替代。
func Result(c *app.RequestContext, httpCode, code int, data any, msg string) { ... }
```

### 12.2 迁移指南

```go
// 旧
hertz.OK(ctx, data)
// 新
gohertz.RespondFrom(ctx).Success(ctx, data)

// 旧
hertz.Err(ctx, err)
// 新
gohertz.RespondFrom(ctx).Error(ctx, err, "操作失败")

// 旧
hertz.ErrWithCode(ctx, 400, 10001, "参数无效")
// 新
gohertz.RespondFrom(ctx).ErrorWithCode(ctx, 400, 10001, "参数无效")
```

---

## 13. 测试策略

### 13.1 单元测试

| 测试目标 | 验证内容 |
|---------|---------|
| `Response` 序列化 | JSON/Protobuf 输出格式正确 |
| `Responder.Success` | HTTP 200 + 正确业务码 + data |
| `Responder.Error` | 错误路由 + Debug 过滤 + i18n 翻译 |
| `Responder.Reply` | 内容协商（Accept 头） |
| `RPCErrorRouter` | 各错误类型映射正确 |
| Request ID 提取 | OTel → Header → UUID fallback 链 |
| Translator 集成 | 语言提取 + 翻译调用 |
| Middleware | ctx 注入（request_id, lang, responder） |

### 13.2 集成测试

| 场景 | 验证内容 |
|------|---------|
| 完整请求流程 | 中间件注入 → handler 调用 → 响应输出 |
| Debug 模式切换 | 相同错误在 debug/release 下输出不同 |
| 无 Translator | 消息原样返回，不 panic |
| 无 ErrorRouter | 走默认路由，不 panic |
| Protobuf 请求 | Accept: application/protobuf → ProtoBuf 输出 |

### 13.3 基准测试

关键路径性能测试：
- `Responder.Success` vs 旧 `OK` 函数开销对比
- Request ID 提取（有/无 OTel）
- 内容协商解析

---

## 14. 依赖分析

### 14.1 新增依赖

| 依赖 | 用途 | 备注 |
|------|------|------|
| `github.com/google/uuid` | Request ID 生成 | 可通过 `WithRequestIDGenerator` 替换 |
| `go.opentelemetry.io/otel/trace` | 提取 trace-id | 已通过 `hertz/observability` 引入 |

### 14.2 已有依赖（不变）

| 依赖 | 用途 |
|------|------|
| `github.com/cloudwego/hertz` | HTTP 框架 |
| `github.com/byx-darwin/go-tools/go-framework/kitex/rpcerror` | RPC 错误类型 |

---

## 15. 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| Protobuf 序列化 `any` 类型失败 | 运行时 panic | 检测非 proto.Message 时回退 JSON + 警告日志 |
| OTel 未初始化时 trace-id 提取 | 返回无效 span | `IsValid()` 检查，自动跳过 |
| Debug 模式误开至生产 | 泄露内部错误 | godoc 强烈警告 + 建议通过环境变量控制 |
| 旧 API 废弃后项目方未迁移 | 编译告警 | 保留一个版本周期，Deprecated 注释明确指向新 API |
| UUID 依赖增加包体积 | 微小影响 | 提供 `WithRequestIDGenerator` 允许零依赖替换 |

---

## 附录 A: 与 reply.go 的能力映射

| reply.go 能力 | 新 API 对应 |
|--------------|------------|
| `Reply(ctx, code, obj)` 内容协商 | `Responder.Reply(ctx, code, obj)` |
| `ReplyWithBindErr(ctx, ctx, log, isDebug, err)` | `Responder.Error(ctx, err, "bind error")` + `WithDebug` |
| `ReplyWithErr(ctx, ctx, format, msg, log, isDebug, err)` | `Responder.Error(ctx, err, publicMsg)` + ErrorRouter + Translator |
| `getLang(ctx)` metainfo 提取 | `Responder.extractLang(ctx)` + `WithLangHeader` |
| `i18n.Msg(lang, msg)` | `Translator.Translate(ctx, lang, msg)` |
| `rpc_error.ParseBizStatusError(err)` | `ErrorRouter.Route(ctx, err)` + 默认 `RPCErrorRouter` |
| `base.BaseReply{Code, Msg}` | `Response{Code, Msg, Data}` + 响应头 `X-Request-ID` |
