package kafka

import (
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// Kafka 错误码 20201-20206。
const (
	// CodeWrite Kafka 消息写入失败
	CodeWrite = 20201
	// CodeRead Kafka 消息读取失败
	CodeRead = 20202
	// CodeCommit Kafka offset 提交失败
	CodeCommit = 20203
	// CodeDLQForward DLQ 转发失败
	CodeDLQForward = 20204
	// CodeOffsetQuery offset/lag 查询失败
	CodeOffsetQuery = 20205
	// CodeSeek Seek 失败
	CodeSeek = 20206
)

// 预定义 Kafka 错误构造器。
var (
	// ErrWrite Kafka 消息写入失败
	ErrWrite = goerror.Code(CodeWrite).Public("kafka_write_error")
	// ErrRead Kafka 消息读取失败
	ErrRead = goerror.Code(CodeRead).Public("kafka_read_error")
	// ErrCommit Kafka offset 提交失败
	ErrCommit = goerror.Code(CodeCommit).Public("kafka_commit_error")
	// ErrDLQForward DLQ 转发失败
	ErrDLQForward = goerror.Code(CodeDLQForward).Public("kafka_dlq_forward_error")
	// ErrOffsetQuery offset/lag 查询失败
	ErrOffsetQuery = goerror.Code(CodeOffsetQuery).Public("kafka_offset_query_error")
	// ErrSeek Seek 失败
	ErrSeek = goerror.Code(CodeSeek).Public("kafka_seek_error")
)

// init 注册 Kafka 错误码的细粒度 HTTP 状态码映射。
func init() {
	goerror.RegisterHTTPStatuses(map[int]int{
		CodeWrite:       500,
		CodeRead:        500,
		CodeCommit:      500,
		CodeDLQForward:  500,
		CodeOffsetQuery: 500,
		CodeSeek:        500,
	})
}
