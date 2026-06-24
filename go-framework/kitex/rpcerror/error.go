// Package rpcerror 提供基于 oops 的 RPC 错误处理。
//
// 错误码范围（详见 specs/00_overview.md）：
//
//	go-framework: 10000-10499  (system, param, auth, config, RPC middleware)
//	go-middleware:  20000-20699 (redis, kafka, db, es, clickhouse, observability)
//	Project custom: 40000-59999 (business modules, external dependencies)
//
// 用法：
//
//	// 创建带错误码的 oops 错误
//	err := rpcerror.Code(10001).Public("param_invalid").Wrap(err)
//
//	// 使用预定义错误
//	err := rpcerror.ErrParamInvalid.Wrap(originalErr)
//
//	// 从 error 中提取错误码
//	code, msg := rpcerror.Extract(err)
package rpcerror

import (
	"errors"

	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/samber/oops"
)

// 编译期接口断言：确保 OopsStatusAdapter 实现 Kitex BizStatusErrorIface。
var _ kerrors.BizStatusErrorIface = (*OopsStatusAdapter)(nil)

// ── 错误分类 ──

// ErrorCategory 错误类别
type ErrorCategory int

const (
	// CategoryBusiness 业务错误（oops 包装的错误）
	CategoryBusiness ErrorCategory = iota
	// CategoryFramework Kitex 框架错误（kerrors.ErrRPCTimeout 等）
	CategoryFramework
	// CategoryUnknown 未知错误
	CategoryUnknown
)

// Classify 对错误进行分类。
//
//	err := kerrors.ErrRPCTimeout
//	cat := rpcerror.Classify(err)  // CategoryFramework
//
//	err = rpcerror.ErrParamInvalid.Wrap(errors.New("bad"))
//	cat := rpcerror.Classify(err)  // CategoryBusiness
func Classify(err error) ErrorCategory {
	if err == nil {
		return CategoryUnknown
	}
	// 先检查是否是 oops 错误（业务错误）
	var oopsErr oops.OopsError
	if errors.As(err, &oopsErr) {
		return CategoryBusiness
	}
	// 再检查是否是 Kitex 框架错误
	if kerrors.IsKitexError(err) {
		return CategoryFramework
	}
	return CategoryUnknown
}

// IsFrameworkError 判断是否是 Kitex 框架错误。
func IsFrameworkError(err error) bool {
	return Classify(err) == CategoryFramework
}

// IsBusinessError 判断是否是业务错误（oops 错误）。
func IsBusinessError(err error) bool {
	return Classify(err) == CategoryBusiness
}

// IsTimeout 判断是否是超时错误（框架或业务）。
func IsTimeout(err error) bool {
	if kerrors.IsTimeoutError(err) {
		return true
	}
	// 检查业务错误码是否是超时
	code, _ := Extract(err)
	return code == CodeRPCTimeout
}

