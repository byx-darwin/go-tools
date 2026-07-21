package handler

import (
	"context"

	"github.com/byx-darwin/go-tools/go-common/auth"
	"github.com/byx-darwin/go-tools/go-common/crypto"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterAkskRoutes 注册 aksk 示例路由。
func RegisterAkskRoutes(h *server.Hertz) {
	h.GET("/common/aksk", akskHandler)
}

func akskHandler(_ context.Context, c *app.RequestContext) {
	// 生成随机 AK。
	ak := auth.GetRandAk(32)

	// 生成密码学安全的随机 SK。
	sk := auth.RefreshSK()

	// 使用 HMAC-SHA256 签名（常见于 API 鉴权场景）。
	message := "GET\n/api/users\n2026-06-26T00:00:00Z"
	signature := crypto.HMACSHA256([]byte(message), []byte(sk))

	results := map[string]any{
		"ak":             ak,
		"sk":             sk,
		"hmac_signature": signature,
		"signed_message": message,
	}

	hertzresp.Success(c, results)
}
