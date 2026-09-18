package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/cloudwego/kitex/pkg/kerrors"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/stats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/byx-darwin/go-tools/go-framework/config"
)

// newTestRPCInfo builds a minimal rpcinfo.RPCInfo with RPCStart/RPCFinish
// events recorded, at the given stats level, for exercising
// serverTracer/clientTracer.Finish.
func newTestRPCInfo(t *testing.T, level stats.Level) rpcinfo.RPCInfo {
	t.Helper()
	from := rpcinfo.NewEndpointInfo("caller-service", "CallerMethod", nil, nil)
	to := rpcinfo.NewEndpointInfo("callee-service", "CalleeMethod", nil, nil)
	ink := rpcinfo.NewInvocation("callee-service", "CalleeMethod")
	cfg := rpcinfo.NewRPCConfig()
	st := rpcinfo.NewRPCStats()

	mst := rpcinfo.AsMutableRPCStats(st)
	require.NotNil(t, mst)
	mst.SetLevel(level)

	ri := rpcinfo.NewRPCInfo(from, to, ink, cfg, st)

	st.Record(context.Background(), stats.RPCStart, stats.StatusInfo, "")
	time.Sleep(time.Millisecond)
	st.Record(context.Background(), stats.RPCFinish, stats.StatusInfo, "")
	st.Record(context.Background(), stats.ReadStart, stats.StatusInfo, "")
	st.Record(context.Background(), stats.ReadFinish, stats.StatusInfo, "")
	st.Record(context.Background(), stats.WriteStart, stats.StatusInfo, "")
	st.Record(context.Background(), stats.WriteFinish, stats.StatusInfo, "")

	return ri
}

func TestServerTracer_StartFinish_Success(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	tracer := tp.Tracer("test")

	st := &serverTracer{cfg: config.ObservabilityConfig{}, tracer: tracer, recordSourceOp: true}

	ctx := st.Start(context.Background())
	_, span := tracer.Start(ctx, "rpc.server")
	tc := traceCarrierFromContext(ctx)
	require.NotNil(t, tc)
	tc.SetSpan(span)

	ri := newTestRPCInfo(t, stats.LevelDetailed)
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)

	st.Finish(ctx)

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "rpc.server", spans[0].Name)

	attrMap := make(map[attribute.Key]string)
	for _, a := range spans[0].Attributes {
		attrMap[a.Key] = a.Value.AsString()
	}
	assert.Equal(t, "CalleeMethod", attrMap["rpc.method"])
	assert.Equal(t, "callee-service", attrMap["rpc.service"])
	assert.Equal(t, "CallerMethod", attrMap[SourceOperationKey])

	// Stats events injected as span events.
	names := make([]string, 0, len(spans[0].Events))
	for _, e := range spans[0].Events {
		names = append(names, e.Name)
	}
	assert.Contains(t, names, "read_start")
	assert.Contains(t, names, "write_finish")
}

func TestServerTracer_Finish_NoTraceCarrier(t *testing.T) {
	st := &serverTracer{cfg: config.ObservabilityConfig{}}
	// No traceCarrier in context: must return without panicking.
	st.Finish(context.Background())
}

func TestServerTracer_Finish_NoRPCInfo(t *testing.T) {
	st := &serverTracer{cfg: config.ObservabilityConfig{}, tracer: noop.NewTracerProvider().Tracer("noop")}
	ctx := st.Start(context.Background())
	// No RPCInfo attached: ri == nil branch.
	st.Finish(ctx)
}

func TestServerTracer_Finish_StatsDisabled(t *testing.T) {
	st := &serverTracer{cfg: config.ObservabilityConfig{}, tracer: noop.NewTracerProvider().Tracer("noop")}
	ctx := st.Start(context.Background())

	ri := newTestRPCInfo(t, stats.LevelDisabled)
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)

	// stats level disabled: Finish returns early.
	st.Finish(ctx)
}

func TestServerTracer_Finish_SpanNotSet(t *testing.T) {
	st := &serverTracer{cfg: config.ObservabilityConfig{}, tracer: noop.NewTracerProvider().Tracer("noop")}
	ctx := st.Start(context.Background())

	ri := newTestRPCInfo(t, stats.LevelDetailed)
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)

	// traceCarrier has no span set: tc.Span() == nil branch.
	st.Finish(ctx)
}

