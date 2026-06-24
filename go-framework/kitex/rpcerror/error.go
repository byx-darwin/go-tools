// Package rpcerror 提供基于 oops 的 RPC 错误处理。
//
// 错误码定义在 go-common/errcode，各层均可引用：
//
//	go-framework: 10000-10499  (system, param, auth, config, RPC middleware)
//	go-middleware:  20000-20699 (redis, kafka, db, es, clickhouse, observability)
//	Project custom: 40000-59999 (business modules, external dependencies)
//
// 用法：
//
//	// 创建 oops 错误
//	err := rpcerror.ErrParamInvalid.Wrap(originalErr)
//
//	// 自定义错误码
//	err := rpcerror.Code(40001).Public("data_duplicate").Wrap(err)
//
//	// 提取 + HTTP 状态码映射
//	code, msg := rpcerror.Extract(err)
//	httpStatus := rpcerror.HTTPStatus(err)
package rpcerror

import (
	"errors"

	"gitee.com/byx_darwin/go-tools/go-common/errcode"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/samber/oops"
)

// 编译期接口断言。
var _ kerrors.BizStatusErrorIface = (*OopsStatusAdapter)(nil)

// ── 错误码常量（从 go-common/errcode 重导出，保持向下兼容） ──

const (
	FrameworkCodeMin  = errcode.FrameworkCodeMin
	FrameworkCodeMax  = errcode.FrameworkCodeMax
	MiddlewareCodeMin = errcode.MiddlewareCodeMin
	MiddlewareCodeMax = errcode.MiddlewareCodeMax
	ProjectCodeMin    = errcode.ProjectCodeMin
	ProjectCodeMax    = errcode.ProjectCodeMax

	// go-framework
	CodeSystem         = errcode.CodeSystem
	CodeParamInvalid   = errcode.CodeParamInvalid
	CodeAuthFailed     = errcode.CodeAuthFailed
	CodeConfigNotFound = errcode.CodeConfigNotFound
	CodeConfigInvalid  = errcode.CodeConfigInvalid
	CodeRPCUnavailable = errcode.CodeRPCUnavailable
	CodeRPCTimeout     = errcode.CodeRPCTimeout
	CodeRPCDecodeError = errcode.CodeRPCDecodeError
	CodeRPCEncodeError = errcode.CodeRPCEncodeError

	// go-middleware
	CodeRedisConnect   = errcode.CodeRedisConnect
	CodeRedisPing      = errcode.CodeRedisPing
	CodeRedisOp        = errcode.CodeRedisOp
	CodeRedisPipeline  = errcode.CodeRedisPipeline
	CodeRedisSentinel  = errcode.CodeRedisSentinel
	CodeKafkaConnect   = errcode.CodeKafkaConnect
	CodeKafkaSend      = errcode.CodeKafkaSend
	CodeKafkaConsume   = errcode.CodeKafkaConsume
	CodeKafkaCommit    = errcode.CodeKafkaCommit
	CodeKafkaRebalance = errcode.CodeKafkaRebalance
	CodeDBConnect      = errcode.CodeDBConnect
	CodeDBQuery        = errcode.CodeDBQuery
	CodeDBExec         = errcode.CodeDBExec
	CodeDBMigrate      = errcode.CodeDBMigrate
	CodeESConnect      = errcode.CodeESConnect
	CodeESQuery        = errcode.CodeESQuery
	CodeCHConnect      = errcode.CodeCHConnect
	CodeCHQuery        = errcode.CodeCHQuery
	CodeTLSConnect     = errcode.CodeTLSConnect
	CodeTLSSend        = errcode.CodeTLSSend
	CodeObsInit        = errcode.CodeObsInit
	CodeObsExport      = errcode.CodeObsExport
)

// ── 错误分类 ──

// ErrorCategory 错误类别
type ErrorCategory int

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

func IsFrameworkError(err error) bool { return Classify(err) == CategoryFramework }
func IsBusinessError(err error) bool  { return Classify(err) == CategoryBusiness }

func IsTimeout(err error) bool {
	if kerrors.IsTimeoutError(err) {
		return true
	}
	code, _ := Extract(err)
	return code == CodeRPCTimeout
}

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

// ── HTTP 状态码映射 ──

