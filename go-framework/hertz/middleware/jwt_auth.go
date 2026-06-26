package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"

	gojwt "github.com/byx-darwin/go-tools/go-auth/jwt"
)

// JWTAuth 返回 JWT 认证中间件。
// 从 Authorization Bearer 头解析 token，验证签名，将 claims 注入 RequestContext。
// T 必须嵌入 jwt.RegisteredClaims。
//
// 使用方式：
//
//	engine.Use(middleware.JWTAuth[UserClaims](secret))
//	claims, ok := middleware.GetClaims[UserClaims](c)
func JWTAuth[T any](secret []byte) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		token := extractBearerToken(c)
		if token == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		claims, err := gojwt.Verify[T](token, secret)
		if err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		SetClaims(c, claims)
		c.Next(ctx)
	}
}

// extractBearerToken 从 Authorization 头提取 Bearer token。
func extractBearerToken(c *app.RequestContext) string {
	auth := string(c.Request.Header.Peek("Authorization"))
	if auth == "" {
		return ""
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}

	token := strings.TrimSpace(auth[len(prefix):])
	if token == "" {
		return ""
	}

	return token
}
