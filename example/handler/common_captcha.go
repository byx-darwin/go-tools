package handler

import (
	"context"

	"github.com/byx-darwin/go-tools/go-common/captcha"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// imageCaptcha 全局验证码实例（应用启动时创建一次）。
var imageCaptcha = captcha.NewImageCaptcha()

// RegisterCaptchaRoutes 注册 captcha 示例路由。
func RegisterCaptchaRoutes(h *server.Hertz) {
	h.GET("/common/captcha", captchaGenerateHandler)
	h.POST("/common/captcha/verify", captchaVerifyHandler)
}

func captchaGenerateHandler(ctx context.Context, c *app.RequestContext) {
	id, b64, answer, err := imageCaptcha.Generate()
	if err != nil {
		hertzresp.Error(ctx, c, err, "captcha generate failed")
		return
	}

	hertzresp.Success(c, map[string]any{
		"id":     id,
		"image":  b64,
		"answer": answer, // 生产环境不应返回答案，此处仅用于演示
	})
}

func captchaVerifyHandler(ctx context.Context, c *app.RequestContext) {
	// 从 JSON body 获取参数。
	type verifyReq struct {
		ID     string `json:"id"`
		Answer string `json:"answer"`
	}

	var req verifyReq
	if err := c.BindAndValidate(&req); err != nil {
		hertzresp.Error(ctx, c, err, "invalid request")
		return
	}

	ok := imageCaptcha.Verify(req.ID, req.Answer, true)
	hertzresp.Success(c, map[string]any{
		"verified": ok,
	})
}
