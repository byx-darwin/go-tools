package observability

import (
	"context"
	"fmt"
	"time"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/stats"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/byx-darwin/go-tools/go-framework/config"
)

// instrumentationName OTel instrumentation 标识，用于 Meter 和 Tracer 命名。
const instrumentationName = "github.com/byx-darwin/go-tools/go-framework/kitex/observability"

var _ stats.Tracer = (*serverTracer)(nil)
var _ stats.Tracer = (*clientTracer)(nil)

// serverTracer 实现 Kitex stats.Tracer 接口，在 RPC 生命周期中记录 span 和 metrics。
type serverTracer struct {
	cfg            config.ObservabilityConfig
	tracer         oteltrace.Tracer
	recordSourceOp bool
}

// Start 在 RPC 开始时创建 TraceCarrier 并注入 context。
func (s *serverTracer) Start(ctx context.Context) context.Context {
	tc := &traceCarrier{}
	tc.SetTracer(s.tracer)
	return withTraceCarrier(ctx, tc)
}

// Finish 在 RPC 结束时记录 span 属性和 RPC duration metrics。
func (s *serverTracer) Finish(ctx context.Context) {
	tc := traceCarrierFromContext(ctx)
	if tc == nil {
		return
	}

	ri := rpcinfo.GetRPCInfo(ctx)
	if ri == nil || ri.Stats().Level() == stats.LevelDisabled {
		return
	}

	st := ri.Stats()
	rpcStart := st.GetEvent(stats.RPCStart)
	rpcFinish := st.GetEvent(stats.RPCFinish)
	if rpcStart.IsNil() || rpcFinish.IsNil() {
		return
	}

	duration := float64(rpcFinish.Time().Sub(rpcStart.Time())) / float64(time.Millisecond)

	span := tc.Span()
	if span == nil || !span.IsRecording() {
		return
	}

	// 记录 RPC 属性
	attrs := []attribute.KeyValue{
		RPCSystemKitex,
		semconv.RPCMethodKey.String(ri.To().Method()),
		semconv.RPCServiceKey.String(ri.To().ServiceName()),
		RPCSystemKitexRecvSize.Int64(int64(st.RecvSize())),
		RPCSystemKitexSendSize.Int64(int64(st.SendSize())),
		RequestProtocolKey.String(ri.Config().TransportProtocol().String()),
	}

	// 调用方操作名（可能造成高基数，可选开启）
	if s.recordSourceOp {
		attrs = append(attrs, SourceOperationKey.String(ri.From().Method()))
	}

	span.SetAttributes(attrs...)

	// 注入 stats 事件到 span（如 read_start, write_finish 等）
	injectStatsEventsToSpan(span, st)

	// 错误处理
	if err, panicMsg := parseRPCError(ri); err != nil || panicMsg != "" {
		recordErrorSpanWithStack(span, err, panicMsg)
	}

	span.End(oteltrace.WithTimestamp(getEndTimeOrNow(ri)))

	// 记录 metrics（如果 MeterProvider 已初始化）
	meter := otel.GetMeterProvider().Meter(instrumentationName)
	serverDuration, err := meter.Float64Histogram(ServerDuration)
	if err == nil {
		metricAttrs := extractMetricsAttributes(span)
		serverDuration.Record(ctx, duration, metric.WithAttributes(metricAttrs...))
	}
}

// clientTracer 实现 Kitex stats.Tracer 接口，在客户端 RPC 生命周期中记录 span 和 metrics。
type clientTracer struct {
	cfg            config.ObservabilityConfig
	tracer         oteltrace.Tracer
	recordSourceOp bool
}

// Start 在客户端 RPC 开始时创建 TraceCarrier。
func (c *clientTracer) Start(ctx context.Context) context.Context {
	tc := &traceCarrier{}
	tc.SetTracer(c.tracer)
	return withTraceCarrier(ctx, tc)
}

