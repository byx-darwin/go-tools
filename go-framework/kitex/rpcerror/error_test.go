package rpcerror

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodeConstants(t *testing.T) {
	// 验证错误码范围不重叠
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

func TestOopsStatusAdapter(t *testing.T) {
	err := Code(CodeParamInvalid).Public("bad_param").Wrap(errors.New("detail"))
	extra := map[string]string{"key": "value"}
	adapter := &OopsStatusAdapter{Err: err, Extra: extra}

	assert.Equal(t, int32(CodeParamInvalid), adapter.BizStatusCode())
	assert.Equal(t, "bad_param", adapter.BizMessage())
	assert.Equal(t, extra, adapter.BizExtra())
	assert.Contains(t, adapter.Error(), "detail")
}

func TestClassify(t *testing.T) {
	// 业务错误
	bizErr := Code(10001).Public("test").Wrap(errors.New("cause"))
	assert.Equal(t, CategoryBusiness, Classify(bizErr))

	// Kitex 框架错误
	frameworkErr := ErrRPCUnavailable
	kitexErr := frameworkErr.Wrap(errors.New("down"))
	// oops 包装的 → 被识别为业务错误（因为 errors.As 先匹配 oops）
	assert.Equal(t, CategoryBusiness, Classify(kitexErr))

	// nil error
	assert.Equal(t, CategoryUnknown, Classify(nil))

	// 普通 error
	assert.Equal(t, CategoryUnknown, Classify(errors.New("plain")))
}

func TestIsBusinessError(t *testing.T) {
	assert.True(t, IsBusinessError(Code(1).Public("x").Wrap(errors.New("y"))))
	assert.False(t, IsBusinessError(errors.New("plain")))
	assert.False(t, IsBusinessError(nil))
}

func TestIsFrameworkError(t *testing.T) {
	frameworkErr := ErrRPCUnavailable.Wrap(errors.New("down"))
	// 这是 oops 错误，不是框架原生错误
	assert.False(t, IsFrameworkError(frameworkErr))
	assert.False(t, IsFrameworkError(errors.New("plain")))
	assert.False(t, IsFrameworkError(nil))
}

func TestIsTimeout(t *testing.T) {
	// 业务超时错误码
	bizTimeout := Code(CodeRPCTimeout).Public("rpc_timeout").Wrap(errors.New("too slow"))
	assert.True(t, IsTimeout(bizTimeout))

	// 非超时业务错误
	bizOther := Code(CodeParamInvalid).Public("bad").Wrap(errors.New("x"))
	assert.False(t, IsTimeout(bizOther))

	// nil error
	assert.False(t, IsTimeout(nil))
}

func TestFrameworkErrorName(t *testing.T) {
	// 业务错误 → 空
	assert.Empty(t, FrameworkErrorName(Code(1).Public("x").Wrap(errors.New("y"))))

	// 普通错误 → 空
	assert.Empty(t, FrameworkErrorName(errors.New("plain")))

	// nil → 空
	assert.Empty(t, FrameworkErrorName(nil))
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
		{"custom business", Code(40001).Public("data_duplicate").Wrap(errors.New("dup")), 500},
		{"plain error", errors.New("plain"), 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, HTTPStatus(tt.err))
		})
	}
}
