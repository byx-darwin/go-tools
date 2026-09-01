package tls

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestSpanLinks_Deduplicates(t *testing.T) {
	sc1 := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1},
		SpanID:     trace.SpanID{1},
		TraceFlags: trace.FlagsSampled,
	})
	sc2 := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{2},
		SpanID:     trace.SpanID{2},
		TraceFlags: trace.FlagsSampled,
	})

	links := spanLinks([]trace.SpanContext{sc1, sc1, sc2})

	require.Len(t, links, 2)
	require.Equal(t, sc1, links[0].SpanContext)
	require.Equal(t, sc2, links[1].SpanContext)
}

func TestSpanLinks_EmptyInput(t *testing.T) {
	links := spanLinks(nil)
	require.Empty(t, links)
}

func attrsToMap(attrs []attribute.KeyValue) map[string]any {
	m := make(map[string]any, len(attrs))
	for _, a := range attrs {
		m[string(a.Key)] = a.Value.AsInterface()
	}
	return m
}

func newTestProducer(t *testing.T, opts ...ProducerOption) *Producer {
	t.Helper()
	p, err := NewProducer(ProducerConfig{
		Endpoint:        "tls.example.com", // 不可达域名，flush 一定失败，仅验证 span 行为
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
		Region:          "cn-beijing",
		TopicID:         "topic-123",
		BatchSize:       1, // 每次 SendLog 立即触发 flush
		FlushInterval:   time.Hour,
	}, opts...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func TestProducer_Flush_TracesSpanWithAttributesOnFailure(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	p := newTestProducer(t, WithTrace())
	p.tracer = tp.Tracer(instrumentationName)

	err := p.SendLog(context.Background(), map[string]string{"level": "info"})
	require.Error(t, err) // fake endpoint: 网络必然失败

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "tls.flush", spans[0].Name)
	assert.Equal(t, codes.Error, spans[0].Status.Code)

	attrs := attrsToMap(spans[0].Attributes)
	assert.Equal(t, "topic-123", attrs["tls.topic_id"])
	assert.EqualValues(t, 1, attrs["tls.batch_size"])
}

func TestProducer_Flush_NoTraceProducesNoSpans(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	p := newTestProducer(t) // 未启用 WithTrace，p.tracer 为 nil

	_ = p.SendLog(context.Background(), map[string]string{"level": "info"})

	assert.Empty(t, exporter.GetSpans(), "tracing 未启用时不应产生任何 span")
}

func TestProducer_SendLog_LinksCallerSpanToFlush(t *testing.T) {
	callerExporter := tracetest.NewInMemoryExporter()
	callerTP := sdktrace.NewTracerProvider(sdktrace.WithSyncer(callerExporter))
	t.Cleanup(func() { _ = callerTP.Shutdown(context.Background()) })
	callerCtx, callerSpan := callerTP.Tracer("caller").Start(context.Background(), "handle-request")

	flushExporter := tracetest.NewInMemoryExporter()
	flushTP := sdktrace.NewTracerProvider(sdktrace.WithSyncer(flushExporter))
	t.Cleanup(func() { _ = flushTP.Shutdown(context.Background()) })

	p := newTestProducer(t, WithTrace())
	p.tracer = flushTP.Tracer(instrumentationName)

	_ = p.SendLog(callerCtx, map[string]string{"level": "info"})
	callerSpan.End()

	spans := flushExporter.GetSpans()
	require.Len(t, spans, 1)
	require.Len(t, spans[0].Links, 1)
	assert.Equal(t, callerSpan.SpanContext().TraceID(), spans[0].Links[0].SpanContext.TraceID())
	assert.Equal(t, callerSpan.SpanContext().SpanID(), spans[0].Links[0].SpanContext.SpanID())
}

func TestProducer_SendLog_NoSpanContextProducesNoLinks(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	p := newTestProducer(t, WithTrace())
	p.tracer = tp.Tracer(instrumentationName)

	// 模拟 flushLoop / FileShipper 场景：ctx 不携带业务 span。
	_ = p.SendLog(context.Background(), map[string]string{"level": "info"})

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Empty(t, spans[0].Links)
}
