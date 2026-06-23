package observability

import (
	"context"
	"fmt"

	"gitee.com/byx_darwin/go-tools/go-framework/config"
	"github.com/cloudwego/hertz/pkg/app"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.36.0"
	"go.opentelemetry.io/otel/trace"
)

// Provider Hertz OTel Provider.
type Provider struct {
	cfg      config.ObservabilityConfig
	tracer   trace.Tracer
	shutdown func(context.Context) error
}

// NewProvider 创建 Hertz OTel Provider（OTLP gRPC 导出）。
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

// ServerMiddleware 返回 Hertz 服务端链路追踪中间件。
func (p *Provider) ServerMiddleware() app.HandlerFunc {
	if !p.cfg.Enabled {
		return func(ctx context.Context, c *app.RequestContext) { c.Next(ctx) }
	}

	return func(ctx context.Context, c *app.RequestContext) {
		// Extract incoming trace context
		ctx = otel.GetTextMapPropagator().Extract(ctx, &hertzCarrier{c: c})

		spanName := fmt.Sprintf("%s %s",
			string(c.Request.Method()),
			string(c.Request.Path()),
		)

		ctx, span := p.tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		span.SetAttributes(
			attribute.String("http.method", string(c.Request.Method())),
			attribute.String("http.path", string(c.Request.Path())),
			attribute.Int("http.status_code", c.Response.StatusCode()),
		)

		c.Next(ctx)
	}
}

// Shutdown 关闭 Provider。
func (p *Provider) Shutdown() error {
	return p.shutdown(context.Background())
}

type hertzCarrier struct {
	c *app.RequestContext
}

func (h *hertzCarrier) Get(key string) string {
	return string(h.c.GetHeader(key))
}

func (h *hertzCarrier) Set(key, value string) {
	h.c.Response.Header.Set(key, value)
}

func (h *hertzCarrier) Keys() []string {
	var keys []string
	h.c.Request.Header.VisitAll(func(k, _ []byte) {
		keys = append(keys, string(k))
	})
	return keys
}
