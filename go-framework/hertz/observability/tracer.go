package observability

import (
	"context"
	"fmt"
	"time"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
	"github.com/cloudwego/hertz/pkg/common/tracer"
	"github.com/cloudwego/hertz/pkg/common/tracer/stats"
	"github.com/cloudwego/hertz/pkg/common/tracer/traceinfo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.36.0"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/byx-darwin/go-tools/go-framework/config"
)

// instrumentationName OTel instrumentation 标识。
const instrumentationName = "github.com/byx-darwin/go-tools/go-framework/hertz/observability"

var _ tracer.Tracer = (*serverTracer)(nil)

// serverTracer 实现 Hertz tracer.Tracer 接口，在 HTTP 生命周期中记录 span 和 HTTP metrics。
type serverTracer struct {
	cfg        config.ObservabilityConfig
	tracer     oteltrace.Tracer
	meter      metric.Meter
	counters   map[string]metric.Int64Counter
	histograms map[string]metric.Float64Histogram
}

// NewServerTracer 创建 Hertz 服务端 OTel Tracer，返回 server.Option。
func NewServerTracer(cfg config.ObservabilityConfig) (tracer.Tracer, *TracerConfig) {
	meter := otel.GetMeterProvider().Meter(instrumentationName)

	counters := make(map[string]metric.Int64Counter)
	histograms := make(map[string]metric.Float64Histogram)

	serverRequestCount, err := meter.Int64Counter(
		ServerRequestCount,
		metric.WithUnit("count"),
		metric.WithDescription("measures incoming request count total"),
	)
	handleInitErr(err)

	serverLatency, err := meter.Float64Histogram(
		ServerLatency,
		metric.WithUnit("ms"),
		metric.WithDescription("measures the incoming end to end duration"),
	)
	handleInitErr(err)

	counters[ServerRequestCount] = serverRequestCount
	histograms[ServerLatency] = serverLatency

	otelTracer := otel.GetTracerProvider().Tracer(instrumentationName)

	st := &serverTracer{
		cfg:        cfg,
		tracer:     otelTracer,
		meter:      meter,
		counters:   counters,
		histograms: histograms,
	}

	return st, &TracerConfig{}
}

// TracerConfig 预留未来扩展。
type TracerConfig struct{}

// Start 在 HTTP 请求开始时创建 TraceCarrier。
func (s *serverTracer) Start(ctx context.Context, c *app.RequestContext) context.Context {
	tc := &traceCarrier{}
	tc.SetTracer(s.tracer)
	return withTraceCarrier(ctx, tc)
}

// Finish 在 HTTP 请求结束时记录 span 属性和 HTTP metrics。
func (s *serverTracer) Finish(ctx context.Context, c *app.RequestContext) {
	tc := traceCarrierFromContext(ctx)
	if tc == nil {
		return
	}

	ti := c.GetTraceInfo()
	st := ti.Stats()

	if st.Level() == stats.LevelDisabled {
		return
	}

	httpStart := st.GetEvent(stats.HTTPStart)
	if httpStart == nil {
		return
	}

	elapsedTime := float64(st.GetEvent(stats.HTTPFinish).Time().Sub(httpStart.Time())) / float64(time.Millisecond)

	span := tc.Span()
	if span == nil || !span.IsRecording() {
		return
	}

	// 设置 OTel HTTP semconv 属性（v1.36.0 函数式 API）
	httpReq, _ := adaptor.GetCompatRequest(c.GetRequest()) //nolint:staticcheck // GetCompatRequest still works

	span.SetAttributes(
		semconv.URLFull(c.URI().String()),
		semconv.ClientAddress(c.ClientIP()),
		semconv.HTTPResponseStatusCode(c.Response.StatusCode()),
		semconv.HTTPRoute(c.FullPath()),
		semconv.HTTPRequestMethodKey.String(string(c.Request.Method())),
		semconv.ServerAddress(string(c.Request.Host())),
	)

	// 从 http.Request 补充 OTel 标准属性
	if httpReq != nil {
		span.SetAttributes(
			attribute.String("http.scheme", httpReq.URL.Scheme),
			attribute.String("http.user_agent", httpReq.UserAgent()),
		)
	}

	// 设置 span 状态
	if c.Response.StatusCode() >= 400 {
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", c.Response.StatusCode()))
	}

	// 注入 stats 事件到 span
	injectStatsEventsToSpan(span, st)

	// 错误处理
	if panicMsg, httpErr := parseHTTPError(ti); httpErr != nil || panicMsg != "" {
		recordHTTPErrorSpan(span, httpErr, panicMsg)
	}

	// 提取 metrics 属性（在 span.End() 前）
	metricsAttrs := extractMetricsAttributesFromSpan(span)

	span.End(oteltrace.WithTimestamp(getEndTimeOrNow(ti)))

	// 记录 HTTP metrics
	s.counters[ServerRequestCount].Add(ctx, 1, metric.WithAttributes(metricsAttrs...))
	s.histograms[ServerLatency].Record(ctx, elapsedTime, metric.WithAttributes(metricsAttrs...))
}

// injectStatsEventsToSpan 将 Hertz stats 事件注入为 span events。
func injectStatsEventsToSpan(span oteltrace.Span, st traceinfo.HTTPStats) {
	events := []struct {
		event stats.Event
		name  string
	}{
		{stats.ReadHeaderStart, "read_header_start"},
		{stats.ReadHeaderFinish, "read_header_finish"},
		{stats.ReadBodyStart, "read_body_start"},
		{stats.ReadBodyFinish, "read_body_finish"},
		{stats.WriteStart, "write_start"},
		{stats.WriteFinish, "write_finish"},
	}

	for _, e := range events {
		ev := st.GetEvent(e.event)
		if ev.IsNil() {
			continue
		}
		span.AddEvent(e.name, oteltrace.WithTimestamp(ev.Time()))
	}
}

// parseHTTPError 从 TraceInfo 中提取错误信息。
func parseHTTPError(ti traceinfo.TraceInfo) (panicMsg string, httpErr error) {
	st := ti.Stats()
	if st == nil {
		return "", nil
	}

	httpErr = st.Error()

	if panicked, panickedVal := st.Panicked(); panicked {
		panicMsg = fmt.Sprintf("%v", panickedVal)
	}

	return
}

// recordHTTPErrorSpan 将 HTTP 错误记录到 span。
// 业务错误码（40000+）不记录，因为 HTTP 请求本身是成功的。
func recordHTTPErrorSpan(span oteltrace.Span, httpErr error, panicMsg string) {
	if httpErr != nil {
		code, _ := goerror.Extract(httpErr)
		if code >= goerror.ProjectCodeMin {
			return
		}
	}

	if panicMsg != "" {
		span.RecordError(fmt.Errorf("panic: %s", panicMsg))
		span.SetAttributes(attribute.String("http.panic", panicMsg))
	}
	if httpErr != nil {
		span.RecordError(httpErr)
		span.SetStatus(codes.Error, httpErr.Error())
	}
	span.SetAttributes(StatusKey.String("Error"))
}

// getEndTimeOrNow 获取 HTTP 结束时间。
func getEndTimeOrNow(ti traceinfo.TraceInfo) time.Time {
	st := ti.Stats()
	if st == nil {
		return time.Now()
	}
	e := st.GetEvent(stats.HTTPFinish)
	if e == nil || e.IsNil() {
		return time.Now()
	}
	return e.Time()
}

func handleInitErr(_ error) {}
