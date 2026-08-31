package es

import (
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// Elasticsearch 错误码 20701。
const (
	// CodeInit Elasticsearch 客户端初始化失败
	CodeInit = 20701
)

// 预定义 Elasticsearch 错误构造器。
var (
	// ErrInit Elasticsearch 客户端初始化失败
	ErrInit = goerror.Code(CodeInit).Public("es_init_error")
)

// init 注册 Elasticsearch 错误码的细粒度 HTTP 状态码映射。
func init() {
	goerror.RegisterHTTPStatuses(map[int]int{
		CodeInit: 503,
	})
}
