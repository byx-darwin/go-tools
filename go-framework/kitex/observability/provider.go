// Package observability 提供 Kitex RPC 服务的 OpenTelemetry Provider。
package observability

import (
	"context"
	"fmt"

	"github.com/byx-darwin/go-tools/go-framework/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Provider Kitex OTel Provider.
type Provider struct {
	cfg      config.ObservabilityConfig
	tracer   trace.Tracer
	shutdown func(context.Context) error
}

// NewProvider 创建 Kitex OTel Provider（OTLP gRPC 导出）。
func NewProvider(ctx context.Context, cfg config.ObservabilityConfig) (*Provider, error) {
	p := &Provider{cfg: cfg, shutdown: func(context.Context) error { return nil }}
	if !cfg.Enabled {
		return p, nil
	}

	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("observability: create exporter: %w", err)
	}

	sampler := sdktrace.TraceIDRatioBased(1.0)
	if cfg.SampleRate > 0 {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	res, _ := resource.Merge(resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
		),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	p.tracer = tp.Tracer(cfg.ServiceName)
	p.shutdown = tp.Shutdown

	return p, nil
}

// Enabled 返回是否启用。
func (p *Provider) Enabled() bool { return p.cfg.Enabled }

// Middleware 返回 Kitex RPC 服务端 OTel 中间件。
// 签名兼容 kitex endpoint.Middleware。
func (p *Provider) Middleware() func(next func(ctx context.Context, req, resp interface{}) error) func(ctx context.Context, req, resp interface{}) error {
	if !p.cfg.Enabled {
		return func(next func(ctx context.Context, req, resp interface{}) error) func(ctx context.Context, req, resp interface{}) error {
			return next
		}
	}
	return func(next func(ctx context.Context, req, resp interface{}) error) func(ctx context.Context, req, resp interface{}) error {
		return func(ctx context.Context, req, resp interface{}) error {
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

// Shutdown 关闭 Provider。
func (p *Provider) Shutdown() error {
	return p.shutdown(context.Background())
}
