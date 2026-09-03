// Package observability 提供 Kitex RPC 服务的 OpenTelemetry Provider（Tracing + Metrics）。
package observability

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/byx-darwin/go-tools/go-framework/config"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
	runtimemetrics "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)

// Provider Kitex OTel Provider（Tracing + Metrics）。
type Provider struct {
	cfg           config.ObservabilityConfig
	tracer        trace.Tracer
	meterProvider *sdkmetric.MeterProvider
	shutdown      func(context.Context) error
}

// NewProvider 创建 Kitex OTel Provider（OTLP gRPC 导出）。
//
// 同时初始化 Tracing 和 Metrics 双通道：
//   - Tracing：OTLP gRPC exporter → TracerProvider
//   - Metrics：OTLP gRPC exporter → MeterProvider + Go runtime metrics
//
// 通过 cfg.EnableMetrics 控制是否启用 Metrics（默认 true，当 Enabled=true 时）。
//
// exporter 的 TLS 凭据按以下优先级选择：
//  1. WithTLSConfig 提供的自定义 *tls.Config（自定义 CA / mTLS）
//  2. cfg.Insecure = true → 明文传输
//  3. 默认：使用系统根证书池建立 TLS 连接
//
// 可通过 WithTraceExporter/WithMetricExporter 注入自定义 exporter（跳过 OTLP gRPC
// 构建，常用于测试隔离），WithSampler/WithPropagator 覆盖默认 sampler/propagator。
func NewProvider(ctx context.Context, cfg config.ObservabilityConfig, opts ...Option) (*Provider, error) {
	p := &Provider{cfg: cfg, shutdown: func(context.Context) error { return nil }}
	if !cfg.Enabled {
		return p, nil
	}

	popts := newProviderOptions(opts)

	res, _ := resource.Merge(resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
		),
	)

	// ── Tracing ──
	var exp sdktrace.SpanExporter
	if popts.traceExporter != nil {
		exp = popts.traceExporter
	} else {
		traceDialOpts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.Endpoint)}
		traceDialOpts = append(traceDialOpts, traceTLSOption(cfg, popts))

		var err error
		exp, err = otlptracegrpc.New(ctx, traceDialOpts...)
		if err != nil {
			return nil, frameworkerror.ErrObsTraceExport.Wrap(err)
		}
	}

	sampler := sdktrace.TraceIDRatioBased(1.0)
	if cfg.SampleRate > 0 {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}
	if popts.sampler != nil {
		sampler = popts.sampler
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	propagator := propagation.NewCompositeTextMapPropagator(
		b3.New(),
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	if popts.propagator != nil {
		propagator = popts.propagator
	}
	otel.SetTextMapPropagator(propagator)

	p.tracer = tp.Tracer(cfg.ServiceName)
	p.shutdown = tp.Shutdown

	// ── Metrics ──
	if cfg.EnableMetrics {
		var metricExp sdkmetric.Exporter
		if popts.metricExporter != nil {
			metricExp = popts.metricExporter
		} else {
			metricDialOpts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(cfg.Endpoint)}
			metricDialOpts = append(metricDialOpts, metricTLSOption(cfg, popts))

			var err error
			metricExp, err = otlpmetricgrpc.New(ctx, metricDialOpts...)
			if err != nil {
				return nil, frameworkerror.ErrObsMetricExport.Wrap(err)
			}
		}

		interval := cfg.MetricsInterval
		if interval <= 0 {
			interval = 15 * time.Second
		}

		reader := sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(interval))
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(reader),
			sdkmetric.WithResource(res),
		)
		otel.SetMeterProvider(mp)
		p.meterProvider = mp

		// Go runtime metrics（goroutines, GC, memory 等）
		if err := runtimemetrics.Start(runtimemetrics.WithMeterProvider(mp)); err != nil {
			return nil, frameworkerror.ErrObsRuntimeMetrics.Wrap(err)
		}

		// 包装 shutdown 以同时关闭 metrics
		prevShutdown := p.shutdown
		p.shutdown = func(ctx context.Context) error {
			errT := prevShutdown(ctx)
			errM := mp.Shutdown(ctx)
			if errT != nil {
				return errT
			}
			return errM
		}
	}

	return p, nil
}

