package middleware

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
)

// Cors 返回 CORS 中间件，允许跨域请求。
func Cors() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Header("Access-Control-Allow-Headers", "Content-Type,X-Authorization, X-Signature")
		ctx.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS,DELETE,PUT")
		ctx.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin,New-Token,New-Expires-At,Access-Control-Allow-Headers, Content-Type")
		ctx.Header("Access-Control-Allow-Credentials", "true")

		if string(ctx.Request.Method()) == "OPTIONS" {
			ctx.AbortWithStatus(http.StatusNoContent)
		}
		ctx.Next(c)
	}
}
