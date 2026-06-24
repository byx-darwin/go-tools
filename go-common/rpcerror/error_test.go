package rpcerror

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodeConstants(t *testing.T) {
	assert.Less(t, FrameworkCodeMax, MiddlewareCodeMin)
	assert.Less(t, MiddlewareCodeMax, ProjectCodeMin)
}

func TestCode_Basic(t *testing.T) {
	original := errors.New("original error")
	err := Code(CodeParamInvalid).Public("param_invalid").Wrap(original)

	code, public := Extract(err)
	assert.Equal(t, CodeParamInvalid, code)
	assert.Equal(t, "param_invalid", public)
}

func TestIn_Basic(t *testing.T) {
	original := errors.New("auth failed")
	err := In("auth").Code(CodeAuthFailed).Public("token_expired").Wrap(original)

	code, public := Extract(err)
	assert.Equal(t, CodeAuthFailed, code)
	assert.Equal(t, "token_expired", public)
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		code   int
		public string
	}{
		{"ErrSystem", ErrSystem.Wrap(errors.New("sys")), CodeSystem, "system_error"},
		{"ErrParamInvalid", ErrParamInvalid.Wrap(errors.New("bad")), CodeParamInvalid, "param_invalid"},
		{"ErrAuthFailed", ErrAuthFailed.Wrap(errors.New("no")), CodeAuthFailed, "auth_failed"},
		{"ErrConfigNotFound", ErrConfigNotFound.Wrap(errors.New("miss")), CodeConfigNotFound, "config_not_found"},
		{"ErrRPCUnavailable", ErrRPCUnavailable.Wrap(errors.New("down")), CodeRPCUnavailable, "rpc_unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, public := Extract(tt.err)
			assert.Equal(t, tt.code, code)
			assert.Equal(t, tt.public, public)
		})
	}
}

func TestExtract_NilError(t *testing.T) {
	code, public := Extract(nil)
	assert.Equal(t, 0, code)
	assert.Empty(t, public)
}

func TestExtract_NonOopsError(t *testing.T) {
	err := errors.New("plain error")
	code, public := Extract(err)
	assert.Equal(t, 0, code)
	assert.Empty(t, public)
}

func TestExtractWithFallback_NonOops(t *testing.T) {
	err := errors.New("plain error")
	code, public := ExtractWithFallback(err, 99999)
	assert.Equal(t, 99999, code)
	assert.Equal(t, "plain error", public)
}

func TestExtractWithFallback_OopsError(t *testing.T) {
	err := Code(12345).Public("custom").Wrap(errors.New("inner"))
	code, public := ExtractWithFallback(err, 99999)
	assert.Equal(t, 12345, code)
	assert.Equal(t, "custom", public)
}

func TestAsOopsError(t *testing.T) {
	err := Code(10001).Public("test").Wrap(errors.New("inner"))

	oopsErr, ok := AsOopsError(err)
	assert.True(t, ok)
	assert.Equal(t, 10001, oopsErr.Code())
}

func TestAsOopsError_NonOops(t *testing.T) {
	err := errors.New("plain")

	_, ok := AsOopsError(err)
	assert.False(t, ok)
}

func TestHTTPStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"param invalid", ErrParamInvalid.Wrap(errors.New("bad")), 400},
		{"auth failed", ErrAuthFailed.Wrap(errors.New("no")), 401},
		{"system", ErrSystem.Wrap(errors.New("boom")), 500},
		{"config not found", ErrConfigNotFound.Wrap(errors.New("miss")), 500},
		{"rpc unavailable", ErrRPCUnavailable.Wrap(errors.New("down")), 503},
		{"rpc timeout", ErrRPCTimeout.Wrap(errors.New("slow")), 504},
		{"redis connect", ErrRedisConnect.Wrap(errors.New("redis down")), 503},
		{"kafka connect", ErrKafkaConnect.Wrap(errors.New("kafka down")), 503},
		{"db connect", ErrDBConnect.Wrap(errors.New("db down")), 503},
		{"redis op", ErrRedisOp.Wrap(errors.New("fail")), 500},
		{"kafka send", ErrKafkaSend.Wrap(errors.New("fail")), 500},
		{"db query", ErrDBQuery.Wrap(errors.New("fail")), 500},
		{"custom business", Code(40001).Public("data_duplicate").Wrap(errors.New("dup")), 200},
		{"plain error", errors.New("plain"), 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HTTPStatus(tt.err))
		})
	}
}
