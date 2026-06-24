// Package rpcerror 提供基于 oops 的错误处理框架。
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

	"github.com/byx-darwin/go-tools/go-common/errcode"
	"github.com/samber/oops"
)

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

	// 业务错误
	CodeDataNotFound        = errcode.CodeDataNotFound
	CodeDataDuplicate       = errcode.CodeDataDuplicate
	CodeDataConflict        = errcode.CodeDataConflict
	CodeLoginFailed         = errcode.CodeLoginFailed
	CodeTokenExpired        = errcode.CodeTokenExpired
	CodeTokenInvalid        = errcode.CodeTokenInvalid
	CodePermissionDenied    = errcode.CodePermissionDenied
	CodeRateLimit           = errcode.CodeRateLimit
	CodeQuotaExceeded       = errcode.CodeQuotaExceeded
	CodeIPBlocked           = errcode.CodeIPBlocked
	CodeAccountDisabled     = errcode.CodeAccountDisabled
	CodeOrderInvalid        = errcode.CodeOrderInvalid
	CodeBalanceInsufficient = errcode.CodeBalanceInsufficient
	CodeVerificationFailed  = errcode.CodeVerificationFailed
	CodeOperationDenied     = errcode.CodeOperationDenied
)

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

	// 业务错误
	ErrDataNotFound        = Code(CodeDataNotFound).Public("data_not_found")
	ErrDataDuplicate       = Code(CodeDataDuplicate).Public("data_duplicate")
	ErrDataConflict        = Code(CodeDataConflict).Public("data_conflict")
	ErrLoginFailed         = Code(CodeLoginFailed).Public("login_failed")
	ErrTokenExpired        = Code(CodeTokenExpired).Public("token_expired")
	ErrTokenInvalid        = Code(CodeTokenInvalid).Public("token_invalid")
	ErrPermissionDenied    = Code(CodePermissionDenied).Public("permission_denied")
	ErrRateLimit           = Code(CodeRateLimit).Public("rate_limit")
	ErrQuotaExceeded       = Code(CodeQuotaExceeded).Public("quota_exceeded")
	ErrIPBlocked           = Code(CodeIPBlocked).Public("ip_blocked")
	ErrAccountDisabled     = Code(CodeAccountDisabled).Public("account_disabled")
	ErrOrderInvalid        = Code(CodeOrderInvalid).Public("order_invalid")
	ErrBalanceInsufficient = Code(CodeBalanceInsufficient).Public("balance_insufficient")
	ErrVerificationFailed  = Code(CodeVerificationFailed).Public("verification_failed")
	ErrOperationDenied     = Code(CodeOperationDenied).Public("operation_denied")
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
// 详细映射见 go-common/errcode.HTTPStatus。
func HTTPStatus(err error) int {
	code, _ := Extract(err)
	return errcode.HTTPStatus(code)
}
