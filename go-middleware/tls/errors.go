package tls

import (
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// TLS 错误码 20501-20504。
const (
	// CodeConnect TLS 连接失败
	CodeConnect = 20501
	// CodeSend TLS 发送失败
	CodeSend = 20502
	// CodeInvalidConfig TLS 配置无效
	CodeInvalidConfig = 20503
	// CodeProducerInit TLS Producer 初始化失败
	CodeProducerInit = 20504
)

// 预定义 TLS 错误构造器。
var (
	// ErrConnect TLS 连接失败
	ErrConnect = goerror.Code(CodeConnect).Public("tls_connect_error")
	// ErrSend TLS 发送失败
	ErrSend = goerror.Code(CodeSend).Public("tls_send_error")
	// ErrInvalidConfig TLS 配置无效
	ErrInvalidConfig = goerror.Code(CodeInvalidConfig).Public("tls_invalid_config_error")
	// ErrProducerInit TLS Producer 初始化失败
	ErrProducerInit = goerror.Code(CodeProducerInit).Public("tls_producer_init_error")
)

// init 注册 TLS 错误码的细粒度 HTTP 状态码映射。
// 映射与原 go-common/error httpStatusByCode 逐值一致。
func init() {
	goerror.RegisterHTTPStatuses(map[int]int{
		CodeConnect:       503,
		CodeSend:          500,
		CodeInvalidConfig: 503,
		CodeProducerInit:  503,
	})
}
