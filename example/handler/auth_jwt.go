package handler

import (
	"context"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/byx-darwin/go-tools/go-auth/jwt"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// AppClaims 应用自定义 JWT Claims。
//
// 嵌入 gojwt.RegisteredClaims（golang-jwt 标准注册声明），
// 扩展 UserUUID 字段用于业务标识。
type AppClaims struct {
	UserUUID string `json:"user_uuid"`
	gojwt.RegisteredClaims
}

// JWT 配置（由 main 通过 SetJWTConfig 注入）。
var (
	jwtSecret        []byte
	jwtIssuer        string
	jwtAccessExpiry  time.Duration
	jwtRefreshExpiry time.Duration
)

// SetJWTConfig 注入 JWT 配置（在 main 中调用）。
func SetJWTConfig(secret, issuer string, accessExpiry, refreshExpiry time.Duration) {
	jwtSecret = []byte(secret)
	jwtIssuer = issuer
	jwtAccessExpiry = accessExpiry
	jwtRefreshExpiry = refreshExpiry
}

// RegisterJWTRoutes 注册 JWT 示例路由。
func RegisterJWTRoutes(h *server.Hertz) {
	h.POST("/auth/jwt/sign", jwtSignHandler)
	h.POST("/auth/jwt/sign-device", jwtSignDeviceHandler)
	h.POST("/auth/jwt/verify", jwtVerifyHandler)
	h.POST("/auth/jwt/refresh", jwtRefreshHandler)
}

// jwtSignHandler 签发 access_token + refresh_token。
//
// 请求体：{"user_id": "xxx"}
// 响应：access_token、refresh_token 以及原始 claims。
func jwtSignHandler(ctx context.Context, c *app.RequestContext) {
	var req struct {
		UserID string `json:"user_id" vd:"len($)>0"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		hertzresp.Error(ctx, c, err, "invalid request")
		return
	}

	opts := []jwt.Option{
		jwt.WithIssuer(jwtIssuer),
	}

	// 签发 access_token（短过期时间）。
	accessClaims := AppClaims{UserUUID: req.UserID}
	accessToken, err := jwt.Sign(accessClaims, jwtSecret, append(opts, jwt.WithExpiration(jwtAccessExpiry))...)
	if err != nil {
		hertzresp.Error(ctx, c, err, "sign access token failed")
		return
	}

	// 签发 refresh_token（长过期时间，Subject 区分用途）。
	refreshClaims := AppClaims{UserUUID: req.UserID}
	refreshClaims.Subject = "refresh"
	refreshToken, err := jwt.Sign(refreshClaims, jwtSecret, append(opts, jwt.WithExpiration(jwtRefreshExpiry))...)
	if err != nil {
		hertzresp.Error(ctx, c, err, "sign refresh token failed")
		return
	}

	hertzresp.Success(c, map[string]any{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"claims": map[string]any{
			"user_uuid": req.UserID,
			"issuer":    jwtIssuer,
		},
	})
}

// jwtVerifyHandler 验证 JWT，返回解析后的 claims。
//
// 请求体：{"token": "xxx"}
// 响应：解析后的 claims（user_uuid、sub、exp、iss 等）。
func jwtVerifyHandler(ctx context.Context, c *app.RequestContext) {
	var req struct {
		Token string `json:"token" vd:"len($)>0"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		hertzresp.Error(ctx, c, err, "invalid request")
		return
	}

	claims, err := jwt.Verify[AppClaims](req.Token, jwtSecret)
	if err != nil {
		hertzresp.Error(ctx, c, err, "verify token failed")
		return
	}

	hertzresp.Success(c, map[string]any{
		"valid":      true,
		"user_uuid":  claims.UserUUID,
		"subject":    claims.Subject,
		"issuer":     claims.Issuer,
		"expires_at": claims.ExpiresAt,
	})
}

// jwtRefreshHandler 使用 refresh_token 签发新的 access_token。
//
// 请求体：{"refresh_token": "xxx"}
// 响应：新的 access_token（保留原 claims 中的 UserUUID）。
func jwtRefreshHandler(ctx context.Context, c *app.RequestContext) {
	var req struct {
		RefreshToken string `json:"refresh_token" vd:"len($)>0"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		hertzresp.Error(ctx, c, err, "invalid request")
		return
	}

	// Refresh 内部先验证原 Token，再以新过期时间重新签发。
	newToken, err := jwt.Refresh[AppClaims](req.RefreshToken, jwtSecret,
		jwt.WithIssuer(jwtIssuer),
		jwt.WithExpiration(jwtAccessExpiry),
	)
	if err != nil {
		hertzresp.Error(ctx, c, err, "refresh token failed")
		return
	}

	hertzresp.Success(c, map[string]any{
		"access_token": newToken,
	})
}

// jwtSignDeviceHandler 签发包含设备信息的 JWT（用于设备认证场景）。
//
// 请求体：{"user_id": "xxx", "device_id": "yyy", "jti": "zzz"}
// 响应：access_token，claims 中包含 device_id 和 jti。
func jwtSignDeviceHandler(ctx context.Context, c *app.RequestContext) {
	var req struct {
		UserID   string `json:"user_id" vd:"len($)>0"`
		DeviceID string `json:"device_id" vd:"len($)>0"`
		JTI      string `json:"jti" vd:"len($)>0"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		hertzresp.Error(ctx, c, err, "invalid request")
		return
	}

	claims := DeviceAppClaims{
		AppClaims: AppClaims{
			UserUUID: req.UserID,
		},
		DeviceID: req.DeviceID,
	}
	claims.ID = req.JTI

	token, err := jwt.Sign(claims, jwtSecret,
		jwt.WithIssuer(jwtIssuer),
		jwt.WithExpiration(jwtAccessExpiry),
	)
	if err != nil {
		hertzresp.Error(ctx, c, err, "sign device token failed")
		return
	}

	hertzresp.Success(c, map[string]any{
		"access_token": token,
		"claims": map[string]any{
			"user_uuid": req.UserID,
			"device_id": req.DeviceID,
			"jti":       req.JTI,
			"issuer":    jwtIssuer,
		},
	})
}
