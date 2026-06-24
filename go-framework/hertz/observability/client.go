package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/hertz/pkg/app/client"
	"github.com/cloudwego/hertz/pkg/protocol"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.36.0"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/byx-darwin/go-tools/go-framework/config"
)

// ClientMiddleware 创建 Hertz 客户端 OTel 中间件。
//
// 功能：
//   - 创建 client span（http.client）
//   - 注入 trace context 到 outgoing HTTP headers
//   - 注入 peer service（当前服务信息）到 outgoing HTTP headers
//   - 记录 http.client.request_count + http.client.duration metrics
func ClientMiddleware(cfg config.ObservabilityConfig) client.Middleware {
	meter := otel.GetMeterProvider().Meter(instrumentationName)
	tracer := otel.GetTracerProvider().Tracer(instrumentationName)

	clientRequestCount, _ := meter.Int64Counter(
		ClientRequestCount,
		metric.WithUnit("count"),
		metric.WithDescription("measures the client request count total"),
	)
	clientLatency, _ := meter.Float64Histogram(
		ClientLatency,
		metric.WithUnit("ms"),
		metric.WithDescription("measures the duration outbound HTTP requests"),
	)

	return func(next client.Endpoint) client.Endpoint {
		return func(ctx context.Context, req *protocol.Request, resp *protocol.Response) (err error) {
			if !cfg.Enabled {
				return next(ctx, req, resp)
			}
			if ctx == nil {
				ctx = context.Background()
			}

			start := time.Now()

			// 创建 client span
			spanName := string(req.Method()) + " " + string(req.Path())
			ctx, span := tracer.Start(ctx, spanName,
				oteltrace.WithTimestamp(start),
				oteltrace.WithSpanKind(oteltrace.SpanKindClient),
			)
			defer span.End()

			// 注入 trace context 到 outgoing HTTP headers
			otel.GetTextMapPropagator().Inject(ctx, &headerCarrier{header: &req.Header})

			// 注入 peer service（当前服务 resource attributes）到 headers
			if readOnlySpan, ok := span.(trace.ReadOnlySpan); ok {
				injectPeerServiceToHTTPHeaders(readOnlySpan.Resource().Attributes(), &req.Header)
			}

			err = next(ctx, req, resp)

			// span 属性
			span.SetAttributes(
				semconv.URLFull(req.URI().String()),
				semconv.HTTPRequestMethodKey.String(string(req.Method())),
			)

			if err == nil {
				if resp.StatusCode() >= 400 {
					span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", resp.StatusCode()))
				}
				span.SetAttributes(semconv.HTTPResponseStatusCode(resp.StatusCode()))
			} else {
				span.SetStatus(codes.Error, err.Error())
			}

			// metrics
			metricsAttrs := []attribute.KeyValue{
				semconv.HTTPRequestMethodKey.String(string(req.Method())),
				semconv.HTTPResponseStatusCode(resp.StatusCode()),
			}
			clientRequestCount.Add(ctx, 1, metric.WithAttributes(metricsAttrs...))
			clientLatency.Record(ctx, float64(time.Since(start))/float64(time.Millisecond), metric.WithAttributes(metricsAttrs...))

			return
		}
	}
}

// headerCarrier 实现 propagation.TextMapCarrier，用于 HTTP header 注入。
type headerCarrier struct {
	header *protocol.RequestHeader
}

func (h *headerCarrier) Get(key string) string {
	return h.header.Get(key)
}

func (h *headerCarrier) Set(key, value string) {
	h.header.Set(key, value)
}

func (h *headerCarrier) Keys() []string {
	var keys []string
	h.header.VisitAll(func(key, value []byte) {
		keys = append(keys, string(key))
	})
	return keys
}
