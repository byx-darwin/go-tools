# SSE（Server-Sent Events）支持设计（Issue #97）

## 背景

`go-framework` 的 hertz/kitex 两侧均未提供 SSE 流式响应能力封装，业务方如需实现服务端推送（如 AI 流式回复、实时通知）需自行处理，缺少框架层统一封装。

来源：用户需求跟进，2026-09-02（Issue #97）。

## 目标

1. **Hertz 侧**：在 `go-framework/hertz/sse` 新增 `Writer` 类型，封装 Hertz 官方 `pkg/protocol/sse` 能力，集成 Request ID 复用、panic recovery、心跳保活、断连检测，对齐现有 `Responder` 中间件规范
2. **Kitex 侧**：评估现有 `middleware/auth/*`、`middleware/accesslog`、`observability/*` 在 streaming RPC 场景下的兼容性，产出评估结论与示例文档
3. 补充使用示例和文档

## 非目标

- 不在 Kitex 侧产出新的导出 API（streaming 是 IDL 生成代码驱动的能力，不是可通用挂载的中间件层）；若评估发现真实兼容性缺陷，作为独立后续任务处理，不在本次范围内修复
- 不修改 Hertz 侧现有 `Responder`/中间件行为，仅新增独立子包
- 不实现 SSE 协议编解码本身（Hertz 已提供官方 `sse` 包，直接复用）
- 不支持客户端重连 `Last-Event-ID` 恢复语义（Hertz 原生 `sse.Reader` 已提供解析能力，但业务侧重放逻辑超出框架层职责）

## 现状调研摘要

- **Hertz 官方 SSE 包**（`github.com/cloudwego/hertz@v0.10.5/pkg/protocol/sse`）：`NewWriter(c *app.RequestContext) *Writer` 自动设置 `Content-Type: text/event-stream; charset=utf-8`、`Cache-Control: no-cache`，基于 chunked body writer；`Writer.WriteEvent(id, eventType, data)` / `Write(*Event)` / `WriteKeepAlive()` / `WriteComment(s)` / `Close()` 线程安全（内部 `sync.Mutex`）。已完整覆盖 SSE 协议编解码，框架层无需重复实现
- **`Responder`**（`go-framework/hertz/response.go`）：`Middleware()` 提取 Request ID（OTel trace-id → `X-Request-ID` header → UUID 生成），存入 ctx，可用 `hertz.RequestIDFrom(ctx)` 读取；同时提供 panic recover → `Error()` 写 JSON/Protobuf 错误响应的模式，可作为 SSE panic recovery 的参考范式（但响应格式必须改为 SSE 事件，因为 headers 已提交为 `text/event-stream`，无法回退到 JSON 响应）
- **Kitex streaming**（`github.com/cloudwego/kitex@v0.16.2`）：`pkg/streaming`、`client/streamclient`、`pkg/remote/trans/ttstream` 等提供流式 RPC 能力，但 stub 由 IDL（Thrift/Protobuf streaming 声明）生成，服务定义层面决定是否为流式方法，框架层无法像 Hertz 中间件那样通用封装
- **Kitex 现有中间件**：`middleware/auth/{jwt,session,device,recovery}.go` 基于 `endpoint.Middleware`（`ctx, req, resp` 单次调用模型）；`middleware/accesslog.go`、`observability/suite.go` 同理。streaming 场景下 `endpoint.Middleware` 仍然包裹整个流的建立过程（非逐帧），需要确认这些中间件是否有"假设请求在中间件返回前已完整完成"的隐含预期（如 accesslog 记录耗时的时间点）

## 架构设计

### 包结构

```text
go-framework/hertz/sse/
  writer.go   # Writer 类型、NewWriter、WriteEvent、Run、Close
  options.go  # Option 类型、WithHeartbeatInterval、WithRecoverHandler
  error.go    # SSE 错误事件格式（对齐 Response 三段式）
  doc.go      # 包级说明 + 前置条件文档（Responder.Middleware 顺序依赖）
  *_test.go

go-framework/kitex/
  streaming_compat.go 或 doc.go  # streaming 兼容性评估结论（文档形式，非新 API）
```

`hertz/sse` 与 `hertz/middleware`、`hertz/observability` 并列，作为独立子包，不侵入 `Responder` 现有代码路径。

### 组件设计

**`writer.go`**

```go
// Writer 封装 Hertz 原生 SSE Writer，集成 Request ID、panic recovery、
// 心跳保活、断连检测，对齐 Responder 规范。
type Writer struct {
    w          *sse.Writer // hertz 原生 writer
    heartbeat  time.Duration
    onRecover  func(rec any)
    cancel     context.CancelFunc
    done       chan struct{}
}

// NewWriter 创建 SSE Writer。
//
// 前置条件：ctx 所在请求链路必须已经过 hertz.Responder.Middleware()，
// 否则 Request ID 特性静默失效（hertz.RequestIDFrom(ctx) 返回空字符串）。
//
// 默认配置：
//   - heartbeatInterval: 15 * time.Second
//   - onRecover: 记录日志，不重新 panic
func NewWriter(c context.Context, rc *app.RequestContext, opts ...Option) *Writer

// WriteEvent 写入一条 SSE 事件（透传 hertz sse.Writer.WriteEvent）。
// 断连或写入失败时返回 error，调用方应据此退出事件循环。
func (w *Writer) WriteEvent(id, eventType string, data []byte) error

// Run 包装业务事件循环：内部启动心跳 + 断连检测 goroutine，函数返回前
// 自动 Close。handler 内 panic 会被捕获，写入 event:error 后关闭连接
// （不向上重新抛出）。
func (w *Writer) Run(handler func(w *Writer) error) error

// Close 关闭连接，停止心跳 goroutine。
func (w *Writer) Close() error
```

