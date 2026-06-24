// Package middleware 提供 Hertz HTTP 访问日志中间件。
package middleware

import (
	"context"
	"time"

	"gitee.com/byx_darwin/go-tools/go-common/log"
	"github.com/cloudwego/hertz/pkg/app"
)

// AccessLog 返回 Hertz AccessLog 中间件。
// 记录每个 HTTP 请求的 method、path、status、latency。
//
// 用法:
//
//	l := log.New(log.WithLevel("info"))
//	h.Use(middleware.AccessLog(l))
func AccessLog(logger *log.Logger) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		c.Next(ctx)
		latency := time.Since(start)

		logger.InfoContext(ctx, "access",
			"method", string(c.Request.Method()),
			"path", string(c.Request.Path()),
			"status", c.Response.StatusCode(),
			"latency_ms", latency.Milliseconds(),
		)
	}
}
