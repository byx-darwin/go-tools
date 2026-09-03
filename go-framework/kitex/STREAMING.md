# Kitex Streaming RPC 中间件兼容性说明

本仓库 `go-framework/kitex` 现有中间件均基于 `endpoint.Middleware`
（`func(ctx, req, resp) error` 单次调用模型）。Kitex 的 streaming 能力由
IDL（Protobuf `stream` 关键字）驱动生成，`endpoint.Middleware` 包裹的是
"流的建立"（即 `next(ctx, req, resp)` 返回前）而非逐帧收发过程本身。
本文档记录现有中间件在 streaming 场景下的行为评估结论。

## 评估范围

- `middleware/auth/{jwt,session,device}.go` — 鉴权中间件
- `middleware/auth/recovery.go` — panic recovery 中间件
- `middleware/accesslog.go` — 访问日志中间件
- `observability/suite.go` — OTel Tracing/Metrics

## 结论

| 中间件 | streaming 场景行为 | 结论 |
|---|---|---|
| JWT/Session/Device Auth | Token 校验发生在 `next(ctx, req, resp)` 被调用**之前**，即流建立（握手）阶段，对整个流生效一次，不逐帧重复校验 | ✅ 兼容。语义等价于 gRPC/Kitex streaming 的标准做法（连接级鉴权），无需改动 |
| `Recovery()` | `defer recover()` 包裹 `next(ctx, req, resp)` 的调用栈；对于 server-streaming，`next` 对应的生成代码在 handler 内部循环调用 `stream.Send(...)`，该循环运行在 `next` 返回之前的同一 goroutine 调用栈内，因此 handler 内的 panic（含发送阶段）仍会被 `next` 的调用方（即本中间件）捕获 | ✅ 兼容，覆盖流式发送阶段的 panic |
| `accesslog.go` | 记录耗时的起止时间点包裹在 `next(ctx, req, resp)` 前后，对 streaming 而言测得的是"整个流从建立到结束"的总时长，而非单次请求延迟；这是预期行为（无更细粒度的逐帧计时需求），无需改动 | ✅ 兼容（语义随 streaming 自然调整，非缺陷） |
| `observability/suite.go` | Kitex `stats.Tracer` 由 Kitex 运行时在流生命周期的固定检点回调（`Start`/`Finish` 等），与是否 streaming 无关，`suite.Options()` 注入方式不变 | ✅ 兼容，无需改动 |

**结论：现有中间件全部兼容 streaming RPC 场景，本次不新增/修改任何导出 API。**
若未来出现细粒度逐帧鉴权/限流等新需求，应作为独立 Issue 评估，不在本次范围内。

## 使用示例

参见 `example/rpc/service.go` 的 `StreamEcho` 方法与 `example/rpc/client.go`
的流式调用示例，演示如何在 streaming 方法上组合 `auth.Recovery()` +
`observability` Suite。
