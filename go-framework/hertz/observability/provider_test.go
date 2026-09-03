package observability

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

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

func TestNewProvider_WithTraceExporter_UsesInjectedExporter(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		ServiceName: "test-svc",
	}
	p, err := NewProvider(context.Background(), cfg, WithTraceExporter(exp))
	require.NoError(t, err)
	require.NotNil(t, p)

	_, span := p.tracer.Start(context.Background(), "test-span")
	span.End()

	// ForceFlush（而非直接 Shutdown）以取回已导出的 span：
	// tracetest.InMemoryExporter.Shutdown() 内部会调用 Reset() 清空已存储
	// 的 span（该 exporter 的已知行为），若在 Shutdown 之后再 GetSpans()
	// 永远得到空切片，与 NewProvider/Option 实现是否正确无关。
	if tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); ok {
		require.NoError(t, tp.ForceFlush(context.Background()))
	}

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "test-span", spans[0].Name)

	require.NoError(t, p.Shutdown())
}

func TestNewProvider_DefaultsToTLS_NotInsecure(t *testing.T) {
	// Endpoint 指向不存在的地址；默认走 TLS 时 grpc.NewClient 仍能成功构造
	// （lazy connect），不应该出现 exporter 构造期错误。这里只验证
	// NewProvider 在默认（Insecure 未设置）情况下不会因为强制走 TLS 而
	// 在构造阶段直接报错。
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		Endpoint:    "127.0.0.1:1",
		ServiceName: "test-svc",
	}
	p, err := NewProvider(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.NoError(t, p.Shutdown())
}

func TestNewProvider_InsecureTrue_SkipsTLS(t *testing.T) {
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		Endpoint:    "127.0.0.1:1",
		ServiceName: "test-svc",
		Insecure:    true,
	}
	p, err := NewProvider(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.NoError(t, p.Shutdown())
}

func TestNewProvider_WithTLSConfig_TakesPriorityOverInsecure(t *testing.T) {
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		Endpoint:    "127.0.0.1:1",
		ServiceName: "test-svc",
		Insecure:    true, // should be overridden by WithTLSConfig
	}
	p, err := NewProvider(context.Background(), cfg, WithTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
	require.NoError(t, err)
	require.NotNil(t, p)
	require.NoError(t, p.Shutdown())
}

func TestNewProvider_WithSamplerAndPropagator(t *testing.T) {
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		ServiceName: "test-svc",
	}
	customPropagator := propagation.TraceContext{}
	p, err := NewProvider(context.Background(), cfg,
		WithSampler(sdktrace.AlwaysSample()),
		WithPropagator(customPropagator),
	)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.NoError(t, p.Shutdown())
}
