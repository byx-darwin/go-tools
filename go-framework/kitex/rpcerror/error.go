// Package rpcerror 提供 Kitex RPC 框架的错误分类与适配。
//
// 框架错误码与预定义错误位于 go-framework/error（frameworkerror）；
// 错误构造与提取机制位于 go-common/error。
// 本包仅保留 Kitex 特定的分类逻辑和 BizStatus 适配器。
package rpcerror

import (
	"errors"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/samber/oops"
)

// ── 错误分类 ──

// ErrorCategory 错误类别
type ErrorCategory int

// 错误分类常量。
const (
	CategoryBusiness  ErrorCategory = iota // 业务错误（oops）
	CategoryFramework                      // Kitex 框架错误
	CategoryUnknown                        // 未知
)

// Classify 对错误进行分类。
func Classify(err error) ErrorCategory {
	if err == nil {
		return CategoryUnknown
	}
	var oopsErr oops.OopsError
	if errors.As(err, &oopsErr) {
		return CategoryBusiness
	}
	if kerrors.IsKitexError(err) {
		return CategoryFramework
	}
	return CategoryUnknown
}

// IsFrameworkError 判断是否为 Kitex 框架错误。
func IsFrameworkError(err error) bool { return Classify(err) == CategoryFramework }

// IsBusinessError 判断是否为业务错误（oops）。
func IsBusinessError(err error) bool { return Classify(err) == CategoryBusiness }

// IsTimeout 判断是否为超时错误（Kitex 原生超时或业务 RPCTimeout）。
func IsTimeout(err error) bool {
	if kerrors.IsTimeoutError(err) {
		return true
	}
	code, _ := goerror.Extract(err)
	return code == frameworkerror.CodeRPCTimeout
}

// FrameworkErrorName 返回 Kitex 框架错误的名称，非框架错误返回空字符串。
func FrameworkErrorName(err error) string {
	if !IsFrameworkError(err) {
		return ""
	}
	switch {
	case errors.Is(err, kerrors.ErrRPCTimeout):
		return "ErrRPCTimeout"
	case errors.Is(err, kerrors.ErrInternalException):
		return "ErrInternalException"
	case errors.Is(err, kerrors.ErrServiceDiscovery):
		return "ErrServiceDiscovery"
	case errors.Is(err, kerrors.ErrGetConnection):
		return "ErrGetConnection"
	case errors.Is(err, kerrors.ErrLoadbalance):
		return "ErrLoadbalance"
	case errors.Is(err, kerrors.ErrNoMoreInstance):
		return "ErrNoMoreInstance"
	case errors.Is(err, kerrors.ErrCanceledByBusiness):
		return "ErrCanceledByBusiness"
	case errors.Is(err, kerrors.ErrTimeoutByBusiness):
		return "ErrTimeoutByBusiness"
	case errors.Is(err, kerrors.ErrACL):
		return "ErrACL"
	case errors.Is(err, kerrors.ErrCircuitBreak):
		return "ErrCircuitBreak"
	case errors.Is(err, kerrors.ErrRemoteOrNetwork):
		return "ErrRemoteOrNetwork"
	case errors.Is(err, kerrors.ErrOverlimit):
		return "ErrOverlimit"
	case errors.Is(err, kerrors.ErrPanic):
		return "ErrPanic"
	case errors.Is(err, kerrors.ErrBiz):
		return "ErrBiz"
	case errors.Is(err, kerrors.ErrRetry):
		return "ErrRetry"
	case errors.Is(err, kerrors.ErrRoute):
		return "ErrRoute"
	case errors.Is(err, kerrors.ErrPayloadValidation):
		return "ErrPayloadValidation"
	default:
		return "UnknownKitexError"
	}
}

// ── Kitex BizStatus 适配 ──

// BizStatusGetter Kitex BizStatus 接口。
type BizStatusGetter interface {
	BizStatusCode() int32
	BizMessage() string
	BizExtra() map[string]string
	Error() string
}

// OopsStatusAdapter 将 oops error 适配为 Kitex BizStatus。
type OopsStatusAdapter struct {
	Err   error
	Extra map[string]string
}

// 编译期接口断言。
var _ kerrors.BizStatusErrorIface = (*OopsStatusAdapter)(nil)

// BizStatusCode 返回 oops 错误码。
func (a *OopsStatusAdapter) BizStatusCode() int32 {
	code, _ := goerror.Extract(a.Err)
	return int32(code)
}

// BizMessage 返回公开错误消息。
func (a *OopsStatusAdapter) BizMessage() string { _, public := goerror.Extract(a.Err); return public }

// BizExtra 返回附加信息。
func (a *OopsStatusAdapter) BizExtra() map[string]string { return a.Extra }

// Error 返回错误字符串。
func (a *OopsStatusAdapter) Error() string { return a.Err.Error() }
