package observability

import (
	"context"
	"testing"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/byx-darwin/go-tools/go-framework/config"
)

func TestTracerOptions(t *testing.T) {
	cfg := newTracerConfig(nil)
	assert.False(t, cfg.recordSourceOperation)
	assert.False(t, cfg.enableGRPCMetadata)

	cfg = newTracerConfig([]TracerOption{
		WithRecordSourceOperation(true),
		WithEnableGRPCMetadata(),
	})
	assert.True(t, cfg.recordSourceOperation)
	assert.True(t, cfg.enableGRPCMetadata)

	cfg = newTracerConfig([]TracerOption{WithRecordSourceOperation(false)})
	assert.False(t, cfg.recordSourceOperation)
}

func TestNewServerSuite(t *testing.T) {
	ss := NewServerSuite(config.ObservabilityConfig{Enabled: true, ServiceName: "svc"})
	require.NotNil(t, ss)
	opts := ss.Options()
	assert.Len(t, opts, 3)
}

func TestNewClientSuite(t *testing.T) {
	cs := NewClientSuite(config.ObservabilityConfig{Enabled: true, ServiceName: "svc"}, WithRecordSourceOperation(true))
	require.NotNil(t, cs)
	opts := cs.Options()
	assert.Len(t, opts, 4)
}

func TestServerMiddleware_Disabled(t *testing.T) {
	mw := ServerMiddleware(config.ObservabilityConfig{Enabled: false})
	called := false
	next := func(ctx context.Context, req, resp any) error {
		called = true
		return nil
	}
	err := mw(next)(context.Background(), nil, nil)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestServerMiddleware_Enabled_NoTraceCarrier(t *testing.T) {
	setTestTracerProvider(t)
	mw := ServerMiddleware(config.ObservabilityConfig{Enabled: true})

	called := false
	next := func(ctx context.Context, req, resp any) error {
		called = true
		return nil
	}
	// No traceCarrier in context (middleware invoked outside of a
	// stats.Tracer.Start-wrapped call) → falls through to next() directly.
	err := mw(next)(context.Background(), nil, nil)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestServerMiddleware_Enabled_WithTraceCarrier(t *testing.T) {
	setTestTracerProvider(t)
	cfg := config.ObservabilityConfig{Enabled: true, EnableGRPCMetadata: true}

	st := &serverTracer{cfg: cfg, tracer: otel.GetTracerProvider().Tracer("test")}
	ctx := st.Start(context.Background())

	// Incoming TTHeader peer service metadata.
	ctx = metainfo.WithValue(ctx, "service.name", "upstream-service")

	// Incoming gRPC metadata.
	grpcMD := metadata.MD{}
	ctx = metadata.NewIncomingContext(ctx, grpcMD)

	mw := ServerMiddleware(cfg)

	var spanInCtx bool
	next := func(ctx context.Context, req, resp any) error {
		spanInCtx = oteltrace.SpanContextFromContext(ctx).IsValid()
		return nil
	}
	err := mw(next)(ctx, nil, nil)
	assert.NoError(t, err)
	assert.True(t, spanInCtx)

	tc := traceCarrierFromContext(ctx)
	require.NotNil(t, tc)
	assert.NotNil(t, tc.Span())
}

func TestClientMiddleware_Disabled(t *testing.T) {
	mw := ClientMiddleware(config.ObservabilityConfig{Enabled: false})
	called := false
	next := func(ctx context.Context, req, resp any) error {
		called = true
		return nil
	}
	err := mw(next)(context.Background(), nil, nil)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestClientMiddleware_Enabled_NoRecordingSpan(t *testing.T) {
	setTestTracerProvider(t)
	mw := ClientMiddleware(config.ObservabilityConfig{Enabled: true})

	called := false
	next := func(ctx context.Context, req, resp any) error {
		called = true
		return nil
	}
	// context.Background() carries no recording span → falls through.
	err := mw(next)(context.Background(), nil, nil)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestClientMiddleware_Enabled_WithRecordingSpan(t *testing.T) {
	tp := setTestTracerProvider(t)
	cfg := config.ObservabilityConfig{Enabled: true, EnableGRPCMetadata: true}

	tracer := tp.Tracer("test")
	ctx, parentSpan := tracer.Start(context.Background(), "parent")
	defer parentSpan.End()

	ct := &clientTracer{cfg: cfg, tracer: tracer}
	ctx = ct.Start(ctx)

	ctx = metainfo.WithBackwardValuesToSend(ctx)
	ctx = metadata.NewOutgoingContext(ctx, metadata.MD{})

	mw := ClientMiddleware(cfg)

	var spanInCtx bool
	next := func(ctx context.Context, req, resp any) error {
		spanInCtx = oteltrace.SpanContextFromContext(ctx).IsValid()
		return nil
	}
	err := mw(next)(ctx, nil, nil)
	assert.NoError(t, err)
	assert.True(t, spanInCtx)

	tc := traceCarrierFromContext(ctx)
	require.NotNil(t, tc)
	assert.NotNil(t, tc.Span())
}

// setTestTracerProvider installs an in-memory-backed TracerProvider as the
// global OTel provider for the duration of the test and returns it.
func setTestTracerProvider(t *testing.T) *sdktrace.TracerProvider {
	t.Helper()
	prev := otel.GetTracerProvider()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(tracetest.NewInMemoryExporter()))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
	})
	return tp
}
