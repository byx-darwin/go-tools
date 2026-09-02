package auth

import (
	"context"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/kitex/pkg/endpoint"

	"github.com/byx-darwin/go-tools/go-auth/device"
	autherror "github.com/byx-darwin/go-tools/go-auth/error"
)

// DeviceClaimsProvider 从 ctx 提取设备身份三元组（userUUID、deviceID、
// jti），用于 DeviceAuthClient 向下游透传。实现方通常从
// auth.GetClaims 取出本地已验证的 JWT claims 中提取这三个字段。
type DeviceClaimsProvider func(ctx context.Context) (userUUID, deviceID, jti string, ok bool)

// DeviceAuthServer 返回 Kitex Server 端设备会话校验中间件。
// 从 incoming metainfo（persistent key）读取 userUUID/deviceID/jti，
// 调用 device.Store.CheckDevice 验证设备会话是否有效。
//
// 使用方式：
//
//	server.WithMiddleware(auth.DeviceAuthServer(deviceStore))
func DeviceAuthServer(store device.Store) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			userUUID, ok1 := metainfo.GetPersistentValue(ctx, metaKeyDeviceUserUUID)
			deviceID, ok2 := metainfo.GetPersistentValue(ctx, metaKeyDeviceID)
			jti, ok3 := metainfo.GetPersistentValue(ctx, metaKeyDeviceJTI)
			if !ok1 || !ok2 || !ok3 || userUUID == "" || deviceID == "" || jti == "" {
				return bizError(autherror.ErrTokenInvalid.Errorf("incomplete device identity in metainfo"))
			}

			valid, err := store.CheckDevice(ctx, userUUID, deviceID, jti)
			if err != nil {
				return bizError(autherror.ErrJWTVerifyFailed.Wrap(err))
			}
			if !valid {
				return bizError(autherror.ErrDeviceKicked.Errorf("device kicked"))
			}

			return next(ctx, req, resp)
		}
	}
}

// DeviceAuthClient 返回 Kitex Client 端设备身份注入中间件。
// 调用 extract 从 ctx 提取设备身份三元组，通过
// metainfo.WithPersistentValue 写入，随调用链自动持久透传到所有下游。
//
// extract 不允许为 nil；提取结果不完整时返回错误。
func DeviceAuthClient(extract DeviceClaimsProvider) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			if extract == nil {
				return bizError(autherror.ErrTokenInvalid.Errorf("no DeviceClaimsProvider configured"))
			}

			userUUID, deviceID, jti, ok := extract(ctx)
			if !ok || userUUID == "" || deviceID == "" || jti == "" {
				return bizError(autherror.ErrTokenInvalid.Errorf("incomplete device claims from provider"))
			}

			ctx = metainfo.WithPersistentValue(ctx, metaKeyDeviceUserUUID, userUUID)
			ctx = metainfo.WithPersistentValue(ctx, metaKeyDeviceID, deviceID)
			ctx = metainfo.WithPersistentValue(ctx, metaKeyDeviceJTI, jti)
			return next(ctx, req, resp)
		}
	}
}