// tlsDialCredentials 按优先级（WithTLSConfig > cfg.Insecure > 默认 TLS）返回
// gRPC 传输凭据。insecure=true 时 creds 为 nil，调用方应使用 exporter 包的
// WithInsecure()；insecure=false 时 creds 非 nil，调用方应使用
// WithTLSCredentials(creds)。
func tlsDialCredentials(cfg config.ObservabilityConfig, popts *providerOptions) (creds credentials.TransportCredentials, insecure bool) {
	switch {
	case popts.tlsConfig != nil:
		return credentials.NewTLS(popts.tlsConfig), false
	case cfg.Insecure:
		return nil, true
	default:
		return credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12}), false
	}
}

// traceTLSOption 按优先级（WithTLSConfig > cfg.Insecure > 默认 TLS）返回
// otlptracegrpc 的凭据 Option。
func traceTLSOption(cfg config.ObservabilityConfig, popts *providerOptions) otlptracegrpc.Option {
	creds, insecure := tlsDialCredentials(cfg, popts)
	if insecure {
		return otlptracegrpc.WithInsecure()
	}
	return otlptracegrpc.WithTLSCredentials(creds)
}

// metricTLSOption 按优先级（WithTLSConfig > cfg.Insecure > 默认 TLS）返回
// otlpmetricgrpc 的凭据 Option。
func metricTLSOption(cfg config.ObservabilityConfig, popts *providerOptions) otlpmetricgrpc.Option {
	creds, insecure := tlsDialCredentials(cfg, popts)
	if insecure {
		return otlpmetricgrpc.WithInsecure()
	}
	return otlpmetricgrpc.WithTLSCredentials(creds)
}

// Enabled 返回是否启用。
func (p *Provider) Enabled() bool { return p.cfg.Enabled }

// MeterProvider 返回 OTel MeterProvider（仅当 EnableMetrics=true 时非 nil）。
func (p *Provider) MeterProvider() *sdkmetric.MeterProvider { return p.meterProvider }

// Middleware 返回 Kitex RPC 服务端 OTel 中间件（简化版，仅 span 包裹）。
//
// 推荐使用 ServerSuite() 替代，它能提供更丰富的 RPC 元数据采集（通过 stats.Tracer）。
func (p *Provider) Middleware() func(next func(ctx context.Context, req, resp any) error) func(ctx context.Context, req, resp any) error {
	if !p.cfg.Enabled {
		return func(next func(ctx context.Context, req, resp any) error) func(ctx context.Context, req, resp any) error {
			return next
		}
	}
	return func(next func(ctx context.Context, req, resp any) error) func(ctx context.Context, req, resp any) error {
		return func(ctx context.Context, req, resp any) error {
			ctx, span := p.tracer.Start(ctx, "rpc",
				trace.WithSpanKind(trace.SpanKindServer),
			)
			defer span.End()

			err := next(ctx, req, resp)

			span.SetAttributes(
				attribute.String("rpc.method", fmt.Sprintf("%T", req)),
			)
			if err != nil {
				span.RecordError(err)
				span.SetAttributes(attribute.String("error", err.Error()))
			}

			return err
		}
	}
}

// ServerSuite 返回 Kitex 服务端 OTel Suite（推荐）。
//
// 相比 Middleware()，ServerSuite 通过 stats.Tracer 接口提供更丰富的 RPC 元数据：
//   - rpc.method, rpc.service, rpc.system
//   - 传输协议、收发包大小
//   - 错误 + panic 记录
//   - RPC duration metrics（如果 EnableMetrics=true）
func (p *Provider) ServerSuite() *serverSuite {
	return NewServerSuite(p.cfg)
}

// ClientSuite 返回 Kitex 客户端 OTel Suite。
func (p *Provider) ClientSuite() *clientSuite {
	return NewClientSuite(p.cfg)
}

// Shutdown 关闭 Provider（Tracing + Metrics）。
func (p *Provider) Shutdown() error {
	return p.shutdown(context.Background())
}
