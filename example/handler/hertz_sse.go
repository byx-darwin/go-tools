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
