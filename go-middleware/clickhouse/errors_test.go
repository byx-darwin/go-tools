package clickhouse_test

import (
	"errors"
	"testing"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/byx-darwin/go-tools/go-middleware/clickhouse"
	"github.com/stretchr/testify/assert"
)

// TestCodeValues 码值是 wire 契约，逐值锁定。
func TestCodeValues(t *testing.T) {
	assert.Equal(t, 20401, clickhouse.CodeConnect)
	assert.Equal(t, 20402, clickhouse.CodeQuery)
	assert.Equal(t, 20403, clickhouse.CodeParseDSN)
}

// TestPredefinedErrors 构造器 code + public 消息与原 go-common 定义一致。
func TestPredefinedErrors(t *testing.T) {
	code, public := goerror.Extract(clickhouse.ErrParseDSN.Wrap(errors.New("x")))
	assert.Equal(t, 20403, code)
	assert.Equal(t, "ch_parse_dsn_error", public)

	code, public = goerror.Extract(clickhouse.ErrConnect.Wrap(errors.New("x")))
	assert.Equal(t, 20401, code)
	assert.Equal(t, "ch_connect_error", public)

	code, public = goerror.Extract(clickhouse.ErrQuery.Wrap(errors.New("x")))
	assert.Equal(t, 20402, code)
	assert.Equal(t, "ch_query_error", public)
}

// TestHTTPStatusRegistration init() 注册映射与原 httpStatusByCode 一致。
func TestHTTPStatusRegistration(t *testing.T) {
	assert.Equal(t, 503, goerror.HTTPStatus(clickhouse.ErrConnect.Wrap(errors.New("x"))))
	assert.Equal(t, 500, goerror.HTTPStatus(clickhouse.ErrQuery.Wrap(errors.New("x"))))
	assert.Equal(t, 503, goerror.HTTPStatus(clickhouse.ErrParseDSN.Wrap(errors.New("x"))))
}
