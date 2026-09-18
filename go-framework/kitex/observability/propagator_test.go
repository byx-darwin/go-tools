package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestMetadataSupplier_NilMetadata(t *testing.T) {
	s := &metadataSupplier{}
	assert.Empty(t, s.Get("key"))
	assert.Empty(t, s.Keys())
	// Set on a nil map must not panic.
	s.Set("key", "value")
}

func TestMetadataSupplier_GetSetKeys(t *testing.T) {
	md := map[string]string{}
	s := &metadataSupplier{metadata: md}

	assert.Empty(t, s.Get("traceparent"))

	s.Set("traceparent", "00-trace-span-01")
	assert.Equal(t, "00-trace-span-01", s.Get("traceparent"))
	assert.Contains(t, s.Keys(), "traceparent")
}

func TestInjectExtract_RoundTrip(t *testing.T) {
	propagator := propagation.TraceContext{}

	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()
	tracer := tp.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "test-span")
	defer span.End()

	md := make(map[string]string)
	Inject(ctx, propagator, md)
	assert.NotEmpty(t, md["traceparent"])

	extractedCtx := Extract(context.Background(), propagator, md)
	extractedSpanCtx := oteltrace.SpanContextFromContext(extractedCtx)
	assert.True(t, extractedSpanCtx.IsValid())
	assert.Equal(t, span.SpanContext().TraceID(), extractedSpanCtx.TraceID())
}

func TestExtract_EmptyMetadata(t *testing.T) {
	ctx := Extract(context.Background(), propagation.TraceContext{}, map[string]string{})
	assert.False(t, oteltrace.SpanContextFromContext(ctx).IsValid())
}
