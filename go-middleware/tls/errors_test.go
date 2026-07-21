package tls_test

import (
	"errors"
	"testing"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	gotls "github.com/byx-darwin/go-tools/go-middleware/tls"
	"github.com/stretchr/testify/assert"
)

// TestCodeValues 码值是 wire 契约，逐值锁定。
func TestCodeValues(t *testing.T) {
	assert.Equal(t, 20501, gotls.CodeConnect)
	assert.Equal(t, 20502, gotls.CodeSend)
	assert.Equal(t, 20503, gotls.CodeInvalidConfig)
	assert.Equal(t, 20504, gotls.CodeProducerInit)
}

// TestPredefinedErrors 构造器 code + public 消息与原 go-common 定义一致。
func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		code   int
		public string
	}{
		{"ErrConnect", gotls.ErrConnect.Wrap(errors.New("x")), 20501, "tls_connect_error"},
		{"ErrSend", gotls.ErrSend.Wrap(errors.New("x")), 20502, "tls_send_error"},
		{"ErrInvalidConfig", gotls.ErrInvalidConfig.Wrap(errors.New("x")), 20503, "tls_invalid_config_error"},
		{"ErrProducerInit", gotls.ErrProducerInit.Wrap(errors.New("x")), 20504, "tls_producer_init_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, public := goerror.Extract(tt.err)
			assert.Equal(t, tt.code, code)
			assert.Equal(t, tt.public, public)
		})
	}
}

// TestHTTPStatusRegistration init() 注册映射与原 httpStatusByCode 一致。
func TestHTTPStatusRegistration(t *testing.T) {
	assert.Equal(t, 503, goerror.HTTPStatus(gotls.ErrConnect.Wrap(errors.New("x"))))
	assert.Equal(t, 500, goerror.HTTPStatus(gotls.ErrSend.Wrap(errors.New("x"))))
	assert.Equal(t, 503, goerror.HTTPStatus(gotls.ErrInvalidConfig.Wrap(errors.New("x"))))
	assert.Equal(t, 503, goerror.HTTPStatus(gotls.ErrProducerInit.Wrap(errors.New("x"))))
}
