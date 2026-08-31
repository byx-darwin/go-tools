package redis

import (
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// Redis 错误码 20101-20102。
const (
	// CodeConnect Redis 连接/Ping 失败
	CodeConnect = 20101
	// CodeInstrument Redis OpenTelemetry 埋点初始化失败
	CodeInstrument = 20102
)

// 预定义 Redis 错误构造器。
var (
	// ErrConnect Redis 连接/Ping 失败
	ErrConnect = goerror.Code(CodeConnect).Public("redis_connect_error")
	// ErrInstrument Redis OpenTelemetry 埋点初始化失败
	ErrInstrument = goerror.Code(CodeInstrument).Public("redis_instrument_error")
)

// init 注册 Redis 错误码的细粒度 HTTP 状态码映射。
func init() {
	goerror.RegisterHTTPStatuses(map[int]int{
		CodeConnect:    503,
		CodeInstrument: 500,
	})
}
