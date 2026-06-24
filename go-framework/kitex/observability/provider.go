// Package observability 提供 Kitex RPC 服务的 OpenTelemetry Provider（Tracing + Metrics）。
package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/byx-darwin/go-tools/go-framework/config"
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
func NewProvider(ctx context.Context, cfg config.ObservabilityConfig) (*Provider, error) {
	p := &Provider{cfg: cfg, shutdown: func(context.Context) error { return nil }}
	if !cfg.Enabled {
		return p, nil
	}

	res, _ := resource.Merge(resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
		),
	)

	// ── Tracing ──
	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("observability: create trace exporter: %w", err)
	}

	sampler := sdktrace.TraceIDRatioBased(1.0)
	if cfg.SampleRate > 0 {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		b3.New(),
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	p.tracer = tp.Tracer(cfg.ServiceName)
	p.shutdown = tp.Shutdown

	// ── Metrics ──
	if cfg.EnableMetrics {
		metricExp, err := otlpmetricgrpc.New(ctx,
			otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
			otlpmetricgrpc.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("observability: create metric exporter: %w", err)
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
			return nil, fmt.Errorf("observability: start runtime metrics: %w", err)
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
