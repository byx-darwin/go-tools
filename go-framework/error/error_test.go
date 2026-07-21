package frameworkerror_test

import (
	"errors"
	"testing"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
	"github.com/stretchr/testify/assert"
)

// TestCodeValues 码值是 wire 契约，逐值锁定（禁止改号）。
func TestCodeValues(t *testing.T) {
	assert.Equal(t, 10000, frameworkerror.CodeSystem)
	assert.Equal(t, 10001, frameworkerror.CodeParamInvalid)
	assert.Equal(t, 10002, frameworkerror.CodeAuthFailed)
	assert.Equal(t, 10003, frameworkerror.CodeConfigNotFound)
	assert.Equal(t, 10004, frameworkerror.CodeConfigInvalid)
	assert.Equal(t, 10005, frameworkerror.CodePolarisInit)
	assert.Equal(t, 10006, frameworkerror.CodePolarisGetConfig)
	assert.Equal(t, 10010, frameworkerror.CodeRPCUnavailable)
	assert.Equal(t, 10011, frameworkerror.CodeRPCTimeout)
	assert.Equal(t, 10012, frameworkerror.CodeRPCDecodeError)
	assert.Equal(t, 10013, frameworkerror.CodeRPCEncodeError)
	assert.Equal(t, 20601, frameworkerror.CodeObsInit)
	assert.Equal(t, 20602, frameworkerror.CodeObsExport)
	assert.Equal(t, 20603, frameworkerror.CodeObsTraceExport)
	assert.Equal(t, 20604, frameworkerror.CodeObsMetricExport)
	assert.Equal(t, 20605, frameworkerror.CodeObsRuntimeMetrics)
}

// TestPredefinedErrors 构造器 code + public 消息与原 go-common 定义逐值一致。
func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		code   int
		public string
	}{
		{"ErrSystem", frameworkerror.ErrSystem.Wrap(errors.New("x")), 10000, "system_error"},
		{"ErrParamInvalid", frameworkerror.ErrParamInvalid.Wrap(errors.New("x")), 10001, "param_invalid"},
		{"ErrAuthFailed", frameworkerror.ErrAuthFailed.Wrap(errors.New("x")), 10002, "auth_failed"},
		{"ErrConfigNotFound", frameworkerror.ErrConfigNotFound.Wrap(errors.New("x")), 10003, "config_not_found"},
		{"ErrConfigInvalid", frameworkerror.ErrConfigInvalid.Wrap(errors.New("x")), 10004, "config_invalid"},
		{"ErrPolarisInit", frameworkerror.ErrPolarisInit.Wrap(errors.New("x")), 10005, "polaris_init_error"},
		{"ErrPolarisGetConfig", frameworkerror.ErrPolarisGetConfig.Wrap(errors.New("x")), 10006, "polaris_get_config_error"},
		{"ErrRPCUnavailable", frameworkerror.ErrRPCUnavailable.Wrap(errors.New("x")), 10010, "rpc_unavailable"},
		{"ErrRPCTimeout", frameworkerror.ErrRPCTimeout.Wrap(errors.New("x")), 10011, "rpc_timeout"},
		{"ErrRPCDecodeError", frameworkerror.ErrRPCDecodeError.Wrap(errors.New("x")), 10012, "rpc_decode_error"},
		{"ErrRPCEncodeError", frameworkerror.ErrRPCEncodeError.Wrap(errors.New("x")), 10013, "rpc_encode_error"},
		{"ErrObsInit", frameworkerror.ErrObsInit.Wrap(errors.New("x")), 20601, "observability_init_error"},
		{"ErrObsExport", frameworkerror.ErrObsExport.Wrap(errors.New("x")), 20602, "observability_export_error"},
		{"ErrObsTraceExport", frameworkerror.ErrObsTraceExport.Wrap(errors.New("x")), 20603, "observability_trace_export_error"},
		{"ErrObsMetricExport", frameworkerror.ErrObsMetricExport.Wrap(errors.New("x")), 20604, "observability_metric_export_error"},
		{"ErrObsRuntimeMetrics", frameworkerror.ErrObsRuntimeMetrics.Wrap(errors.New("x")), 20605, "observability_runtime_metrics_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, public := goerror.Extract(tt.err)
			assert.Equal(t, tt.code, code)
			assert.Equal(t, tt.public, public)
		})
	}
}

// TestHTTPStatusRegistration init() 注册的细粒度映射经 goerror.HTTPStatus 生效，
// 全部 case 与原 go-common/error httpStatusByCode 逐值一致。
func TestHTTPStatusRegistration(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"param invalid → 400", frameworkerror.ErrParamInvalid.Wrap(errors.New("x")), 400},
		{"auth failed → 401", frameworkerror.ErrAuthFailed.Wrap(errors.New("x")), 401},
		{"system → 500", frameworkerror.ErrSystem.Wrap(errors.New("x")), 500},
		{"config not found → 500", frameworkerror.ErrConfigNotFound.Wrap(errors.New("x")), 500},
		{"config invalid → 500", frameworkerror.ErrConfigInvalid.Wrap(errors.New("x")), 500},
		{"rpc decode → 500", frameworkerror.ErrRPCDecodeError.Wrap(errors.New("x")), 500},
		{"rpc encode → 500", frameworkerror.ErrRPCEncodeError.Wrap(errors.New("x")), 500},
		{"rpc unavailable → 503", frameworkerror.ErrRPCUnavailable.Wrap(errors.New("x")), 503},
		{"rpc timeout → 504", frameworkerror.ErrRPCTimeout.Wrap(errors.New("x")), 504},
		{"polaris init → 503", frameworkerror.ErrPolarisInit.Wrap(errors.New("x")), 503},
		{"polaris get config → 503", frameworkerror.ErrPolarisGetConfig.Wrap(errors.New("x")), 503},
		{"obs init → 503", frameworkerror.ErrObsInit.Wrap(errors.New("x")), 503},
		{"obs export → 500", frameworkerror.ErrObsExport.Wrap(errors.New("x")), 500},
		{"obs trace export → 503", frameworkerror.ErrObsTraceExport.Wrap(errors.New("x")), 503},
		{"obs metric export → 503", frameworkerror.ErrObsMetricExport.Wrap(errors.New("x")), 503},
		{"obs runtime metrics → 503", frameworkerror.ErrObsRuntimeMetrics.Wrap(errors.New("x")), 503},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, goerror.HTTPStatus(tt.err))
		})
	}
}
