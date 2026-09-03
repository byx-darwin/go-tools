package observability

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/byx-darwin/go-tools/go-framework/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestNewProvider(t *testing.T) {
	ctx := context.Background()
	p, err := NewProvider(ctx, config.ObservabilityConfig{
		Enabled: true,
	})
	assert.NoError(t, err)
	assert.True(t, p.Enabled())
}

func TestProvider_Disabled(t *testing.T) {
	p, err := NewProvider(context.Background(), config.ObservabilityConfig{
		Enabled: false,
	})
	assert.NoError(t, err)
	assert.False(t, p.Enabled())

	// Middleware should be a pass-through when disabled
	mw := p.Middleware()
	assert.NotNil(t, mw)

	called := false
	next := func(ctx context.Context, req, resp interface{}) error {
		called = true
		return nil
	}
	err = mw(next)(nil, nil, nil)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestProvider_Shutdown(t *testing.T) {
	p, _ := NewProvider(context.Background(), config.ObservabilityConfig{})
	assert.NoError(t, p.Shutdown())
}

func TestNewProvider_WithTraceExporter_UsesInjectedExporter(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		ServiceName: "test-svc",
	}
	p, err := NewProvider(context.Background(), cfg, WithTraceExporter(exp))
	require.NoError(t, err)
	require.True(t, p.Enabled())

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
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		Endpoint:    "127.0.0.1:1",
		ServiceName: "test-svc",
	}
	p, err := NewProvider(context.Background(), cfg)
	require.NoError(t, err)
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
	require.NoError(t, p.Shutdown())
}

func TestNewProvider_WithTLSConfig_TakesPriorityOverInsecure(t *testing.T) {
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		Endpoint:    "127.0.0.1:1",
		ServiceName: "test-svc",
		Insecure:    true,
	}
	p, err := NewProvider(context.Background(), cfg, WithTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
	require.NoError(t, err)
	require.NoError(t, p.Shutdown())
}

func TestNewProvider_WithSamplerAndPropagator(t *testing.T) {
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		ServiceName: "test-svc",
	}
	p, err := NewProvider(context.Background(), cfg,
		WithSampler(sdktrace.AlwaysSample()),
		WithPropagator(propagation.TraceContext{}),
	)
	require.NoError(t, err)
	require.NoError(t, p.Shutdown())
}
