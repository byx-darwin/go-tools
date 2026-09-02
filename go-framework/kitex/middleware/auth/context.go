// Package auth 提供 Kitex RPC 鉴权中间件：JWT / Session / Device 三种鉴权
// （各含 Server 端校验 + Client 端注入身份，对称实现）以及独立的 panic
// recovery 中间件。
//
// 身份信息通过 github.com/bytedance/gopkg/cloud/metainfo 的
// WithPersistentValue/GetPersistentValue 在 RPC 调用链路中传递，
// 天然支持多跳自动持久透传（无需业务代码在中间服务手动转发）。
//
// 鉴权失败统一通过 rpcerror.OopsStatusAdapter 包装为 Kitex
// BizStatusErrorIface 返回，错误码复用 go-auth/error 与 go-framework/error
// 已有的鉴权错误码。
package auth

import (
	"context"

	"github.com/byx-darwin/go-tools/go-auth/session"
	"github.com/byx-darwin/go-tools/go-framework/kitex/rpcerror"
)

// ctxKey 是本包 context value 的私有 key 类型，避免与其他包的 key 冲突。
type ctxKey string

const (
	ctxKeyJWTClaims ctxKey = "auth:jwt:claims"
	ctxKeyJWTToken  ctxKey = "auth:jwt:token"
	ctxKeySession   ctxKey = "auth:session"
	ctxKeySessionID ctxKey = "auth:session:id"
)

// metainfo persistent key 常量：用于在 RPC 调用链路中透传身份信息。
const (
	metaKeyJWTToken       = "auth-jwt-token"
	metaKeySessionID      = "auth-session-id"
	metaKeyDeviceUserUUID = "auth-device-user-uuid"
	metaKeyDeviceID       = "auth-device-id"
	metaKeyDeviceJTI      = "auth-device-jti"
)

// SetClaims 将 JWT claims 注入 ctx，返回携带该值的新 ctx。
func SetClaims[T any](ctx context.Context, claims *T) context.Context {
	return context.WithValue(ctx, ctxKeyJWTClaims, claims)
}

// GetClaims 从 ctx 提取 JWT claims。
func GetClaims[T any](ctx context.Context) (*T, bool) {
	v := ctx.Value(ctxKeyJWTClaims)
	if v == nil {
		return nil, false
	}
	claims, ok := v.(*T)
	return claims, ok
}

// SetJWTToken 将已校验通过的原始 JWT token 字符串注入 ctx，供
// JWTAuthClient 在同一调用链继续向下游透传时复用（避免重新签发）。
func SetJWTToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, ctxKeyJWTToken, token)
}

// GetJWTToken 从 ctx 提取已校验通过的原始 JWT token 字符串。
func GetJWTToken(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKeyJWTToken).(string)
	return v, ok
}

// SetSession 将 Session 注入 ctx，返回携带该值的新 ctx。
func SetSession(ctx context.Context, s *session.Session) context.Context {
	return context.WithValue(ctx, ctxKeySession, s)
}

// GetSession 从 ctx 提取 Session。
func GetSession(ctx context.Context) (*session.Session, bool) {
	v, ok := ctx.Value(ctxKeySession).(*session.Session)
	return v, ok
}

// SetSessionID 将已校验通过的 Session ID 注入 ctx，供 SessionAuthClient
// 在同一调用链继续向下游透传时复用。
func SetSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeySessionID, id)
}

// GetSessionID 从 ctx 提取已校验通过的 Session ID。
func GetSessionID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxKeySessionID).(string)
	return v, ok
}

// bizError 将错误包装为 Kitex BizStatusErrorIface，使 oops 错误码/公开消息
// 能通过 Kitex BizStatus 机制正确返回给调用方。
func bizError(err error) error {
	return &rpcerror.OopsStatusAdapter{Err: err}
}
