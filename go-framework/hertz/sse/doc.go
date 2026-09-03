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
// event:error 并记录结构化日志，不会导致进程崩溃或向上传播。
package sse
