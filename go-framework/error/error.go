// Package frameworkerror 提供 go-framework 模块的错误码和预定义错误构造器。
//
// 错误码范围：
//   - 10000-10013: system/param/auth/config/Polaris/RPC
//   - 20601-20605: observability（obs 段由 framework 适配层 hertz/kitex 使用；
//     码值为 wire 契约，禁止改号）
//
// 细粒度 HTTP 状态码映射通过 init() 注册到 go-common/error 注册表。
package frameworkerror

import (
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// Builder 是错误构建器类型别名。
type Builder = goerror.Builder

// 框架错误码 10000-10013。
const (
	// CodeSystem 系统内部错误（兜底）
	CodeSystem = 10000
	// CodeParamInvalid 参数无效
	CodeParamInvalid = 10001
	// CodeAuthFailed 鉴权失败
	CodeAuthFailed = 10002
	// CodeConfigNotFound 配置未找到
	CodeConfigNotFound = 10003
	// CodeConfigInvalid 配置无效
	CodeConfigInvalid = 10004
	// CodePolarisInit Polaris 初始化失败
	CodePolarisInit = 10005
	// CodePolarisGetConfig Polaris 获取配置文件失败
	CodePolarisGetConfig = 10006
	// CodeRPCUnavailable RPC 服务不可用
	CodeRPCUnavailable = 10010
	// CodeRPCTimeout RPC 超时
	CodeRPCTimeout = 10011
	// CodeRPCDecodeError RPC 解码错误
	CodeRPCDecodeError = 10012
	// CodeRPCEncodeError RPC 编码错误
	CodeRPCEncodeError = 10013
)

// Observability 错误码 20601-20605（obs 段，由 framework 适配层使用）。
const (
	// CodeObsInit Observability 初始化失败
	CodeObsInit = 20601
	// CodeObsExport Observability 导出失败
	CodeObsExport = 20602
	// CodeObsTraceExport Trace exporter 创建失败
	CodeObsTraceExport = 20603
	// CodeObsMetricExport Metric exporter 创建失败
	CodeObsMetricExport = 20604
	// CodeObsRuntimeMetrics Runtime metrics 启动失败
	CodeObsRuntimeMetrics = 20605
)

// 预定义框架错误构造器。
var (
	// ErrSystem 系统内部错误（兜底）
	ErrSystem = goerror.Code(CodeSystem).Public("system_error")
	// ErrParamInvalid 参数无效
	ErrParamInvalid = goerror.Code(CodeParamInvalid).Public("param_invalid")
	// ErrAuthFailed 鉴权失败
	ErrAuthFailed = goerror.Code(CodeAuthFailed).Public("auth_failed")
	// ErrConfigNotFound 配置未找到
	ErrConfigNotFound = goerror.Code(CodeConfigNotFound).Public("config_not_found")
	// ErrConfigInvalid 配置无效
	ErrConfigInvalid = goerror.Code(CodeConfigInvalid).Public("config_invalid")
	// ErrPolarisInit Polaris 初始化失败
	ErrPolarisInit = goerror.Code(CodePolarisInit).Public("polaris_init_error")
	// ErrPolarisGetConfig Polaris 获取配置文件失败
	ErrPolarisGetConfig = goerror.Code(CodePolarisGetConfig).Public("polaris_get_config_error")
	// ErrRPCUnavailable RPC 服务不可用
	ErrRPCUnavailable = goerror.Code(CodeRPCUnavailable).Public("rpc_unavailable")
	// ErrRPCTimeout RPC 超时
	ErrRPCTimeout = goerror.Code(CodeRPCTimeout).Public("rpc_timeout")
	// ErrRPCDecodeError RPC 解码错误
	ErrRPCDecodeError = goerror.Code(CodeRPCDecodeError).Public("rpc_decode_error")
	// ErrRPCEncodeError RPC 编码错误
	ErrRPCEncodeError = goerror.Code(CodeRPCEncodeError).Public("rpc_encode_error")
)

// 预定义 Observability 错误构造器。
var (
	// ErrObsInit Observability 初始化失败
	ErrObsInit = goerror.Code(CodeObsInit).Public("observability_init_error")
	// ErrObsExport Observability 导出失败
	ErrObsExport = goerror.Code(CodeObsExport).Public("observability_export_error")
	// ErrObsTraceExport Trace exporter 创建失败
	ErrObsTraceExport = goerror.Code(CodeObsTraceExport).Public("observability_trace_export_error")
	// ErrObsMetricExport Metric exporter 创建失败
	ErrObsMetricExport = goerror.Code(CodeObsMetricExport).Public("observability_metric_export_error")
	// ErrObsRuntimeMetrics Runtime metrics 启动失败
	ErrObsRuntimeMetrics = goerror.Code(CodeObsRuntimeMetrics).Public("observability_runtime_metrics_error")
)

// init 注册框架错误码的细粒度 HTTP 状态码映射。
// 映射与原 go-common/error httpStatusByCode 逐值一致。
func init() {
	goerror.RegisterHTTPStatuses(map[int]int{
		CodeParamInvalid:      400,
		CodeAuthFailed:        401,
		CodeSystem:            500,
		CodeConfigNotFound:    500,
		CodeConfigInvalid:     500,
		CodeRPCDecodeError:    500,
		CodeRPCEncodeError:    500,
		CodeObsExport:         500,
		CodeRPCUnavailable:    503,
		CodePolarisInit:       503,
		CodePolarisGetConfig:  503,
		CodeObsInit:           503,
		CodeObsTraceExport:    503,
		CodeObsMetricExport:   503,
		CodeObsRuntimeMetrics: 503,
		CodeRPCTimeout:        504,
	})
}
