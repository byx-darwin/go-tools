package hertzlog

import (
	"context"

	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/cloudwego/hertz/pkg/app"
)

// HertzRequestIDMiddleware 从 HTTP header 提取 request_id 并注入 context。
func HertzRequestIDMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		requestID := string(c.Request.Header.Peek("X-Request-ID"))
		if requestID != "" {
			ctx = log.WithRequestID(ctx, requestID)
		}
		c.Next(ctx)
	}
}
