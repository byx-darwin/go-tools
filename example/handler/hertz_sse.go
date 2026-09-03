// Package handler（本文件）演示 go-framework/hertz/sse 的用法：
// 一个按 query 参数 count 循环推送 message-i 事件的 SSE demo 端点。
package handler

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/byx-darwin/go-tools/go-framework/hertz/sse"
)

// HandleSSEDemo 处理 GET /sse/demo?message=<str>&count=<n>。
// 依次推送 count 个 message 事件（data 为 "<message>-<i>"），
// 演示 sse.Writer 的基本用法与 Request ID 复用（依赖上游已注册
// hertz.Responder.Middleware()）。
//
// 本示例使用默认心跳配置（15s），这是当前唯一可靠的断连检测路径——标准
// Hertz handler 的 ctx 不会在客户端断连时自动 cancel，详见
// go-framework/hertz/sse 包文档「断连检测的真实机制」。若改为
// WithHeartbeatInterval(0) 且 handler 是阻塞等待数据源（不同于本示例的
// 有限 for 循环），断连检测会失效，需自行保证 handler 能通过其他方式退出。
func HandleSSEDemo(ctx context.Context, c *app.RequestContext) {
	message := string(c.Query("message"))
	if message == "" {
		message = "hello"
	}
	count := 3
	if raw := string(c.Query("count")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			count = n
		}
	}

	w := sse.NewWriter(ctx, c)
	_ = w.Run(func(w *sse.Writer) error {
		for i := 0; i < count; i++ {
			data := []byte(fmt.Sprintf("%s-%d", message, i))
			if err := w.WriteEvent("", "message", data); err != nil {
				return err
			}
		}
		return nil
	})
}

// RegisterSSERoutes 注册 go-framework/hertz/sse 示例路由。
func RegisterSSERoutes(h *server.Hertz) {
	h.GET("/sse/demo", HandleSSEDemo)
}