// FrameworkErrorName 返回 Kitex 框架错误的名称（用于日志）。
// 如果不是框架错误，返回空字符串。
func FrameworkErrorName(err error) string {
	if !IsFrameworkError(err) {
		return ""
	}
	// 通过 errors.Is 判断具体类型
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

// ── 错误码范围 ──

const (
	// FrameworkCodeMin go-framework 错误码最小值
	FrameworkCodeMin = 10000
	// FrameworkCodeMax go-framework 错误码最大值
	FrameworkCodeMax = 10499

	// MiddlewareCodeMin go-middleware 错误码最小值
	MiddlewareCodeMin = 20000
	// MiddlewareCodeMax go-middleware 错误码最大值
	MiddlewareCodeMax = 20699

	// ProjectCodeMin 项目自定义错误码最小值
	ProjectCodeMin = 40000
	// ProjectCodeMax 项目自定义错误码最大值
	ProjectCodeMax = 59999
)

// ── go-framework 预定义错误码 (10000-10499) ──

const (
	// CodeSystem 系统错误
	CodeSystem = 10000
	// CodeParamInvalid 参数无效
	CodeParamInvalid = 10001
	// CodeAuthFailed 鉴权失败
	CodeAuthFailed = 10002
	// CodeConfigNotFound 配置未找到
	CodeConfigNotFound = 10003
	// CodeConfigInvalid 配置无效
	CodeConfigInvalid = 10004
	// CodeRPCUnavailable RPC 服务不可用
	CodeRPCUnavailable = 10010
	// CodeRPCTimeout RPC 超时
	CodeRPCTimeout = 10011
	// CodeRPCDecodeError RPC 解码错误
	CodeRPCDecodeError = 10012
	// CodeRPCEncodeError RPC 编码错误
	CodeRPCEncodeError = 10013
)

// ── go-middleware 预定义错误码 (20000-20699) ──

const (
	// Redis 错误码 20001-20099
	CodeRedisConnect  = 20001
	CodeRedisPing     = 20002
	CodeRedisOp       = 20003
	CodeRedisPipeline = 20004
	CodeRedisSentinel = 20005

	// Kafka 错误码 20101-20199
	CodeKafkaConnect   = 20101
	CodeKafkaSend      = 20102
	CodeKafkaConsume   = 20103
	CodeKafkaCommit    = 20104
	CodeKafkaRebalance = 20105

	// DB 错误码 20201-20299
	CodeDBConnect = 20201
	CodeDBQuery   = 20202
	CodeDBExec    = 20203
	CodeDBMigrate = 20204

	// ES 错误码 20301-20399
	CodeESConnect = 20301
	CodeESQuery   = 20302

	// ClickHouse 错误码 20401-20499
	CodeCHConnect = 20401
	CodeCHQuery   = 20402

	// TLS 错误码 20501-20599
	CodeTLSConnect = 20501
	CodeTLSSend    = 20502

	// Observability 错误码 20601-20699
	CodeObsInit   = 20601
	CodeObsExport = 20602
)

// ── 预定义错误构造器 ──

// ErrSystem 系统错误
var ErrSystem = Code(CodeSystem).Public("system_error")

// ErrParamInvalid 参数无效
var ErrParamInvalid = Code(CodeParamInvalid).Public("param_invalid")

// ErrAuthFailed 鉴权失败
var ErrAuthFailed = Code(CodeAuthFailed).Public("auth_failed")

// ErrConfigNotFound 配置未找到
var ErrConfigNotFound = Code(CodeConfigNotFound).Public("config_not_found")

// ErrConfigInvalid 配置无效
var ErrConfigInvalid = Code(CodeConfigInvalid).Public("config_invalid")

// ErrRPCUnavailable RPC 服务不可用
var ErrRPCUnavailable = Code(CodeRPCUnavailable).Public("rpc_unavailable")

// ErrRPCTimeout RPC 超时
var ErrRPCTimeout = Code(CodeRPCTimeout).Public("rpc_timeout")

// ── 构造函数 ──

// Builder 是 oops.OopsErrorBuilder 的别名，用于链式构造错误。
type Builder = oops.OopsErrorBuilder

// Code 创建带错误码的 Builder。
//
//	err := rpcerror.Code(10001).Public("param_invalid").Wrap(err)
func Code(code any) oops.OopsErrorBuilder {
	return oops.Code(code)
}

// In 创建带域名的 Builder。
//
//	err := rpcerror.In("auth").Code(10002).Public("token_expired").Wrap(err)
func In(domain string) oops.OopsErrorBuilder {
	return oops.In(domain)
}

// ── 提取函数 ──

// Extract 从 error 中提取错误码和公开消息。
// 如果不是 oops 错误，返回 (0, "")。
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

// ExtractWithFallback 从 error 中提取错误码和公开消息。
// 如果不是 oops 错误，返回 fallbackCode 和 err.Error()。
func ExtractWithFallback(err error, fallbackCode int) (code int, public string) {
	if err == nil {
		return 0, ""
	}
	code, public = Extract(err)
	if code == 0 {
		return fallbackCode, err.Error()
	}
	return code, public
}

// AsOopsError 尝试将 error 转换为 oops.OopsError。
// 如果不是 oops 错误，返回 (nil, false)。
func AsOopsError(err error) (oops.OopsError, bool) {
	var oopsErr oops.OopsError
	if errors.As(err, &oopsErr) {
		return oopsErr, true
	}
	return oops.OopsError{}, false
}

// ── Statuser 接口（兼容 Kitex kerrors.BizStatusErrorIface） ──

// Statuser 定义业务状态错误接口（兼容 kitex kerrors.BizStatusErrorIface）。
type Statuser interface {
	BizStatusCode() int32
	BizMessage() string
	BizExtra() map[string]string
	Error() string
}

// OopsStatusAdapter 将 oops 错误适配为 Kitex BizStatusErrorIface 接口。
type OopsStatusAdapter struct {
	Err   error
	Extra map[string]string
}

// BizStatusCode 返回错误码（int32 兼容 Kitex）。
func (a *OopsStatusAdapter) BizStatusCode() int32 {
	code, _ := Extract(a.Err)
	return int32(code)
}

// BizMessage 返回公开消息。
func (a *OopsStatusAdapter) BizMessage() string {
	_, public := Extract(a.Err)
	return public
}

// BizExtra 返回额外信息。
func (a *OopsStatusAdapter) BizExtra() map[string]string {
	return a.Extra
}

// Error 返回错误字符串。
func (a *OopsStatusAdapter) Error() string {
	return a.Err.Error()
}
