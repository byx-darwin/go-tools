package observability

import (
	"crypto/tls"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"go.opentelemetry.io/otel/propagation"
)

// Option 配置 Provider 的构造行为。
type Option func(*providerOptions)

// providerOptions 收集 NewProvider 构造期可选配置。
type providerOptions struct {
	traceExporter  sdktrace.SpanExporter
	metricExporter sdkmetric.Exporter
	sampler        sdktrace.Sampler
	propagator     propagation.TextMapPropagator
	tlsConfig      *tls.Config
}

// WithTraceExporter 注入自定义 trace exporter，跳过内置的 otlptracegrpc 构建。
// 主要用于单测场景注入内存 exporter（如 tracetest.NewInMemoryExporter()），
// 避免真实网络连接。
func WithTraceExporter(exp sdktrace.SpanExporter) Option {
	return func(o *providerOptions) {
		if exp != nil {
			o.traceExporter = exp
		}
	}
}

// WithMetricExporter 注入自定义 metric exporter，跳过内置的 otlpmetricgrpc 构建。
func WithMetricExporter(exp sdkmetric.Exporter) Option {
	return func(o *providerOptions) {
		if exp != nil {
			o.metricExporter = exp
		}
	}
}

// WithSampler 覆盖默认 sampler（TraceIDRatioBased）。
func WithSampler(sampler sdktrace.Sampler) Option {
	return func(o *providerOptions) {
		if sampler != nil {
			o.sampler = sampler
		}
	}
}

// WithPropagator 覆盖默认 propagator（b3 + tracecontext + baggage）。
func WithPropagator(p propagation.TextMapPropagator) Option {
	return func(o *providerOptions) {
		if p != nil {
			o.propagator = p
		}
	}
}

// WithTLSConfig 设置自定义 TLS 配置（自定义 CA、mTLS 证书等）。
// 优先级高于 cfg.Insecure：一旦提供，exporter 始终使用该 TLS 配置建连。
// 本函数不解析证书文件路径，证书加载由调用方负责。
func WithTLSConfig(cfg *tls.Config) Option {
	return func(o *providerOptions) {
		if cfg != nil {
			o.tlsConfig = cfg
		}
	}
}

func newProviderOptions(opts []Option) *providerOptions {
	o := &providerOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
