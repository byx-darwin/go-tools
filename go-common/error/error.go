// Package error 提供基于 oops 的统一错误处理机制。
//
// 本包是纯机制包，不持有任何模块的具体错误码：
//
//   - 构造/提取机制：Code、In、Extract、ExtractWithFallback、AsOopsError
//   - 码段边界常量：Framework/Middleware/Project 的 Min/Max
//   - HTTP 状态注册表：RegisterHTTPStatuses 供各属主模块在 init() 注册
//     细粒度映射；HTTPStatus 先查注册表，再走范围兜底
//     （业务码 ≥ ProjectCodeMin → 200；其余 >0 → 500；非 oops → 200）
//
// 具体错误码由各属主模块定义：
//
//	go-framework/error (frameworkerror): 10000-10013 + obs 20601-20605
//	go-auth/error      (autherror):      40000-40099
//	go-middleware/clickhouse:            20401-20403
//	go-middleware/tls:                   20501-20504
//
// 用法：
//
//	import goerror "github.com/byx-darwin/go-tools/go-common/error"
//
//	// 自定义错误码
//	err := goerror.Code(40001).Public("data_duplicate").Wrap(err)
//
//	// 提取
//	code, msg := goerror.Extract(err)
//	httpStatus := goerror.HTTPStatus(err)
package error

import (
	"errors"

	"github.com/samber/oops"
)

// ── 范围常量 ──

// 错误码范围边界常量。
const (
	FrameworkCodeMin  = 10000 // go-framework 最小错误码
	FrameworkCodeMax  = 10499 // go-framework 最大错误码
	MiddlewareCodeMin = 20000 // go-middleware 最小错误码
	MiddlewareCodeMax = 20699 // go-middleware 最大错误码
	ProjectCodeMin    = 40000 // 项目自定义最小错误码
	ProjectCodeMax    = 59999 // 项目自定义最大错误码
)

// ── 构造函数 ──

// Builder 是 oops 错误构建器类型别名。
type Builder = oops.OopsErrorBuilder

// Code 创建带错误码的 oops 构建器。
func Code(code any) Builder { return oops.Code(code) }

// In 创建带 domain 的 oops 构建器。
func In(domain string) Builder { return oops.In(domain) }

// ── 提取函数 ──

// Extract 从 error 中提取 oops 错误码和公开消息。
// 非 oops 错误返回 (0, "")。
func Extract(err error) (code int, public string) {
	if err == nil {
		return 0, ""
	}
	var oopsErr oops.OopsError
	if errors.As(err, &oopsErr) {
		if c, ok := oopsErr.Code().(int); ok {
			return c, oopsErr.Public()
		}
		return 0, oopsErr.Public()
	}
	return 0, ""
}

// ExtractWithFallback 从 error 中提取错误码，非 oops 错误使用 fallbackCode。
func ExtractWithFallback(err error, fallbackCode int) (code int, public string) {
	if err == nil {
		return 0, ""
	}
	code, public = Extract(err)
	if code == 0 {
		return fallbackCode, err.Error()
	}
	return
}

// AsOopsError 将 error 转换为 oops.OopsError。
func AsOopsError(err error) (oops.OopsError, bool) {
	var oopsErr oops.OopsError
	if errors.As(err, &oopsErr) {
		return oopsErr, true
	}
	return oops.OopsError{}, false
}

// ── HTTP 状态码映射 ──

// HTTPStatus 从 error 中提取错误码，映射为 HTTP 状态码。
// 优先级：各模块注册的细粒度映射 → 范围兜底。
func HTTPStatus(err error) int {
	code, _ := Extract(err)
	return httpStatusForCode(code)
}

// httpStatusForCode 按注册表 + 范围兜底映射错误码到 HTTP 状态码。
func httpStatusForCode(code int) int {
	if status, ok := lookupHTTPStatus(code); ok {
		return status
	}
	switch {
	case code >= ProjectCodeMin:
		return 200 // 业务错误（RPC 调用成功，HTTP 200）
	case code > 0:
		return 500 // 未注册的框架/基础设施错误
	default:
		return 200 // 非 oops 错误 / 无错误码
	}
}

// IsClientError 判断错误码是否属于客户端错误（4xx）。
func IsClientError(code int) bool {
	s := httpStatusForCode(code)
	return s >= 400 && s < 500
}

// IsServerError 判断错误码是否属于服务端/基础设施错误（5xx）。
func IsServerError(code int) bool {
	return httpStatusForCode(code) >= 500
}

// IsBusinessErrorCode 判断错误码是否属于业务错误（200，RPC 成功）。
func IsBusinessErrorCode(code int) bool {
	return code >= ProjectCodeMin || (code < FrameworkCodeMin && code > 0)
}
