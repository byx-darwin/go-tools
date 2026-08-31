package db

import (
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// db 错误码 20301-20302。
const (
	// CodeOpen 数据库驱动打开失败（无效 driver/DSN）
	CodeOpen = 20301
	// CodeConnect 数据库连接不可达（Ping 失败）
	CodeConnect = 20302
)

// 预定义 db 错误构造器。
var (
	// ErrOpen 数据库驱动打开失败
	ErrOpen = goerror.Code(CodeOpen).Public("db_open_error")
	// ErrConnect 数据库连接不可达
	ErrConnect = goerror.Code(CodeConnect).Public("db_connect_error")
)

// init 注册 db 错误码的细粒度 HTTP 状态码映射。
func init() {
	goerror.RegisterHTTPStatuses(map[int]int{
		CodeOpen:    500,
		CodeConnect: 503,
	})
}
