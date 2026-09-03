package observability

import (
	"context"
	"crypto/tls"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/byx-darwin/go-tools/go-framework/config"
)

// fakeMetricExporter 是最小化的 sdkmetric.Exporter 测试替身，用于验证
// WithMetricExporter 注入的 exporter 确实被使用（而不是内置的 OTLP gRPC
// exporter）。
type fakeMetricExporter struct {
	exportCount int32
}

func (f *fakeMetricExporter) Temporality(kind sdkmetric.InstrumentKind) metricdata.Temporality {
	return sdkmetric.DefaultTemporalitySelector(kind)
}

func (f *fakeMetricExporter) Aggregation(kind sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	return sdkmetric.DefaultAggregationSelector(kind)
}

func (f *fakeMetricExporter) Export(_ context.Context, _ *metricdata.ResourceMetrics) error {
	atomic.AddInt32(&f.exportCount, 1)
	return nil
}

func (f *fakeMetricExporter) ForceFlush(_ context.Context) error { return nil }

func (f *fakeMetricExporter) Shutdown(_ context.Context) error { return nil }

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
	tp, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider)
	require.True(t, ok)
	require.NoError(t, tp.ForceFlush(context.Background()))

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

// TestTLSDialCredentials_Priority 是 tlsDialCredentials 的表驱动单测，直接
// 验证 WithTLSConfig > cfg.Insecure > 默认 TLS 的优先级，不依赖
// NewProvider 的错误返回值（otlptracegrpc.New 是 lazy-connect，构造期不会
// 因为凭据错误而报错，因此不能靠 NewProvider 的 err 来做回归保护）。
func TestTLSDialCredentials_Priority(t *testing.T) {
	cases := []struct {
		name         string
		cfg          config.ObservabilityConfig
		tlsCfg       *tls.Config
		wantInsecure bool
		wantCredsNil bool
	}{
		{"default_secure", config.ObservabilityConfig{}, nil, false, false},
		{"insecure_true", config.ObservabilityConfig{Insecure: true}, nil, true, true},
		{"tls_config_overrides_insecure", config.ObservabilityConfig{Insecure: true}, &tls.Config{MinVersion: tls.VersionTLS12}, false, false},
		{"tls_config_alone", config.ObservabilityConfig{}, &tls.Config{MinVersion: tls.VersionTLS12}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			popts := &providerOptions{}
			if tc.tlsCfg != nil {
				WithTLSConfig(tc.tlsCfg)(popts)
			}
			creds, insecure := tlsDialCredentials(tc.cfg, popts)
			assert.Equal(t, tc.wantInsecure, insecure)
			if tc.wantCredsNil {
				assert.Nil(t, creds)
			} else {
				assert.NotNil(t, creds)
			}
		})
	}
}

func TestNewProvider_WithMetricExporter_UsesInjectedExporter(t *testing.T) {
	exp := &fakeMetricExporter{}
	cfg := config.ObservabilityConfig{
		Enabled:         true,
		ServiceName:     "test-svc",
		EnableMetrics:   true,
		MetricsInterval: time.Hour, // 避免自动周期性触发，靠 ForceFlush 手动触发
	}
	p, err := NewProvider(context.Background(), cfg, WithMetricExporter(exp))
	require.NoError(t, err)
	require.NotNil(t, p)
	require.NotNil(t, p.meterProvider)

	require.NoError(t, p.meterProvider.ForceFlush(context.Background()))
	assert.GreaterOrEqual(t, atomic.LoadInt32(&exp.exportCount), int32(1))

	require.NoError(t, p.Shutdown())
}

// TestNewProvider_WithMetricExporter_MetricsDisabled 验证 EnableMetrics=false
// 时，即便注入了 WithMetricExporter，也不会构建 metrics pipeline
// （p.meterProvider 保持 nil，注入的 exporter 被静默忽略）。
func TestNewProvider_WithMetricExporter_MetricsDisabled(t *testing.T) {
	exp := &fakeMetricExporter{}
	cfg := config.ObservabilityConfig{
		Enabled:       true,
		ServiceName:   "test-svc",
		EnableMetrics: false,
	}
	p, err := NewProvider(context.Background(), cfg, WithMetricExporter(exp))
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Nil(t, p.meterProvider)
	assert.Equal(t, int32(0), atomic.LoadInt32(&exp.exportCount))

	require.NoError(t, p.Shutdown())
}
