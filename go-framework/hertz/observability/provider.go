package observability

import (
	"context"
	"fmt"
	"time"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/byx-darwin/go-tools/go-framework/config"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/tracer"
	"github.com/cloudwego/hertz/pkg/common/tracer/stats"
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
	semconv "go.opentelemetry.io/otel/semconv/v1.36.0"
	"go.opentelemetry.io/otel/trace"
)

// Provider Hertz OTel Provider（Tracing + Metrics）。
type Provider struct {
	cfg           config.ObservabilityConfig
	tracer        trace.Tracer
	meterProvider *sdkmetric.MeterProvider
	shutdown      func(context.Context) error
}

// NewProvider 创建 Hertz OTel Provider（OTLP gRPC 导出）。
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
		return nil, goerror.ErrObsTraceExport.Wrap(err)
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
			return nil, goerror.ErrObsMetricExport.Wrap(err)
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
			return nil, goerror.ErrObsRuntimeMetrics.Wrap(err)
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

// ServerMiddleware 返回 Hertz 服务端链路追踪中间件（简化版）。
//
// 推荐使用 ServerTracer() + tracer-aware ServerMiddleware 以获得完整的 HTTP metrics 采集。
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

// ServerTracer 返回 Hertz tracer.Tracer 实现，用于 server.WithTracer() 接入。
//
// 与简化版 ServerMiddleware 不同，ServerTracer 通过 tracer.Tracer 接口
// 提供更丰富的 HTTP 元数据采集：
//   - OTel HTTP 标准 semconv 属性（url.full, client.address, http.route 等）
//   - http.server.request_count + http.server.duration metrics
//   - Stats 事件注入（read_header, read_body, write 等）
//   - Panic 错误记录
//
// 用法：
//
//	tracer, _ := p.ServerTracer()
//	h := server.Default(
//	    server.WithTracer(tracer),
//	    server.WithHostPorts(":8080"),
//	)
//	h.Use(TracerServerMiddleware(p.cfg))
func (p *Provider) ServerTracer() (tracer.Tracer, *TracerConfig) {
	return NewServerTracer(p.cfg)
}

// TracerServerMiddleware 配合 ServerTracer 使用的服务端中间件。
//
// 与 ServerMiddleware 的区别：本中间件依赖 tracer.Tracer 的 Start/Finish 生命周期，
// span 创建和 metrics 记录由 serverTracer 完成，中间件仅负责 trace context 提取和 span 关联。
func TracerServerMiddleware(cfg config.ObservabilityConfig) app.HandlerFunc {
	if !cfg.Enabled {
		return func(ctx context.Context, c *app.RequestContext) { c.Next(ctx) }
	}

	return func(ctx context.Context, c *app.RequestContext) {
		tc := traceCarrierFromContext(ctx)
		if tc == nil {
			c.Next(ctx)
			return
		}

		ti := c.GetTraceInfo()
		if ti.Stats().Level() == stats.LevelDisabled {
			c.Next(ctx)
			return
		}

		// 提取 trace context + peer service
		ctx = otel.GetTextMapPropagator().Extract(ctx, &hertzCarrier{c: c})

		peerAttrs := extractPeerServiceAttributesFromHTTPHeaders(&c.Request.Header)

		ctx, span := tc.Tracer().Start(ctx, "HTTP "+string(c.Request.Method()),
			trace.WithSpanKind(trace.SpanKindServer),
		)

		span.SetAttributes(peerAttrs...)
		tc.SetSpan(span)

		c.Next(ctx)

		// span.End() 和 metrics 由 serverTracer.Finish() 完成
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
