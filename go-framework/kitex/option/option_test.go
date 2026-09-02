package option

import (
	"reflect"
	"testing"
	"time"

	"github.com/cloudwego/kitex/pkg/circuitbreak"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/byx-darwin/go-tools/go-framework/config/kitex"
)

func TestNewServerOption_NilConfig(t *testing.T) {
	_, err := NewServerOption(t.Context(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "server config is nil")
}

func TestNewServerOption_Defaults(t *testing.T) {
	cfg := &kitex.ServerConfig{}
	opts, err := NewServerOption(t.Context(), cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, opts)
	assert.Equal(t, "tcp", cfg.RPC.Network)
}

func TestNewServerOption_WithPort(t *testing.T) {
	cfg := &kitex.ServerConfig{
		RPC: &kitex.RPCOption{Port: "8080"},
	}
	opts, err := NewServerOption(t.Context(), cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, opts)
}

func TestNewServerOption_WithTimeout(t *testing.T) {
	cfg := &kitex.ServerConfig{
		RPC:     &kitex.RPCOption{Port: "8080"},
		Timeout: &kitex.ServerTimeout{ReadWriteTimeout: 10 * time.Second, ExitWaitTimeout: 3 * time.Second},
	}
	opts, err := NewServerOption(t.Context(), cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, opts)
}

func TestNewServerOption_WithLimit(t *testing.T) {
	cfg := &kitex.ServerConfig{
		RPC:   &kitex.RPCOption{Port: "8080"},
		Limit: &kitex.LimitOption{Enable: true, MaxConnections: 100, MaxQPS: 50},
	}
	opts, err := NewServerOption(t.Context(), cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, opts)
}

func TestNewClientOption_NilConfig(t *testing.T) {
	_, err := NewClientOption(t.Context(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client config is nil")
}

func TestNewClientOption_DefaultTimeouts(t *testing.T) {
	cfg := &kitex.ClientConfig{
		ClientOption: &kitex.ClientOption{},
	}
	opts, err := NewClientOption(t.Context(), cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, opts)
}

func TestNewClientOption_ExplicitTimeoutsOverrideDefaults(t *testing.T) {
	cfg := &kitex.ClientConfig{
		ClientOption: &kitex.ClientOption{
			Timeout: kitex.ClientTimeout{
				RPCTimeout:     10 * time.Second,
				ConnectTimeOut: 500 * time.Millisecond,
			},
		},
	}
	opts, err := NewClientOption(t.Context(), cfg)
	require.NoError(t, err)
	assert.NotEmpty(t, opts)
}

func TestNewClientOption_CBSuiteDisabled(t *testing.T) {
	cfg := &kitex.ClientConfig{
		ClientOption: &kitex.ClientOption{
			CBSuite: kitex.CBSuite{Enable: false},
		},
	}
	optsDisabled, err := NewClientOption(t.Context(), cfg)
	require.NoError(t, err)

	cfg.ClientOption.CBSuite.Enable = true
	optsEnabled, err := NewClientOption(t.Context(), cfg)
	require.NoError(t, err)

	assert.Greater(t, len(optsEnabled), len(optsDisabled))
}

func TestWithCircuitBreakerKeyFunc_DefaultsToRPCInfo2Key(t *testing.T) {
	occ := &clientOptionConfig{cbKeyFunc: circuitbreak.RPCInfo2Key}
	assert.NotNil(t, occ.cbKeyFunc)
}

func TestWithCircuitBreakerKeyFunc_Custom(t *testing.T) {
	called := false
	custom := func(ri rpcinfo.RPCInfo) string {
		called = true
		return "custom-key"
	}

	occ := &clientOptionConfig{cbKeyFunc: circuitbreak.RPCInfo2Key}
	WithCircuitBreakerKeyFunc(custom)(occ)
	require.NotNil(t, occ.cbKeyFunc)

	got := occ.cbKeyFunc(nil)
	assert.True(t, called)
	assert.Equal(t, "custom-key", got)
}

func TestWithCircuitBreakerKeyFunc_NilIgnored(t *testing.T) {
	occ := &clientOptionConfig{cbKeyFunc: circuitbreak.RPCInfo2Key}
	before := reflect.ValueOf(occ.cbKeyFunc).Pointer()

	WithCircuitBreakerKeyFunc(nil)(occ)

	after := reflect.ValueOf(occ.cbKeyFunc).Pointer()
	assert.Equal(t, before, after)
}

func TestNewClientOption_CustomCBKeyFuncApplied(t *testing.T) {
	cfg := &kitex.ClientConfig{
		ClientOption: &kitex.ClientOption{
			CBSuite: kitex.CBSuite{Enable: true},
		},
	}
	custom := func(ri rpcinfo.RPCInfo) string { return "k" }

	opts, err := NewClientOption(t.Context(), cfg, WithCircuitBreakerKeyFunc(custom))
	require.NoError(t, err)
	assert.NotEmpty(t, opts)
}

func TestResolveAddr(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		port string
		want string
	}{
		{"empty port", "10.0.0.1", "", "10.0.0.1:9000"},
		{"port with colon", "10.0.0.1", ":8080", "10.0.0.1:8080"},
		{"port without colon", "10.0.0.1", "8080", "10.0.0.1:8080"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, resolveAddr(tt.ip, tt.port))
		})
	}
}
