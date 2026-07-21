package clickhouse

import (
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// ClickHouse 错误码 20401-20403。
const (
	// CodeConnect ClickHouse 连接失败
	CodeConnect = 20401
	// CodeQuery ClickHouse 查询失败
	CodeQuery = 20402
	// CodeParseDSN ClickHouse DSN 解析失败
	CodeParseDSN = 20403
)

// 预定义 ClickHouse 错误构造器。
var (
	// ErrConnect ClickHouse 连接失败
	ErrConnect = goerror.Code(CodeConnect).Public("ch_connect_error")
	// ErrQuery ClickHouse 查询失败
	ErrQuery = goerror.Code(CodeQuery).Public("ch_query_error")
	// ErrParseDSN ClickHouse DSN 解析失败
	ErrParseDSN = goerror.Code(CodeParseDSN).Public("ch_parse_dsn_error")
)

// init 注册 ClickHouse 错误码的细粒度 HTTP 状态码映射。
// 映射与原 go-common/error httpStatusByCode 逐值一致。
func init() {
	goerror.RegisterHTTPStatuses(map[int]int{
		CodeConnect:  503,
		CodeQuery:    500,
		CodeParseDSN: 503,
	})
}
