// Package errcode 定义 go-tools 错误码体系，所有层（go-common/go-middleware/go-framework）均可引用。
//
// 错误码范围：
//
//	go-framework: 10000-10499  (system, param, auth, config, RPC middleware)
//	go-middleware:  20000-20699 (redis, kafka, db, es, clickhouse, observability)
//	Project custom: 40000-59999 (business modules, external dependencies)
package errcode

// ── 范围常量 ──

const (
	FrameworkCodeMin  = 10000
	FrameworkCodeMax  = 10499
	MiddlewareCodeMin = 20000
	MiddlewareCodeMax = 20699
	ProjectCodeMin    = 40000
	ProjectCodeMax    = 59999
)

// ── go-framework 预定义错误码 (10000-10499) ──

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

// ── 项目自定义业务错误码 (40000-59999) — RPC 调用成功，HTTP 返回 200 ──
// 按百位分区：数据/认证/限制/业务状态

const (
	// 数据相关 40010-40019
	CodeDataNotFound  = 40010 // 数据不存在
	CodeDataDuplicate = 40011 // 数据重复
	CodeDataConflict  = 40012 // 数据冲突（并发修改）

	// 认证/授权 40110-40119
	CodePasswordWrong    = 40110 // 密码错误
	CodeTokenExpired     = 40111 // 凭证过期
	CodeTokenInvalid     = 40112 // 凭证无效
	CodePermissionDenied = 40113 // 无权限

	// 限制/风控 40210-40219
	CodeRateLimit     = 40210 // 频率限制
	CodeQuotaExceeded = 40211 // 配额用尽
	CodeIPBlocked     = 40212 // IP 受限

	// 业务状态 40310-40319
	CodeAccountDisabled     = 40310 // 账户已禁用
	CodeOrderInvalid        = 40311 // 订单状态不符
	CodeBalanceInsufficient = 40312 // 余额不足
	CodeVerificationFailed  = 40313 // 验证码错误
	CodeOperationDenied     = 40314 // 操作被拒绝
)

// ── HTTP 状态码映射 ──

// HTTPStatus 根据错误码返回对应的 HTTP 状态码。
//   - 框架/中间件错误（10000-10499, 20000-20699）→ 5xx
//   - 业务错误（40000+）→ 200（RPC 调用成功，错误信息在响应体中）
func HTTPStatus(code int) int {
	switch code {
	// 4xx — 客户端错误
	case CodeParamInvalid:
		return 400
	case CodeAuthFailed:
		return 401

	// 5xx — 框架/基础设施错误
	case CodeSystem:
		return 500
	case CodeConfigNotFound:
		return 500
	case CodeConfigInvalid:
		return 500
	case CodeRPCDecodeError:
		return 500
	case CodeRPCEncodeError:
		return 500

	// 503 — 依赖服务不可用
	case CodeRPCUnavailable:
		return 503
	case CodeRedisConnect, CodeRedisPing, CodeRedisSentinel:
		return 503
	case CodeKafkaConnect:
		return 503
	case CodeDBConnect:
		return 503
	case CodeESConnect:
		return 503
	case CodeCHConnect:
		return 503
	case CodeTLSConnect:
		return 503
	case CodeObsInit:
		return 503

	// 504 — 超时
	case CodeRPCTimeout:
		return 504

	// 中间件操作错误 → 500
	case CodeRedisOp, CodeRedisPipeline:
		return 500
	case CodeKafkaSend, CodeKafkaConsume, CodeKafkaCommit, CodeKafkaRebalance:
		return 500
	case CodeDBQuery, CodeDBExec, CodeDBMigrate:
		return 500
	case CodeESQuery:
		return 500
	case CodeCHQuery:
		return 500
	case CodeTLSSend:
		return 500
	case CodeObsExport:
		return 500

	// 业务错误（40000+）→ 200，RPC 调用本身成功
	default:
		return 200
	}
}

// IsClientError 判断错误码是否属于客户端错误（4xx）。
func IsClientError(code int) bool {
	s := HTTPStatus(code)
	return s >= 400 && s < 500
}

// IsServerError 判断错误码是否属于服务端/基础设施错误（5xx）。
func IsServerError(code int) bool {
	return HTTPStatus(code) >= 500
}

// IsBusinessError 判断错误码是否属于业务错误（200，RPC 成功）。
func IsBusinessErrorCode(code int) bool {
	return code >= ProjectCodeMin || (code < FrameworkCodeMin && code > 0)
}
