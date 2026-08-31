package db_test

import (
	"errors"
	"testing"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/byx-darwin/go-tools/go-middleware/db"
	"github.com/stretchr/testify/assert"
)

// TestCodeValues 码值是 wire 契约，逐值锁定。
func TestCodeValues(t *testing.T) {
	assert.Equal(t, 20301, db.CodeOpen)
	assert.Equal(t, 20302, db.CodeConnect)
}

// TestPredefinedErrors 构造器 code + public 消息符合预期。
func TestPredefinedErrors(t *testing.T) {
	code, public := goerror.Extract(db.ErrOpen.Wrap(errors.New("x")))
	assert.Equal(t, 20301, code)
	assert.Equal(t, "db_open_error", public)

	code, public = goerror.Extract(db.ErrConnect.Wrap(errors.New("x")))
	assert.Equal(t, 20302, code)
	assert.Equal(t, "db_connect_error", public)
}

// TestHTTPStatusRegistration init() 注册的 HTTP 状态映射。
func TestHTTPStatusRegistration(t *testing.T) {
	assert.Equal(t, 500, goerror.HTTPStatus(db.ErrOpen.Wrap(errors.New("x"))))
	assert.Equal(t, 503, goerror.HTTPStatus(db.ErrConnect.Wrap(errors.New("x"))))
}
