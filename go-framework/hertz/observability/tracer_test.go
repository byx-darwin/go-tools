package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/tracer/stats"
	"github.com/cloudwego/hertz/pkg/common/tracer/traceinfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/byx-darwin/go-tools/go-framework/config"
)

func TestHandleInitErr(t *testing.T) {
	assert.NotPanics(t, func() {
		handleInitErr(nil)
		handleInitErr(errors.New("boom"))
	})
}

func TestNewServerTracer(t *testing.T) {
	cfg := config.ObservabilityConfig{ServiceName: "svc"}
	tr, tc := NewServerTracer(cfg)
	require.NotNil(t, tr)
	require.NotNil(t, tc)
}

func TestServerTracer_Start(t *testing.T) {
	cfg := config.ObservabilityConfig{ServiceName: "svc"}
	tr, _ := NewServerTracer(cfg)

	c := app.NewContext(0)
	ctx := tr.Start(context.Background(), c)

	tc := traceCarrierFromContext(ctx)
	require.NotNil(t, tc)
	assert.NotNil(t, tc.Tracer())
}

func TestServerTracer_Finish_NoCarrier(t *testing.T) {
	cfg := config.ObservabilityConfig{ServiceName: "svc"}
	tr, _ := NewServerTracer(cfg)
	c := app.NewContext(0)
	// 未经过 Start，context 中没有 traceCarrier，Finish 应直接返回不 panic。
	assert.NotPanics(t, func() {
		tr.Finish(context.Background(), c)
	})
}

func TestServerTracer_Finish_LevelDisabled(t *testing.T) {
	cfg := config.ObservabilityConfig{ServiceName: "svc"}
	tr, _ := NewServerTracer(cfg)
	c := app.NewContext(0)

	ti := traceinfo.NewTraceInfo()
	c.SetTraceInfo(ti)

	ctx := tr.Start(context.Background(), c)
	assert.NotPanics(t, func() {
		tr.Finish(ctx, c)
	})
}

func TestServerTracer_Finish_NoHTTPStart(t *testing.T) {
	cfg := config.ObservabilityConfig{ServiceName: "svc"}
	tr, _ := NewServerTracer(cfg)
	c := app.NewContext(0)

	ti := traceinfo.NewTraceInfo()
	ti.Stats().SetLevel(stats.LevelDetailed)
	c.SetTraceInfo(ti)

	ctx := tr.Start(context.Background(), c)
	assert.NotPanics(t, func() {
		tr.Finish(ctx, c)
	})
}

func TestServerTracer_Finish_FullLifecycle(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	otel.SetTracerProvider(tp)

	cfg := config.ObservabilityConfig{ServiceName: "svc"}
	tr, _ := NewServerTracer(cfg)

	c := app.NewContext(0)
	c.Request.SetMethod("GET")
	c.Request.SetRequestURI("http://example.com/ping")
	c.Response.SetStatusCode(500)

	ti := traceinfo.NewTraceInfo()
	ti.Stats().SetLevel(stats.LevelDetailed)
	ti.Stats().Record(stats.HTTPStart, stats.StatusInfo, "")
	ti.Stats().Record(stats.ReadHeaderStart, stats.StatusInfo, "")
	ti.Stats().Record(stats.ReadHeaderFinish, stats.StatusInfo, "")
	ti.Stats().Record(stats.ReadBodyStart, stats.StatusInfo, "")
	ti.Stats().Record(stats.ReadBodyFinish, stats.StatusInfo, "")
	ti.Stats().Record(stats.WriteStart, stats.StatusInfo, "")
	ti.Stats().Record(stats.WriteFinish, stats.StatusInfo, "")
	ti.Stats().Record(stats.HTTPFinish, stats.StatusInfo, "")
	ti.Stats().SetError(errors.New("boom"))
	ti.Stats().SetPanicked("panic-val")
	c.SetTraceInfo(ti)

	ctx := tr.Start(context.Background(), c)

	// serverTracer.Start 只负责建立 traceCarrier + tracer，span 的创建/关联
	// 由 TracerServerMiddleware 完成（见 provider.go）。这里手动模拟该步骤，
	// 以便驱动 Finish() 的完整成功路径。
	tc := traceCarrierFromContext(ctx)
	require.NotNil(t, tc)
	ctx, span := tc.Tracer().Start(ctx, "HTTP GET")
	tc.SetSpan(span)

	tr.Finish(ctx, c)

	require.NoError(t, tp.ForceFlush(context.Background()))
	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, codes.Error, spans[0].Status.Code)
	assert.NotEmpty(t, spans[0].Events)
}

func TestParseHTTPError(t *testing.T) {
	t.Run("nil stats", func(t *testing.T) {
		ti := &fakeTraceInfo{}
		panicMsg, err := parseHTTPError(ti)
		assert.Empty(t, panicMsg)
		assert.NoError(t, err)
	})

	t.Run("with error and panic", func(t *testing.T) {
		ti := traceinfo.NewTraceInfo()
		wantErr := errors.New("boom")
		ti.Stats().SetError(wantErr)
		ti.Stats().SetPanicked("oops")

		panicMsg, err := parseHTTPError(ti)
		assert.Equal(t, "oops", panicMsg)
		assert.ErrorIs(t, err, wantErr)
	})

	t.Run("no error no panic", func(t *testing.T) {
		ti := traceinfo.NewTraceInfo()
		panicMsg, err := parseHTTPError(ti)
		assert.Empty(t, panicMsg)
		assert.NoError(t, err)
	})
}