**`options.go`**

```go
// Option 定义 Writer 配置选项。
type Option func(*config)

// WithHeartbeatInterval 设置心跳保活间隔。<=0 禁用心跳。默认 15s。
func WithHeartbeatInterval(d time.Duration) Option

// WithRecoverHandler 设置自定义 panic 上报回调（如埋点/告警），
// 在写入 error 事件之前调用。默认仅记录结构化日志。
func WithRecoverHandler(fn func(rec any)) Option
```

**`error.go`**

```go
// sseErrorPayload SSE 错误事件负载，对齐 Response 三段式。
type sseErrorPayload struct {
    Code int    `json:"code"`
    Msg  string `json:"msg"`
    Data any    `json:"data,omitempty"`
}

// writeErrorEvent 写入 event:error，data 为 JSON 序列化的三段式结构。
func writeErrorEvent(w *sse.Writer, code int, msg string) error
```

### 数据流

1. 业务 handler 调用 `sse.NewWriter(ctx, rc, opts...)` 创建 Writer（此时已写入 SSE 响应头）
2. `Run(handler)`：
   a. 启动内部 goroutine：`select { case <-ticker.C: WriteKeepAlive(); case <-ctx.Done(): Close() }`
   b. 调用 `handler(w)`，业务在其中通过 `w.WriteEvent(...)` 推送数据
   c. `handler` 返回或 panic 后，停止心跳 goroutine，`Close()` 释放连接
3. 写入失败（客户端已断开）：`WriteEvent` 返回 error，业务据此退出循环；`Run` 不重试
4. panic 被 `Run` 捕获 → 调用 `onRecover`（若配置）→ `writeErrorEvent(w, 500, "internal server error")` → `Close()` → 结构化日志记录，不重新 panic
5. Request ID：`hertz.RequestIDFrom(ctx)` 读取，仅用于日志关联（如 panic 日志、心跳异常日志），不作为 SSE 事件字段写入响应流（避免污染业务事件流）

### 错误处理

- SSE 建立后无法再切回 JSON/Protobuf 响应（headers 已提交），因此错误一律走 `event: error` + JSON payload（`{"code":..,"msg":".."}`），不复用 `Responder.Error()` 的内容协商逻辑
- `WriteEvent`/`WriteKeepAlive` 失败（通常意味着客户端断开）直接返回给调用方，不在 `Writer` 内部吞掉，由业务决定是否记录日志
- panic recovery 与 `Responder.Middleware()` 的整体设计理念一致（不让 panic 导致进程崩溃），但响应写法必须适配流式场景

## Kitex 侧：兼容性评估范围（非新增 API）

- 逐一检查 `middleware/auth/{jwt,session,device,recovery}.go`、`middleware/accesslog.go`、`observability/suite.go` 在 streaming（`pkg/streaming`）场景下的行为：
  - 是否假设请求在中间件返回前已完整完成（如 accesslog 记录耗时的时间点，streaming 场景下 `endpoint.Middleware` 返回时流可能尚未结束）
  - auth 中间件的 token 校验时机（连接建立时校验一次，还是逐帧校验）是否与业务预期一致
  - `Recovery()` 对 streaming handler 内 panic 的捕获范围是否覆盖流式发送阶段
- 产出：在 `go-framework/kitex/doc.go`（或新增专门文档）中补充"streaming 兼容性说明"章节 + 一个基于现有 IDL 生成流程的最小可运行 streaming 示例（配合上述中间件使用）
- 若评估发现真实兼容性缺陷（如 accesslog 计时错误），记录为独立 Issue，不在本次任务中修复（避免范围蔓延）

## 测试计划

- `go-framework/hertz/sse/writer_test.go`：
  - `WriteEvent` 正常路径（写入成功，格式正确）
  - 断连路径（`ctx` 被 cancel 后，后续 `WriteEvent` 返回 error）
  - 心跳触发（mock ticker 或缩短测试用 heartbeat 间隔，断言 `WriteKeepAlive` 被调用）
  - panic recovery（`Run` 内 handler panic，断言写出 `event:error`、`Close()` 被调用、不向上抛出 panic、`onRecover` 回调被执行）
  - Request ID 缺失时静默降级（未经过 `Responder.Middleware()` 的 ctx，不 panic、日志字段为空）
- Kitex 侧：不新增单测（无新导出 API），提供一个可运行的 streaming demo 作为文档验证，人工/CI 层面确认示例可编译运行
