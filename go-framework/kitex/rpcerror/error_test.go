package rpcerror

import (
	"errors"
	"testing"

	"github.com/cloudwego/kitex/pkg/kerrors"
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

	// 原生 Kitex 框架错误（basicError）
	assert.Equal(t, CategoryFramework, Classify(kerrors.ErrRPCTimeout))

	// 原生 Kitex 框架错误（DetailedError）
	assert.Equal(t, CategoryFramework, Classify(kerrors.ErrNoConnection))
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
	assert.True(t, IsFrameworkError(kerrors.ErrRPCTimeout))
}

func TestIsTimeout(t *testing.T) {
	bizTimeout := goerror.Code(frameworkerror.CodeRPCTimeout).Public("rpc_timeout").Wrap(errors.New("too slow"))
	assert.True(t, IsTimeout(bizTimeout))

	bizOther := goerror.Code(frameworkerror.CodeParamInvalid).Public("bad").Wrap(errors.New("x"))
	assert.False(t, IsTimeout(bizOther))

	assert.False(t, IsTimeout(nil))

	// 原生 Kitex 超时错误（kerrors.IsTimeoutError 分支）
	assert.True(t, IsTimeout(kerrors.ErrRPCTimeout))
	// errors.Is 语义下，WithCause 包装的超时错误依然满足 errors.Is(err, ErrRPCTimeout)
	assert.True(t, IsTimeout(kerrors.ErrRPCTimeout.WithCause(errors.New("deadline exceeded"))))
}

func TestFrameworkErrorName(t *testing.T) {
	assert.Empty(t, FrameworkErrorName(goerror.Code(1).Public("x").Wrap(errors.New("y"))))
	assert.Empty(t, FrameworkErrorName(errors.New("plain")))
	assert.Empty(t, FrameworkErrorName(nil))

	cases := []struct {
		name string
		err  error
		want string
	}{
		{"rpc_timeout", kerrors.ErrRPCTimeout, "ErrRPCTimeout"},
		{"internal_exception", kerrors.ErrInternalException, "ErrInternalException"},
		{"service_discovery", kerrors.ErrServiceDiscovery, "ErrServiceDiscovery"},
		{"get_connection", kerrors.ErrGetConnection, "ErrGetConnection"},
		{"loadbalance", kerrors.ErrLoadbalance, "ErrLoadbalance"},
		{"no_more_instance", kerrors.ErrNoMoreInstance, "ErrNoMoreInstance"},
		{"canceled_by_business", kerrors.ErrCanceledByBusiness, "ErrCanceledByBusiness"},
		{"timeout_by_business", kerrors.ErrTimeoutByBusiness, "ErrTimeoutByBusiness"},
		{"acl", kerrors.ErrACL, "ErrACL"},
		{"circuit_break", kerrors.ErrCircuitBreak, "ErrCircuitBreak"},
		{"remote_or_network", kerrors.ErrRemoteOrNetwork, "ErrRemoteOrNetwork"},
		{"overlimit", kerrors.ErrOverlimit, "ErrOverlimit"},
		{"panic", kerrors.ErrPanic, "ErrPanic"},
		{"biz", kerrors.ErrBiz, "ErrBiz"},
		{"retry", kerrors.ErrRetry, "ErrRetry"},
		{"route", kerrors.ErrRoute, "ErrRoute"},
		{"payload_validation", kerrors.ErrPayloadValidation, "ErrPayloadValidation"},
		// ErrRPCFinish 是已废弃且未在 FrameworkErrorName switch 中显式匹配的
		// basicError，用于覆盖 default 分支。
		{"unknown_kitex_error", kerrors.ErrRPCFinish, "UnknownKitexError"}, //nolint:staticcheck // ErrRPCFinish 已废弃但仍是 kitex 保留的 basicError，是唯一未被 FrameworkErrorName switch 显式匹配的分支，用于覆盖 default 分支
		// DetailedError（WithCause 包装）应通过 errors.Is 命中对应分支。
		{"detailed_no_connection", kerrors.ErrNoConnection, "ErrInternalException"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, FrameworkErrorName(tc.err))
		})
	}
}
