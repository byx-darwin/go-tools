package rpcerror

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
)

func TestOopsStatusAdapter(t *testing.T) {
	err := goerror.Code(frameworkerror.CodeParamInvalid).Public("bad_param").Wrap(errors.New("detail"))
	extra := map[string]string{"key": "value"}
	adapter := &OopsStatusAdapter{Err: err, Extra: extra}

	assert.Equal(t, int32(frameworkerror.CodeParamInvalid), adapter.BizStatusCode())
	assert.Equal(t, "bad_param", adapter.BizMessage())
	assert.Equal(t, extra, adapter.BizExtra())
	assert.Contains(t, adapter.Error(), "detail")
}

func TestClassify(t *testing.T) {
	// 业务错误
	bizErr := goerror.Code(10001).Public("test").Wrap(errors.New("cause"))
	assert.Equal(t, CategoryBusiness, Classify(bizErr))

	// Kitex 框架错误（oops 包装的 → 被识别为业务错误）
	frameworkErr := frameworkerror.ErrRPCUnavailable
	kitexErr := frameworkErr.Wrap(errors.New("down"))
	assert.Equal(t, CategoryBusiness, Classify(kitexErr))

	// nil error
	assert.Equal(t, CategoryUnknown, Classify(nil))

	// 普通 error
	assert.Equal(t, CategoryUnknown, Classify(errors.New("plain")))
}

func TestIsBusinessError(t *testing.T) {
	assert.True(t, IsBusinessError(goerror.Code(1).Public("x").Wrap(errors.New("y"))))
	assert.False(t, IsBusinessError(errors.New("plain")))
	assert.False(t, IsBusinessError(nil))
}

func TestIsFrameworkError(t *testing.T) {
	frameworkErr := frameworkerror.ErrRPCUnavailable.Wrap(errors.New("down"))
	assert.False(t, IsFrameworkError(frameworkErr))
	assert.False(t, IsFrameworkError(errors.New("plain")))
	assert.False(t, IsFrameworkError(nil))
}

func TestIsTimeout(t *testing.T) {
	bizTimeout := goerror.Code(frameworkerror.CodeRPCTimeout).Public("rpc_timeout").Wrap(errors.New("too slow"))
	assert.True(t, IsTimeout(bizTimeout))

	bizOther := goerror.Code(frameworkerror.CodeParamInvalid).Public("bad").Wrap(errors.New("x"))
	assert.False(t, IsTimeout(bizOther))

	assert.False(t, IsTimeout(nil))
}

func TestFrameworkErrorName(t *testing.T) {
	assert.Empty(t, FrameworkErrorName(goerror.Code(1).Public("x").Wrap(errors.New("y"))))
	assert.Empty(t, FrameworkErrorName(errors.New("plain")))
	assert.Empty(t, FrameworkErrorName(nil))
}
