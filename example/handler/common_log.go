package handler

import (
	"context"

	"github.com/byx-darwin/go-tools/go-common/log"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterLogRoutes 注册 log 示例路由。
func RegisterLogRoutes(h *server.Hertz) {
	h.GET("/common/log", logHandler)
}

func logHandler(ctx context.Context, c *app.RequestContext) {
	// 基础日志输出。
	log.L().Info("demo log message", "component", "handler", "action", "log_demo")

	// 分类日志。
	log.L().Info("access log example",
		"category", log.CategoryAccess,
		"method", "GET",
		"path", "/common/log",
	)

	// 带 Request ID 的 context 日志。
	reqCtx := log.WithRequestID(ctx, "demo-request-001")
	log.L().InfoContext(reqCtx, "context log with request_id")

	// 脱敏演示：直接创建 Masker 展示脱敏效果。
	masker := log.NewMasker(log.MaskConfig{
		Enabled:      true,
		Mode:         "partial",
		MaskedFields: []string{"password", "token"},
	})
	_ = masker // 实际脱敏在全局 Logger 初始化时通过 Config 注入

	results := map[string]any{
		"message":     "check server logs for demo output",
		"categories":  []string{log.CategoryAccess, log.CategoryError, log.CategoryBiz, log.CategoryRPC, log.CategoryDB},
		"context_log": "see infoContext call above",
		"mask_demo": map[string]any{
			"enabled":       true,
			"mode":          "partial",
			"masked_fields": []string{"password", "token"},
		},
	}

	hertzresp.Success(c, results)
}