// Finish 在客户端 RPC 结束时记录 span 属性和 RPC duration metrics。
func (c *clientTracer) Finish(ctx context.Context) {
	tc := traceCarrierFromContext(ctx)
	if tc == nil {
		return
	}

	ri := rpcinfo.GetRPCInfo(ctx)
	if ri == nil || ri.Stats().Level() == stats.LevelDisabled {
		return
	}

	st := ri.Stats()
	rpcStart := st.GetEvent(stats.RPCStart)
	rpcFinish := st.GetEvent(stats.RPCFinish)
	if rpcStart.IsNil() || rpcFinish.IsNil() {
		return
	}

	duration := float64(rpcFinish.Time().Sub(rpcStart.Time())) / float64(time.Millisecond)

	span := tc.Span()
	if span == nil || !span.IsRecording() {
		return
	}

	attrs := []attribute.KeyValue{
		RPCSystemKitex,
		semconv.RPCMethodKey.String(ri.To().Method()),
		semconv.RPCServiceKey.String(ri.To().ServiceName()),
		RequestProtocolKey.String(ri.Config().TransportProtocol().String()),
	}

	if c.recordSourceOp {
		attrs = append(attrs, SourceOperationKey.String(ri.From().Method()))
	}

	span.SetAttributes(attrs...)

	injectStatsEventsToSpan(span, st)

	if err, panicMsg := parseRPCError(ri); err != nil || panicMsg != "" {
		recordErrorSpanWithStack(span, err, panicMsg)
	}

	span.End(oteltrace.WithTimestamp(getEndTimeOrNow(ri)))

	// 记录 metrics（如果 MeterProvider 已初始化）
	meter := otel.GetMeterProvider().Meter(instrumentationName)
	if clientDuration, err := meter.Float64Histogram(ClientDuration); err == nil {
		metricAttrs := extractMetricsAttributes(span)
		clientDuration.Record(ctx, duration, metric.WithAttributes(metricAttrs...))
	}
}

// injectStatsEventsToSpan 将 Kitex stats 事件作为 span events 注入。
func injectStatsEventsToSpan(span oteltrace.Span, st rpcinfo.RPCStats) {
	events := []struct {
		event stats.Event
		name  string
	}{
		{stats.ReadStart, "read_start"},
		{stats.ReadFinish, "read_finish"},
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

// parseRPCError 从 RPCInfo 中提取错误信息。
func parseRPCError(ri rpcinfo.RPCInfo) (rpcErr error, panicMsg string) {
	st := ri.Stats()
	if st == nil {
		return nil, ""
	}

	rpcErr = st.Error()

	if panicked, panickedVal := st.Panicked(); panicked {
		panicMsg = fmt.Sprintf("%v", panickedVal)
	}

	return
}

// recordErrorSpanWithStack 将 RPC 错误记录到 span。
// 业务错误码（40000+）不记录，因为 RPC 调用本身是成功的。
func recordErrorSpanWithStack(span oteltrace.Span, rpcErr error, panicMsg string) {
	code, _ := goerror.Extract(rpcErr)
	if code >= goerror.ProjectCodeMin {
		// 业务错误，RPC 成功，不记录为 span error
		return
	}

	if panicMsg != "" {
		span.RecordError(fmt.Errorf("panic: %s", panicMsg))
		span.SetAttributes(attribute.String("rpc.panic", panicMsg))
	}
	if rpcErr != nil {
		span.RecordError(rpcErr)
	}
	span.SetAttributes(StatusKey.String("Error"))
}

// getEndTimeOrNow 获取 RPC 结束时间，若无法获取则返回当前时间。
func getEndTimeOrNow(ri rpcinfo.RPCInfo) time.Time {
	if ri == nil {
		return time.Now()
	}
	st := ri.Stats()
	if st == nil {
		return time.Now()
	}
	e := st.GetEvent(stats.RPCFinish)
	if e.IsNil() {
		return time.Now()
	}
	return e.Time()
}
