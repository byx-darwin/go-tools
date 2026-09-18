package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.36.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestMatchAttributeKey(t *testing.T) {
	keys := []attribute.Key{attribute.Key("a"), attribute.Key("b")}
	assert.True(t, matchAttributeKey(attribute.Key("a"), keys))
	assert.False(t, matchAttributeKey(attribute.Key("c"), keys))
}

func TestExtractMetricsAttributesFromSpan_NotReadOnlySpan(t *testing.T) {
	// noop span 不实现 trace.ReadOnlySpan。
	_, span := noop.NewTracerProvider().Tracer("t").Start(context.Background(), "s")
	attrs := extractMetricsAttributesFromSpan(span)
	assert.Empty(t, attrs)
}

func TestExtractMetricsAttributesFromSpan_ReadOnlySpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSyncer(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	tracer := tp.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "http.request",
		oteltrace.WithAttributes(
			semconv.HTTPRequestMethodKey.String("GET"),
			semconv.HTTPResponseStatusCode(200),
			attribute.String("irrelevant.key", "x"),
		))
	_ = ctx

	_, ok := span.(trace.ReadOnlySpan)
	require.True(t, ok)

	attrs := extractMetricsAttributesFromSpan(span)
	span.End()

	var foundMethod, foundStatus, foundStatusKey bool
	for _, a := range attrs {
		switch a.Key {
		case semconv.HTTPRequestMethodKey:
			foundMethod = true
		case semconv.HTTPResponseStatusCodeKey:
			foundStatus = true
		case StatusKey:
			foundStatusKey = true
		case attribute.Key("irrelevant.key"):
			t.Errorf("unexpected irrelevant attribute leaked into metrics attrs")
		}
	}
	assert.True(t, foundMethod)
	assert.True(t, foundStatus)
	assert.True(t, foundStatusKey)
}