func TestServerTracer_Finish_WithFrameworkError(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	tracer := tp.Tracer("test")

	st := &serverTracer{cfg: config.ObservabilityConfig{}, tracer: tracer}
	ctx := st.Start(context.Background())
	_, span := tracer.Start(ctx, "rpc.server")
	tc := traceCarrierFromContext(ctx)
	tc.SetSpan(span)

	from := rpcinfo.NewEndpointInfo("caller", "M", nil, nil)
	to := rpcinfo.NewEndpointInfo("callee", "M", nil, nil)
	ink := rpcinfo.NewInvocation("callee", "M")
	cfg := rpcinfo.NewRPCConfig()
	rst := rpcinfo.NewRPCStats()
	mst := rpcinfo.AsMutableRPCStats(rst)
	require.NotNil(t, mst)
	mst.SetLevel(stats.LevelDetailed)
	mst.SetError(kerrors.ErrRPCTimeout)
	mst.SetPanicked("boom")
	ri := rpcinfo.NewRPCInfo(from, to, ink, cfg, rst)
	rst.Record(context.Background(), stats.RPCStart, stats.StatusInfo, "")
	rst.Record(context.Background(), stats.RPCFinish, stats.StatusInfo, "")
	rst.Record(context.Background(), stats.ReadStart, stats.StatusInfo, "")
	rst.Record(context.Background(), stats.ReadFinish, stats.StatusInfo, "")
	rst.Record(context.Background(), stats.WriteStart, stats.StatusInfo, "")
	rst.Record(context.Background(), stats.WriteFinish, stats.StatusInfo, "")

	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	st.Finish(ctx)

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	assert.NotEmpty(t, spans[0].Events)

	var hasError bool
	for _, e := range spans[0].Events {
		if e.Name == "exception" {
			hasError = true
		}
	}
	assert.True(t, hasError)
}

func TestServerTracer_Finish_BusinessErrorSkipsSpanError(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	tracer := tp.Tracer("test")

	st := &serverTracer{cfg: config.ObservabilityConfig{}, tracer: tracer}
	ctx := st.Start(context.Background())
	_, span := tracer.Start(ctx, "rpc.server")
	tc := traceCarrierFromContext(ctx)
	tc.SetSpan(span)

	from := rpcinfo.NewEndpointInfo("caller", "M", nil, nil)
	to := rpcinfo.NewEndpointInfo("callee", "M", nil, nil)
	ink := rpcinfo.NewInvocation("callee", "M")
	cfg := rpcinfo.NewRPCConfig()
	rst := rpcinfo.NewRPCStats()
	mst := rpcinfo.AsMutableRPCStats(rst)
	require.NotNil(t, mst)
	mst.SetLevel(stats.LevelDetailed)
	bizErr := goerror.Code(goerror.ProjectCodeMin + 1).Public("biz_error").Wrap(errors.New("business failure"))
	mst.SetError(bizErr)
	ri := rpcinfo.NewRPCInfo(from, to, ink, cfg, rst)
	rst.Record(context.Background(), stats.RPCStart, stats.StatusInfo, "")
	rst.Record(context.Background(), stats.RPCFinish, stats.StatusInfo, "")
	rst.Record(context.Background(), stats.ReadStart, stats.StatusInfo, "")
	rst.Record(context.Background(), stats.ReadFinish, stats.StatusInfo, "")
	rst.Record(context.Background(), stats.WriteStart, stats.StatusInfo, "")
	rst.Record(context.Background(), stats.WriteFinish, stats.StatusInfo, "")

	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	st.Finish(ctx)

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	// Business error (code >= ProjectCodeMin) must not be recorded as span error.
	for _, e := range spans[0].Events {
		assert.NotEqual(t, "exception", e.Name)
	}
}

func TestClientTracer_StartFinish_Success(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	tracer := tp.Tracer("test")

	ct := &clientTracer{cfg: config.ObservabilityConfig{}, tracer: tracer, recordSourceOp: true}

	ctx := ct.Start(context.Background())
	_, span := tracer.Start(ctx, "rpc.client")
	tc := traceCarrierFromContext(ctx)
	require.NotNil(t, tc)
	tc.SetSpan(span)

	ri := newTestRPCInfo(t, stats.LevelDetailed)
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)

	ct.Finish(ctx)

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "rpc.client", spans[0].Name)
}

func TestClientTracer_Finish_NoTraceCarrier(t *testing.T) {
	ct := &clientTracer{cfg: config.ObservabilityConfig{}}
	ct.Finish(context.Background())
}

func TestParseRPCError(t *testing.T) {
	from := rpcinfo.NewEndpointInfo("caller", "M", nil, nil)
	to := rpcinfo.NewEndpointInfo("callee", "M", nil, nil)
	ink := rpcinfo.NewInvocation("callee", "M")
	cfg := rpcinfo.NewRPCConfig()
	rst := rpcinfo.NewRPCStats()
	ri := rpcinfo.NewRPCInfo(from, to, ink, cfg, rst)

	panicMsg, err := parseRPCError(ri)
	assert.Empty(t, panicMsg)
	assert.NoError(t, err)

	mst := rpcinfo.AsMutableRPCStats(rst)
	require.NotNil(t, mst)
	mst.SetError(errors.New("boom"))
	mst.SetPanicked("panic-val")

	panicMsg, err = parseRPCError(ri)
	assert.Equal(t, "panic-val", panicMsg)
	assert.Error(t, err)
}

