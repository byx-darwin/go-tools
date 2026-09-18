package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
)

func TestTraceCarrier_SetGet(t *testing.T) {
	tc := &traceCarrier{}
	assert.Nil(t, tc.Tracer())
	assert.Nil(t, tc.Span())

	tracer := otel.GetTracerProvider().Tracer(instrumentationName)
	tc.SetTracer(tracer)
	assert.Equal(t, tracer, tc.Tracer())

	_, span := tracer.Start(context.Background(), "test-span")
	defer span.End()
	tc.SetSpan(span)
	assert.Equal(t, span, tc.Span())
}

func TestWithTraceCarrier_And_FromContext(t *testing.T) {
	tc := &traceCarrier{}
	ctx := withTraceCarrier(context.Background(), tc)

	got := traceCarrierFromContext(ctx)
	assert.Same(t, tc, got)
}

func TestTraceCarrierFromContext_NotPresent(t *testing.T) {
	got := traceCarrierFromContext(context.Background())
	assert.Nil(t, got)
}