// HTTPStatus 从 error 中提取错误码，映射为 HTTP 状态码。
//   - 框架/中间件错误 → 500/503/504
//   - 业务错误（40000+）→ 500 兜底
//   - 非 oops 错误 → 500
//
// 详细映射见 go-common/errcode.HTTPStatus。
func HTTPStatus(err error) int {
	code, _ := Extract(err)
	return errcode.HTTPStatus(code)
}

// ── 预定义错误构造器 ──

var (
	ErrSystem         = Code(CodeSystem).Public("system_error")
	ErrParamInvalid   = Code(CodeParamInvalid).Public("param_invalid")
	ErrAuthFailed     = Code(CodeAuthFailed).Public("auth_failed")
	ErrConfigNotFound = Code(CodeConfigNotFound).Public("config_not_found")
	ErrConfigInvalid  = Code(CodeConfigInvalid).Public("config_invalid")
	ErrRPCUnavailable = Code(CodeRPCUnavailable).Public("rpc_unavailable")
	ErrRPCTimeout     = Code(CodeRPCTimeout).Public("rpc_timeout")
	ErrRPCDecodeError = Code(CodeRPCDecodeError).Public("rpc_decode_error")
	ErrRPCEncodeError = Code(CodeRPCEncodeError).Public("rpc_encode_error")

	// Redis
	ErrRedisConnect  = Code(CodeRedisConnect).Public("redis_connect_error")
	ErrRedisPing     = Code(CodeRedisPing).Public("redis_ping_error")
	ErrRedisOp       = Code(CodeRedisOp).Public("redis_operation_error")
	ErrRedisPipeline = Code(CodeRedisPipeline).Public("redis_pipeline_error")
	ErrRedisSentinel = Code(CodeRedisSentinel).Public("redis_sentinel_error")

	// Kafka
	ErrKafkaConnect   = Code(CodeKafkaConnect).Public("kafka_connect_error")
	ErrKafkaSend      = Code(CodeKafkaSend).Public("kafka_send_error")
	ErrKafkaConsume   = Code(CodeKafkaConsume).Public("kafka_consume_error")
	ErrKafkaCommit    = Code(CodeKafkaCommit).Public("kafka_commit_error")
	ErrKafkaRebalance = Code(CodeKafkaRebalance).Public("kafka_rebalance_error")

	// DB
	ErrDBConnect = Code(CodeDBConnect).Public("db_connect_error")
	ErrDBQuery   = Code(CodeDBQuery).Public("db_query_error")
	ErrDBExec    = Code(CodeDBExec).Public("db_exec_error")
	ErrDBMigrate = Code(CodeDBMigrate).Public("db_migrate_error")

	// ES
	ErrESConnect = Code(CodeESConnect).Public("es_connect_error")
	ErrESQuery   = Code(CodeESQuery).Public("es_query_error")

	// ClickHouse
	ErrCHConnect = Code(CodeCHConnect).Public("ch_connect_error")
	ErrCHQuery   = Code(CodeCHQuery).Public("ch_query_error")

	// TLS
	ErrTLSConnect = Code(CodeTLSConnect).Public("tls_connect_error")
	ErrTLSSend    = Code(CodeTLSSend).Public("tls_send_error")

	// Observability
	ErrObsInit   = Code(CodeObsInit).Public("observability_init_error")
	ErrObsExport = Code(CodeObsExport).Public("observability_export_error")
)

// ── 构造函数 ──

type Builder = oops.OopsErrorBuilder

func Code(code any) oops.OopsErrorBuilder    { return oops.Code(code) }
func In(domain string) oops.OopsErrorBuilder { return oops.In(domain) }

// ── 提取函数 ──

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

func AsOopsError(err error) (oops.OopsError, bool) {
	var oopsErr oops.OopsError
	if errors.As(err, &oopsErr) {
		return oopsErr, true
	}
	return oops.OopsError{}, false
}

// ── Kitex 适配 ──

type Statuser interface {
	BizStatusCode() int32
	BizMessage() string
	BizExtra() map[string]string
	Error() string
}

type OopsStatusAdapter struct {
	Err   error
	Extra map[string]string
}

func (a *OopsStatusAdapter) BizStatusCode() int32        { code, _ := Extract(a.Err); return int32(code) }
func (a *OopsStatusAdapter) BizMessage() string          { _, public := Extract(a.Err); return public }
func (a *OopsStatusAdapter) BizExtra() map[string]string { return a.Extra }
func (a *OopsStatusAdapter) Error() string               { return a.Err.Error() }