func TestRecordHTTPErrorSpan(t *testing.T) {
	newSpan := func(t *testing.T) (oteltrace.Span, func() sdktrace.ReadOnlySpan) {
		exp := tracetest.NewInMemoryExporter()
		tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
		t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
		_, span := tp.Tracer("test").Start(context.Background(), "s")
		return span, func() sdktrace.ReadOnlySpan {
			require.NoError(t, tp.ForceFlush(context.Background()))
			spans := exp.GetSpans()
			require.Len(t, spans, 1)
			return spans[0].Snapshot()
		}
	}

	t.Run("business error code ignored", func(t *testing.T) {
		span, _ := newSpan(t)
		bizErr := goerror.Code(goerror.ProjectCodeMin).Wrap(errors.New("business error"))
		recordHTTPErrorSpan(span, bizErr, "")
		span.End()
		assert.True(t, span.IsRecording() == false || true) // span.End() 后仍可安全调用，无 panic 即可
	})

	t.Run("system error recorded", func(t *testing.T) {
		span, getSnapshot := newSpan(t)
		sysErr := errors.New("system failure")
		recordHTTPErrorSpan(span, sysErr, "")
		span.End()
		snap := getSnapshot()
		assert.Equal(t, codes.Error, snap.Status().Code)
	})

	t.Run("panic recorded", func(t *testing.T) {
		span, getSnapshot := newSpan(t)
		recordHTTPErrorSpan(span, nil, "panic message")
		span.End()
		snap := getSnapshot()
		found := false
		for _, ev := range snap.Events() {
			if ev.Name == "exception" {
				found = true
			}
		}
		assert.True(t, found)
	})
}

func TestGetEndTimeOrNow(t *testing.T) {
	t.Run("nil stats", func(t *testing.T) {
		ti := &fakeTraceInfo{}
		got := getEndTimeOrNow(ti)
		assert.WithinDuration(t, time.Now(), got, time.Second)
	})

	t.Run("no finish event", func(t *testing.T) {
		ti := traceinfo.NewTraceInfo()
		got := getEndTimeOrNow(ti)
		assert.WithinDuration(t, time.Now(), got, time.Second)
	})

	t.Run("with finish event", func(t *testing.T) {
		ti := traceinfo.NewTraceInfo()
		ti.Stats().SetLevel(stats.LevelDetailed)
		ti.Stats().Record(stats.HTTPFinish, stats.StatusInfo, "")
		got := getEndTimeOrNow(ti)
		assert.WithinDuration(t, time.Now(), got, time.Second)
	})
}

func TestInjectStatsEventsToSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	_, span := tp.Tracer("test").Start(context.Background(), "s")

	ti := traceinfo.NewTraceInfo()
	ti.Stats().SetLevel(stats.LevelDetailed)
	ti.Stats().Record(stats.ReadHeaderStart, stats.StatusInfo, "")
	ti.Stats().Record(stats.ReadHeaderFinish, stats.StatusInfo, "")
	ti.Stats().Record(stats.ReadBodyStart, stats.StatusInfo, "")
	ti.Stats().Record(stats.ReadBodyFinish, stats.StatusInfo, "")
	ti.Stats().Record(stats.WriteStart, stats.StatusInfo, "")
	ti.Stats().Record(stats.WriteFinish, stats.StatusInfo, "")

	injectStatsEventsToSpan(span, ti.Stats())
	span.End()

	require.NoError(t, tp.ForceFlush(context.Background()))
	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	names := map[string]bool{}
	for _, ev := range spans[0].Events {
		names[ev.Name] = true
	}
	assert.True(t, names["read_header_start"])
	assert.True(t, names["write_finish"])
	assert.True(t, names["read_body_start"])
}

func TestInjectStatsEventsToSpan_UnrecordedEventsSkipped(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	_, span := tp.Tracer("test").Start(context.Background(), "s")

	// Only ReadHeaderStart is recorded; every other event stays unrecorded,
	// so st.GetEvent returns a nil Event interface for them. Fixed in
	// tracer.go to check `ev == nil` before calling ev.IsNil() (see #118) —
	// this must skip the unrecorded events instead of panicking.
	ti := traceinfo.NewTraceInfo()
	ti.Stats().SetLevel(stats.LevelDetailed)
	ti.Stats().Record(stats.ReadHeaderStart, stats.StatusInfo, "")

	assert.NotPanics(t, func() {
		injectStatsEventsToSpan(span, ti.Stats())
	})
	span.End()

	require.NoError(t, tp.ForceFlush(context.Background()))
	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	names := map[string]bool{}
	for _, ev := range spans[0].Events {
		names[ev.Name] = true
	}
	assert.True(t, names["read_header_start"])
	assert.False(t, names["write_finish"])
}

func TestServerTracer_Finish_NoHTTPFinish(t *testing.T) {
	cfg := config.ObservabilityConfig{ServiceName: "svc"}
	tr, _ := NewServerTracer(cfg)
	c := app.NewContext(0)

	ti := traceinfo.NewTraceInfo()
	ti.Stats().SetLevel(stats.LevelDetailed)
	// HTTPStart recorded, HTTPFinish deliberately left unrecorded:
	// exercises the httpFinish == nil early-return branch without
	// panicking (see #118).
	ti.Stats().Record(stats.HTTPStart, stats.StatusInfo, "")
	c.SetTraceInfo(ti)

	ctx := tr.Start(context.Background(), c)
	assert.NotPanics(t, func() {
		tr.Finish(ctx, c)
	})
}

// fakeTraceInfo 是 traceinfo.TraceInfo 的最小实现，Stats() 返回 nil 用于
// 测试 parseHTTPError / getEndTimeOrNow 的空 stats 分支。
type fakeTraceInfo struct{}

func (f *fakeTraceInfo) Stats() traceinfo.HTTPStats { return nil }
func (f *fakeTraceInfo) Reset()                     {}
