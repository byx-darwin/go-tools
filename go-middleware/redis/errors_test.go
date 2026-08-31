package redis_test

import (
	"errors"
	"testing"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/byx-darwin/go-tools/go-middleware/redis"
	"github.com/stretchr/testify/assert"
)

// TestCodeValues 码值是 wire 契约，逐值锁定。
func TestCodeValues(t *testing.T) {
	assert.Equal(t, 20101, redis.CodeConnect)
	assert.Equal(t, 20102, redis.CodeInstrument)
}

// TestPredefinedErrors 构造器 code + public 消息符合预期。
func TestPredefinedErrors(t *testing.T) {
	code, public := goerror.Extract(redis.ErrConnect.Wrap(errors.New("x")))
	assert.Equal(t, 20101, code)
	assert.Equal(t, "redis_connect_error", public)

	code, public = goerror.Extract(redis.ErrInstrument.Wrap(errors.New("x")))
	assert.Equal(t, 20102, code)
	assert.Equal(t, "redis_instrument_error", public)
}

// TestHTTPStatusRegistration init() 注册的 HTTP 状态映射。
func TestHTTPStatusRegistration(t *testing.T) {
	assert.Equal(t, 503, goerror.HTTPStatus(redis.ErrConnect.Wrap(errors.New("x"))))
	assert.Equal(t, 500, goerror.HTTPStatus(redis.ErrInstrument.Wrap(errors.New("x"))))
}
