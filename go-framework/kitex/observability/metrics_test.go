package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestMatchAttributeKey(t *testing.T) {
	keys := []attribute.Key{semconv.RPCServiceKey, semconv.RPCMethodKey}
	assert.True(t, matchAttributeKey(semconv.RPCServiceKey, keys))
	assert.False(t, matchAttributeKey(attribute.Key("unrelated"), keys))
}

func TestExtractMetricsAttributes(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	tracer := tp.Tracer("test")

	_, span := tracer.Start(context.Background(), "test-span")
	span.SetAttributes(
		semconv.RPCServiceKey.String("demo-service"),
		semconv.PeerServiceKey.String("peer-service"),
		attribute.String("unrelated.key", "ignored"),
	)
	span.End()

	attrs := extractMetricsAttributes(span)
	assert.NotEmpty(t, attrs)

	attrMap := make(map[attribute.Key]string)
	for _, a := range attrs {
		attrMap[a.Key] = a.Value.AsString()
	}
	assert.Equal(t, "demo-service", attrMap[semconv.RPCServiceKey])
	assert.Equal(t, "peer-service", attrMap[semconv.PeerServiceKey])
	_, unrelatedPresent := attrMap["unrelated.key"]
	assert.False(t, unrelatedPresent)
	// status code is always appended.
	_, statusPresent := attrMap[StatusKey]
	assert.True(t, statusPresent)

	assert.NoError(t, tp.Shutdown(context.Background()))
}

// TestExtractMetricsAttributes_NotReadOnlySpan 验证传入非 sdktrace.ReadOnlySpan
// 实现的 span 时返回空切片，而不是 panic。
func TestExtractMetricsAttributes_NotReadOnlySpan(t *testing.T) {
	span := oteltrace.SpanFromContext(context.Background())
	attrs := extractMetricsAttributes(span)
	assert.Empty(t, attrs)
}
