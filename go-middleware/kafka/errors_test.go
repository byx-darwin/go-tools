package kafka_test

import (
	"errors"
	"testing"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/byx-darwin/go-tools/go-middleware/kafka"
	"github.com/stretchr/testify/assert"
)

// TestCodeValues 码值是 wire 契约，逐值锁定。
func TestCodeValues(t *testing.T) {
	assert.Equal(t, 20201, kafka.CodeWrite)
	assert.Equal(t, 20202, kafka.CodeRead)
	assert.Equal(t, 20203, kafka.CodeCommit)
	assert.Equal(t, 20204, kafka.CodeDLQForward)
	assert.Equal(t, 20205, kafka.CodeOffsetQuery)
	assert.Equal(t, 20206, kafka.CodeSeek)
}

// TestPredefinedErrors 构造器 code + public 消息符合预期。
func TestPredefinedErrors(t *testing.T) {
	code, public := goerror.Extract(kafka.ErrWrite.Wrap(errors.New("x")))
	assert.Equal(t, 20201, code)
	assert.Equal(t, "kafka_write_error", public)

	code, public = goerror.Extract(kafka.ErrRead.Wrap(errors.New("x")))
	assert.Equal(t, 20202, code)
	assert.Equal(t, "kafka_read_error", public)

	code, public = goerror.Extract(kafka.ErrCommit.Wrap(errors.New("x")))
	assert.Equal(t, 20203, code)
	assert.Equal(t, "kafka_commit_error", public)

	code, public = goerror.Extract(kafka.ErrDLQForward.Wrap(errors.New("x")))
	assert.Equal(t, 20204, code)
	assert.Equal(t, "kafka_dlq_forward_error", public)

	code, public = goerror.Extract(kafka.ErrOffsetQuery.Wrap(errors.New("x")))
	assert.Equal(t, 20205, code)
	assert.Equal(t, "kafka_offset_query_error", public)

	code, public = goerror.Extract(kafka.ErrSeek.Wrap(errors.New("x")))
	assert.Equal(t, 20206, code)
	assert.Equal(t, "kafka_seek_error", public)
}

// TestHTTPStatusRegistration init() 注册的 HTTP 状态映射。
func TestHTTPStatusRegistration(t *testing.T) {
	assert.Equal(t, 500, goerror.HTTPStatus(kafka.ErrWrite.Wrap(errors.New("x"))))
	assert.Equal(t, 500, goerror.HTTPStatus(kafka.ErrRead.Wrap(errors.New("x"))))
	assert.Equal(t, 500, goerror.HTTPStatus(kafka.ErrCommit.Wrap(errors.New("x"))))
	assert.Equal(t, 500, goerror.HTTPStatus(kafka.ErrDLQForward.Wrap(errors.New("x"))))
	assert.Equal(t, 500, goerror.HTTPStatus(kafka.ErrOffsetQuery.Wrap(errors.New("x"))))
	assert.Equal(t, 500, goerror.HTTPStatus(kafka.ErrSeek.Wrap(errors.New("x"))))
}
