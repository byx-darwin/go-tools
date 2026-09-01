package middleware

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/byx-darwin/go-tools/go-auth/device"
	autherror "github.com/byx-darwin/go-tools/go-auth/error"
)

// DeviceClaims 提取函数类型。
// 从用户自定义的 Claims 中提取 DeviceAuth 所需的字段。
// userUUID 通常对应 JWT RegisteredClaims.Subject。
type DeviceClaims func(claims any) (userUUID, deviceID, jti string)

// DeviceAuth 返回设备会话检查中间件。
// 需配合 JWTAuth 使用。通过 extract 函数从用户 Claims 中提取 user_uuid、device_id、jti，
// 然后调用 device.Store.CheckDevice 验证设备会话是否有效。
//
// extract 函数的 claims 参数是 JWTAuth 注入的 *T 指针。
// 用户需提供提取逻辑，例如：
//
//	extract := func(claims any) (string, string, string) {
//	    c := claims.(*UserClaims)
//	    return c.Subject, c.DeviceID, c.JTI
//	}
//	engine.Use(middleware.DeviceAuth(deviceStore, extract))
func DeviceAuth(store device.Store, extract DeviceClaims) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		claims, ok := c.Get(string(ctxKeyClaims))
		if !ok || extract == nil {
			writeAuthError(c, autherror.ErrTokenInvalid.Errorf("missing claims in context"))
			return
		}

		userUUID, deviceID, jti := extract(claims)
		if userUUID == "" || deviceID == "" || jti == "" {
			writeAuthError(c, autherror.ErrTokenInvalid.Errorf("incomplete device claims"))
			return
		}

		valid, err := store.CheckDevice(ctx, userUUID, deviceID, jti)
		if err != nil {
			writeAuthError(c, autherror.ErrJWTVerifyFailed.Wrap(err))
			return
		}
		if !valid {
			writeAuthError(c, autherror.ErrDeviceKicked.Errorf("device kicked"))
			return
		}

		c.Next(ctx)
	}
}
