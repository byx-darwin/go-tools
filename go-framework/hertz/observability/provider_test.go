package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/byx-darwin/go-tools/go-framework/config"
)

func TestNewProvider_Disabled(t *testing.T) {
	cfg := config.ObservabilityConfig{Enabled: false}
	p, err := NewProvider(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.False(t, p.cfg.Enabled)
}

func TestNewProvider_Enabled_InvalidEndpoint(t *testing.T) {
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		Endpoint:    "invalid-endpoint-that-will-fail:9999",
		ServiceName: "test-svc",
	}
	// OTel exporter creation may succeed or fail depending on DNS resolution.
	// We mainly test that NewProvider doesn't panic.
	_, _ = NewProvider(context.Background(), cfg)
}

func TestProvider_Shutdown_Disabled(t *testing.T) {
	cfg := config.ObservabilityConfig{Enabled: false}
	p, err := NewProvider(context.Background(), cfg)
	require.NoError(t, err)
	err = p.Shutdown()
	assert.NoError(t, err)
}

func TestProvider_ServerMiddleware_Disabled(t *testing.T) {
	cfg := config.ObservabilityConfig{Enabled: false}
	p, err := NewProvider(context.Background(), cfg)
	require.NoError(t, err)
	mw := p.ServerMiddleware()
	assert.NotNil(t, mw)
}

func TestProvider_ServerTracer(t *testing.T) {
	cfg := config.ObservabilityConfig{
		Enabled:     false,
		ServiceName: "test-svc",
	}
	p, err := NewProvider(context.Background(), cfg)
	require.NoError(t, err)
	tracer, tracerCfg := p.ServerTracer()
	assert.NotNil(t, tracer)
	assert.NotNil(t, tracerCfg)
}

func TestProvider_Enabled_WithMetrics(t *testing.T) {
	// This test verifies the code path when metrics are enabled.
	// The actual OTel gRPC connection will fail in test env, so we just
	// verify the initialization logic doesn't panic.
	cfg := config.ObservabilityConfig{
		Enabled:         true,
		Endpoint:        "localhost:4317",
		ServiceName:     "test-svc",
		EnableMetrics:   true,
		MetricsInterval: 0, // will use default 15s
	}
	// We don't assert on the error since gRPC connection may fail in CI.
	_, _ = NewProvider(context.Background(), cfg)
}
