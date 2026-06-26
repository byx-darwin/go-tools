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
	CodeTokenInvalid   = 40001 // Token 无效
	CodeTokenExpired   = 40002 // Token 已过期
	CodeTokenRevoked   = 40003 // Token 已撤销
	CodeDeviceKicked   = 40004 // 设备已被踢出
	CodeSessionInvalid = 40005 // Session 无效
	CodeSessionExpired = 40006 // Session 已过期
)

// 预定义认证错误构造器。
var (
	ErrTokenInvalid   = goerror.Code(CodeTokenInvalid).Public("token_invalid")     // Token 无效
	ErrTokenExpired   = goerror.Code(CodeTokenExpired).Public("token_expired")     // Token 已过期
	ErrTokenRevoked   = goerror.Code(CodeTokenRevoked).Public("token_revoked")     // Token 已撤销
	ErrDeviceKicked   = goerror.Code(CodeDeviceKicked).Public("device_kicked")     // 设备已被踢出
	ErrSessionInvalid = goerror.Code(CodeSessionInvalid).Public("session_invalid") // Session 无效
	ErrSessionExpired = goerror.Code(CodeSessionExpired).Public("session_expired") // Session 已过期
)
