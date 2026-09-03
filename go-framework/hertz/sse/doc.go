// Package sse 在 Hertz 官方 pkg/protocol/sse 之上提供一层贴合
// go-framework 现有 Responder/中间件规范的 SSE（Server-Sent Events）封装。
//
// # 快速开始
//
//	engine.Use(responder.Middleware()) // 必须先注册，SSE Request ID 依赖它
//	engine.GET("/stream", func(ctx context.Context, c *app.RequestContext) {
//	    w := sse.NewWriter(ctx, c, sse.WithHeartbeatInterval(15*time.Second))
//	    _ = w.Run(func(w *sse.Writer) error {
//	        for event := range dataCh {
//	            if err := w.WriteEvent("", "message", event); err != nil {
//	                return err // 客户端已断开，退出事件循环
//	            }
//	        }
//	        return nil
//	    })
//	})
//
// # 前置条件
//
// ctx 所在请求链路必须已经过 hertz.Responder.Middleware()，否则 Request ID
// 特性静默失效（hertz.RequestIDFrom(ctx) 返回空字符串，不 panic、不报错）。
//
// # 错误处理
//
// SSE 响应头一旦提交（NewWriter 内部立即完成），无法再切回 JSON/Protobuf，
// 因此错误一律走 event:error（JSON 三段式 {code,msg,data}），不复用
// Responder.Error() 的内容协商逻辑。Run 内部的 panic 会被捕获、转换为
// event:error 并记录结构化日志，不会导致进程崩溃或向上传播。event:error 的
// code 字段是 HTTP 状态码语义（如 500），与 Responder 正常 JSON 响应里的
// code（go-framework 10000-10499 业务错误码）不是同一套编号体系，客户端
// 需按 SSE 场景单独处理，不能直接复用解析 Responder 响应的逻辑。
//
// # 断连检测的真实机制（重要）
//
// Hertz v0.10.5 标准（非 hijack）路由 handler 收到的 context.Context 在
// 客户端断开 TCP 连接时**不会**被 cancel（这一点不同于 net/http 的
// r.Context()）。因此 Run 内部对 ctx.Done() 的监听实际只能捕获"业务方自己
// 主动 cancel 传入的 ctx"这一种情况，无法单独探测真实客户端断连。
//
// 断连检测在实践中依赖的是心跳保活的写失败路径：WriteKeepAlive() 在底层
// socket 已断开时会返回 error，heartbeatLoop 据此调用 doClose()。也就是说：
//
//   - heartbeatInterval > 0（默认 15s）：客户端断连后，最迟一个心跳周期内
//     会被检测到并关闭 Writer——这是当前唯一可靠的断连检测路径。
//   - WithHeartbeatInterval(0)（禁用心跳）：若 handler 内部是纯阻塞等待
//     （如 `for e := range dataCh`，dataCh 迟迟不来数据），断连后既不会
//     触发心跳写失败，也不会有 ctx.Done() 信号，Run 内的 goroutine 会
//     无限阻塞，造成 goroutine 泄漏。禁用心跳的场景必须自行保证 handler
//     能通过其他方式（如自带超时、显式 cancel）退出，否则不要禁用心跳。
package sse
