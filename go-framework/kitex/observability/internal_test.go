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

	tracer := otel.GetTracerProvider().Tracer("test")
	tc.SetTracer(tracer)
	assert.Equal(t, tracer, tc.Tracer())

	_, span := tracer.Start(context.Background(), "span")
	defer span.End()
	tc.SetSpan(span)
	assert.Equal(t, span, tc.Span())
}

func TestWithTraceCarrier_RoundTrip(t *testing.T) {
	tc := &traceCarrier{}
	ctx := withTraceCarrier(context.Background(), tc)

	got := traceCarrierFromContext(ctx)
	assert.Same(t, tc, got)
}

func TestTraceCarrierFromContext_Missing(t *testing.T) {
	got := traceCarrierFromContext(context.Background())
	assert.Nil(t, got)
}

func TestTraceCarrierFromContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), traceCarrierKey{}, "not-a-carrier")
	got := traceCarrierFromContext(ctx)
	assert.Nil(t, got)
}
