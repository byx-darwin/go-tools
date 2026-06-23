// Package middleware 定义 go-middleware 库的统一错误码。
package middleware

// 错误码范围：20000-20699
const (
	// Redis 错误码 20000-20099
	ErrCodeRedisConnect    = 20001
	ErrCodeRedisPing       = 20002
	ErrCodeRedisOp         = 20003
	ErrCodeRedisPipeline   = 20004
	ErrCodeRedisSentinel   = 20005

	// Kafka 错误码 20100-20199
	ErrCodeKafkaConnect    = 20101
	ErrCodeKafkaSend       = 20102
	ErrCodeKafkaConsume    = 20103
	ErrCodeKafkaCommit     = 20104
	ErrCodeKafkaRebalance  = 20105

	// DB 错误码 20200-20299
	ErrCodeDBConnect       = 20201
	ErrCodeDBQuery         = 20202
	ErrCodeDBExec          = 20203
	ErrCodeDBMigrate       = 20204

	// ES 错误码 20300-20399（预留）
	ErrCodeESConnect       = 20301
	ErrCodeESQuery         = 20302

	// ClickHouse 错误码 20400-20499（预留）
	ErrCodeCHConnect       = 20401
	ErrCodeCHQuery         = 20402

	// TLS 错误码 20500-20599（预留）
	ErrCodeTLSConnect      = 20501
	ErrCodeTLSSend         = 20502

	// Observability 错误码 20600-20699（预留）
	ErrCodeObsInit         = 20601
	ErrCodeObsExport       = 20602
)
