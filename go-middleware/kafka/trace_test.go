package kafka

import (
	"context"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func init() {
	// 生产环境由 go-framework 的 Provider 初始化时注册；测试里手动注册一份，
	// 否则 otel.GetTextMapPropagator() 默认是 no-op，Inject/Extract 都不会写入数据。
	otel.SetTextMapPropagator(propagation.TraceContext{})
}

func attrsToMap(attrs []attribute.KeyValue) map[string]any {
	m := make(map[string]any, len(attrs))
	for _, a := range attrs {
		m[string(a.Key)] = a.Value.AsInterface()
	}
	return m
}

func TestHeaderCarrier_SetGetKeys(t *testing.T) {
	var headers []kafka.Header
	c := headerCarrier{headers: &headers}

	c.Set("traceparent", "00-a-01")
	assert.Equal(t, "00-a-01", c.Get("traceparent"))
	assert.Equal(t, []string{"traceparent"}, c.Keys())

	// 覆盖已存在的 key，不应追加新条目。
	c.Set("traceparent", "00-a-02")
	assert.Len(t, headers, 1)
	assert.Equal(t, "00-a-02", c.Get("traceparent"))

	c.Set("baggage", "k=v")
	assert.ElementsMatch(t, []string{"traceparent", "baggage"}, c.Keys())
	assert.Empty(t, c.Get("missing"))
}

func TestWriter_WriteMessages_TraceInjectsHeadersOnFailure(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	w := NewWriter(WriterConfig{
		Broker: []string{"127.0.0.1:1"}, // 不可达地址，WriteMessages 一定失败
		Topic:  "test-topic",
	}, WithTrace())
	w.tracer = tp.Tracer(instrumentationName)
	defer func() { _ = w.Close() }()

	msgs := []kafka.Message{{Key: []byte("k"), Value: []byte("v")}}
	err := w.WriteMessages(context.Background(), msgs...)
	require.Error(t, err)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "kafka.send", spans[0].Name)
	assert.Equal(t, codes.Error, spans[0].Status.Code)

	// inject 发生在实际网络调用之前，即使发送失败，Headers 里也应该已经写入了
	// traceparent（msgs 与内部处理共享同一个底层数组，能在调用后观察到变更）。
	found := false
	for _, h := range msgs[0].Headers {
		if h.Key == "traceparent" {
			found = true
		}
	}
	assert.True(t, found, "expected traceparent header to be injected")
}

func TestWriter_WriteMessages_NoTraceProducesNoSpans(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	w := NewWriter(WriterConfig{
		Broker: []string{"127.0.0.1:1"},
		Topic:  "test-topic",
	}) // 未开启 WithTrace，w.tracer 为 nil
	defer func() { _ = w.Close() }()

	_ = w.WriteMessages(context.Background(), kafka.Message{Key: []byte("k")})

	assert.Empty(t, exporter.GetSpans(), "no spans should be produced when tracing is disabled")
}

func TestConsumer_TraceReceive_LinksProducerContext(t *testing.T) {
	producerExporter := tracetest.NewInMemoryExporter()
	producerTP := sdktrace.NewTracerProvider(sdktrace.WithSyncer(producerExporter))
	t.Cleanup(func() { _ = producerTP.Shutdown(context.Background()) })

	producerCtx, producerSpan := producerTP.Tracer("producer").Start(context.Background(), "produce")
	var headers []kafka.Header
	injectTraceContext(producerCtx, &headers)
	producerSpan.End()

	consumerExporter := tracetest.NewInMemoryExporter()
	consumerTP := sdktrace.NewTracerProvider(sdktrace.WithSyncer(consumerExporter))
	t.Cleanup(func() { _ = consumerTP.Shutdown(context.Background()) })

	c := &Consumer{tracer: consumerTP.Tracer(instrumentationName)}
	msg := kafka.Message{Topic: "t", Partition: 3, Offset: 42, Headers: headers}
	c.traceReceive(context.Background(), "ReadMessage", msg)

	spans := consumerExporter.GetSpans()
	require.Len(t, spans, 1)
	span := spans[0]
	assert.Equal(t, "kafka.ReadMessage", span.Name)

	require.Len(t, span.Links, 1)
	assert.Equal(t, producerSpan.SpanContext().TraceID(), span.Links[0].SpanContext.TraceID())
	assert.Equal(t, producerSpan.SpanContext().SpanID(), span.Links[0].SpanContext.SpanID())

	attrs := attrsToMap(span.Attributes)
	assert.Equal(t, "kafka", attrs["messaging.system"])
	assert.Equal(t, "t", attrs["messaging.destination"])
	assert.EqualValues(t, 3, attrs["messaging.kafka.partition"])
	assert.EqualValues(t, 42, attrs["messaging.kafka.offset"])
}

func TestConsumer_TraceReceive_NoTracerIsNoop(t *testing.T) {
	c := &Consumer{}
	// tracer == nil：不应 panic，直接返回。
	c.traceReceive(context.Background(), "ReadMessage", kafka.Message{Topic: "t"})
}
