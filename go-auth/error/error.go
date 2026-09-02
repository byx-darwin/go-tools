// Package autherror 提供 go-auth 模块的认证错误码和预定义错误构造器。
//
// 错误码范围：40000-40099（go-auth 认证错误）。
package autherror

import (
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// Builder 是错误构建器类型别名。
type Builder = goerror.Builder

// 认证错误码 40000-40099。
const (
	CodeTokenInvalid       = 40001 // Token 无效
	CodeTokenExpired       = 40002 // Token 已过期
	CodeTokenRevoked       = 40003 // Token 已撤销
	CodeDeviceKicked       = 40004 // 设备已被踢出
	CodeSessionInvalid     = 40005 // Session 无效
	CodeSessionExpired     = 40006 // Session 已过期
	CodeJWTSignFailed      = 40007 // JWT 签名失败
	CodeJWTVerifyFailed    = 40008 // JWT 验证失败
	CodeJWTRefreshFailed   = 40009 // JWT 刷新失败
	CodeJWTKeyTypeMismatch = 40010 // JWT 密钥类型与签名算法不匹配
	CodeJWTWeakSecret      = 40011 // JWT HMAC 密钥强度不足（长度低于哈希算法输出长度）
)

// 预定义认证错误构造器。
var (
	ErrTokenInvalid       = goerror.Code(CodeTokenInvalid).Public("token_invalid")               // Token 无效
	ErrTokenExpired       = goerror.Code(CodeTokenExpired).Public("token_expired")               // Token 已过期
	ErrTokenRevoked       = goerror.Code(CodeTokenRevoked).Public("token_revoked")               // Token 已撤销
	ErrDeviceKicked       = goerror.Code(CodeDeviceKicked).Public("device_kicked")               // 设备已被踢出
	ErrSessionInvalid     = goerror.Code(CodeSessionInvalid).Public("session_invalid")           // Session 无效
	ErrSessionExpired     = goerror.Code(CodeSessionExpired).Public("session_expired")           // Session 已过期
	ErrJWTSignFailed      = goerror.Code(CodeJWTSignFailed).Public("jwt_sign_failed")            // JWT 签名失败
	ErrJWTVerifyFailed    = goerror.Code(CodeJWTVerifyFailed).Public("jwt_verify_failed")        // JWT 验证失败
	ErrJWTRefreshFailed   = goerror.Code(CodeJWTRefreshFailed).Public("jwt_refresh_failed")      // JWT 刷新失败
	ErrJWTKeyTypeMismatch = goerror.Code(CodeJWTKeyTypeMismatch).Public("jwt_key_type_mismatch") // JWT 密钥类型与签名算法不匹配
	ErrJWTWeakSecret      = goerror.Code(CodeJWTWeakSecret).Public("jwt_weak_secret")            // JWT HMAC 密钥强度不足
)

func init() {
	goerror.RegisterHTTPStatuses(map[int]int{
		CodeTokenInvalid:       401,
		CodeTokenExpired:       401,
		CodeTokenRevoked:       401,
		CodeDeviceKicked:       403,
		CodeSessionInvalid:     401,
		CodeSessionExpired:     401,
		CodeJWTSignFailed:      500,
		CodeJWTVerifyFailed:    500,
		CodeJWTRefreshFailed:   500,
		CodeJWTKeyTypeMismatch: 500,
		CodeJWTWeakSecret:      500,
	})
}