func TestGetEndTimeOrNow(t *testing.T) {
	assert.False(t, getEndTimeOrNow(nil).IsZero())

	from := rpcinfo.NewEndpointInfo("caller", "M", nil, nil)
	to := rpcinfo.NewEndpointInfo("callee", "M", nil, nil)
	ink := rpcinfo.NewInvocation("callee", "M")
	cfg := rpcinfo.NewRPCConfig()
	rst := rpcinfo.NewRPCStats()
	mst := rpcinfo.AsMutableRPCStats(rst)
	require.NotNil(t, mst)
	mst.SetLevel(stats.LevelBase)
	ri := rpcinfo.NewRPCInfo(from, to, ink, cfg, rst)

	// RPCFinish never recorded: rpcinfo's GetEvent returns a plain nil
	// interface for an unrecorded event. Fixed in tracer.go to check
	// `e == nil` before calling e.IsNil(), so this must fall back to
	// time.Now() instead of panicking (see Issue #118).
	assert.False(t, getEndTimeOrNow(ri).IsZero())

	rst.Record(context.Background(), stats.RPCFinish, stats.StatusInfo, "")
	assert.False(t, getEndTimeOrNow(ri).IsZero())
}

func TestInjectStatsEventsToSpan_UnrecordedEventsSkipped(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	tracer := tp.Tracer("test")

	rst := rpcinfo.NewRPCStats()
	mst := rpcinfo.AsMutableRPCStats(rst)
	require.NotNil(t, mst)
	mst.SetLevel(stats.LevelDetailed)
	// Only ReadStart is recorded; ReadFinish/WriteStart/WriteFinish stay
	// unrecorded so GetEvent returns a nil interface for them (see #118).
	rst.Record(context.Background(), stats.ReadStart, stats.StatusInfo, "")

	_, span := tracer.Start(context.Background(), "test-span")
	assert.NotPanics(t, func() {
		injectStatsEventsToSpan(span, rst)
	})
	span.End()

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	names := make([]string, 0, len(spans[0].Events))
	for _, e := range spans[0].Events {
		names = append(names, e.Name)
	}
	assert.Contains(t, names, "read_start")
	assert.NotContains(t, names, "read_finish")
}

func TestServerClientTracer_Finish_EventsUnrecorded(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	tracer := tp.Tracer("test")

	from := rpcinfo.NewEndpointInfo("caller-service", "CallerMethod", nil, nil)
	to := rpcinfo.NewEndpointInfo("callee-service", "CalleeMethod", nil, nil)
	ink := rpcinfo.NewInvocation("callee-service", "CalleeMethod")
	cfg := rpcinfo.NewRPCConfig()
	rst := rpcinfo.NewRPCStats()
	mst := rpcinfo.AsMutableRPCStats(rst)
	require.NotNil(t, mst)
	mst.SetLevel(stats.LevelDetailed)
	// RPCStart recorded, RPCFinish deliberately left unrecorded: exercises
	// the rpcFinish == nil early-return branch without panicking (#118).
	rst.Record(context.Background(), stats.RPCStart, stats.StatusInfo, "")
	ri := rpcinfo.NewRPCInfo(from, to, ink, cfg, rst)

	sTracer := &serverTracer{cfg: config.ObservabilityConfig{}, tracer: tracer}
	ctx := sTracer.Start(context.Background())
	_, span := tracer.Start(ctx, "rpc.server")
	tc := traceCarrierFromContext(ctx)
	require.NotNil(t, tc)
	tc.SetSpan(span)
	ctx = rpcinfo.NewCtxWithRPCInfo(ctx, ri)
	assert.NotPanics(t, func() { sTracer.Finish(ctx) })

	cTracer := &clientTracer{cfg: config.ObservabilityConfig{}, tracer: tracer}
	cctx := cTracer.Start(context.Background())
	_, cspan := tracer.Start(cctx, "rpc.client")
	ctc := traceCarrierFromContext(cctx)
	require.NotNil(t, ctc)
	ctc.SetSpan(cspan)
	cctx = rpcinfo.NewCtxWithRPCInfo(cctx, ri)
	assert.NotPanics(t, func() { cTracer.Finish(cctx) })

	// Neither Finish call should have produced a span (early return before
	// span.End()), since RPCFinish was never recorded.
	assert.Empty(t, exp.GetSpans())
}

func TestRecordErrorSpanWithStack(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	tracer := tp.Tracer("test")

	_, span := tracer.Start(context.Background(), "test-span")
	recordErrorSpanWithStack(span, errors.New("plain failure"), "panic-info")
	span.End()

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	assert.NotEmpty(t, spans[0].Events)
}

func TestRecordErrorSpanWithStack_BusinessErrorSkipped(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	tracer := tp.Tracer("test")

	_, span := tracer.Start(context.Background(), "test-span")
	bizErr := goerror.Code(goerror.ProjectCodeMin + 5).Public("biz").Wrap(errors.New("business"))
	recordErrorSpanWithStack(span, bizErr, "")
	span.End()

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	assert.Empty(t, spans[0].Events)
}
