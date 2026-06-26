package handler

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	hertzmw "github.com/byx-darwin/go-tools/go-framework/hertz/middleware"
)

// DeviceAppClaims 设备认证扩展 Claims。
//
// 嵌入 AppClaims，扩展 DeviceID 字段用于设备认证。
// JTI 使用标准 RegisteredClaims.Id 字段（jwt.RegisteredClaims.Id）。
type DeviceAppClaims struct {
	// DeviceID 设备唯一标识。
	DeviceID string `json:"device_id"`
	AppClaims
}

// ExtractDeviceClaims 从 DeviceAppClaims 提取 device 中间件所需字段。
//
// 返回 userUUID（AppClaims.UserUUID）、deviceID（DeviceAppClaims.DeviceID）、
// jti（RegisteredClaims.Id）。供 hertzmw.DeviceAuth 使用。
func ExtractDeviceClaims(claims any) (userUUID, deviceID, jti string) {
	c, ok := claims.(*DeviceAppClaims)
	if !ok {
		return "", "", ""
	}
	return c.UserUUID, c.DeviceID, c.ID
}

// HandleProtectedJWT 返回 JWT claims 中的用户信息。
//
// 需通过 JWT 认证中间件。从 context 提取 AppClaims 并返回 user_uuid、subject、issuer。
func HandleProtectedJWT(ctx context.Context, c *app.RequestContext) {
	claims, ok := hertzmw.GetClaims[AppClaims](c)
	if !ok || claims == nil {
		hertzresp.ErrorWithCode(ctx, c, 401, 40001, "claims not found")
		return
	}

	hertzresp.Success(c, map[string]any{
		"user_uuid": claims.UserUUID,
		"subject":   claims.Subject,
		"issuer":    claims.Issuer,
	})
}

// HandleProtectedSession 返回 Session 信息。
//
// 需通过 Session 认证中间件。从 context 提取 Session 并返回 session 详情。
func HandleProtectedSession(ctx context.Context, c *app.RequestContext) {
	sess, ok := hertzmw.GetSession(c)
	if !ok || sess == nil {
		hertzresp.ErrorWithCode(ctx, c, 401, 40002, "session not found")
		return
	}

	hertzresp.Success(c, map[string]any{
		"session_id": sess.ID,
		"user_uuid":  sess.UserUUID,
		"data":       sess.Data,
		"expires_at": sess.ExpiresAt,
	})
}

// HandleProtectedDevice 返回设备认证信息。
//
// 需通过 JWT + Device 认证中间件。从 context 提取 DeviceAppClaims 并返回设备信息。
func HandleProtectedDevice(ctx context.Context, c *app.RequestContext) {
	claims, ok := hertzmw.GetClaims[DeviceAppClaims](c)
	if !ok || claims == nil {
		hertzresp.ErrorWithCode(ctx, c, 401, 40001, "claims not found")
		return
	}

	hertzresp.Success(c, map[string]any{
		"user_uuid": claims.UserUUID,
		"device_id": claims.DeviceID,
		"jti":       claims.ID,
		"subject":   claims.Subject,
	})
}
