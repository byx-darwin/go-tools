package option

import (
	"testing"
	"time"

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
