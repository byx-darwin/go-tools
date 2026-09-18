package observability

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/byx-darwin/go-tools/go-framework/config"
)

func TestHeaderCarrier_GetSetKeys(t *testing.T) {
	header := &protocol.RequestHeader{}
	hc := &headerCarrier{header: header}

	hc.Set("X-Trace-Id", "abc123")
	assert.Equal(t, "abc123", hc.Get("X-Trace-Id"))

	keys := hc.Keys()
	assert.Contains(t, keys, "X-Trace-Id")
}

func TestClientMiddleware_Disabled(t *testing.T) {
	cfg := config.ObservabilityConfig{Enabled: false}
	mw := ClientMiddleware(cfg)
	require.NotNil(t, mw)

	called := false
	endpoint := mw(func(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
		called = true
		return nil
	})

	req := &protocol.Request{}
	resp := &protocol.Response{}
	err := endpoint(context.Background(), req, resp)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestClientMiddleware_Enabled_Success(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSyncer(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	otel.SetTracerProvider(tp)

	cfg := config.ObservabilityConfig{Enabled: true}
	mw := ClientMiddleware(cfg)

	endpoint := mw(func(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
		resp.SetStatusCode(200)
		return nil
	})

	req := &protocol.Request{}
	req.SetMethod("GET")
	req.SetRequestURI("http://example.com/path")
	resp := &protocol.Response{}

	err := endpoint(nil, req, resp) //nolint:staticcheck // 验证 ctx==nil 时中间件自行兜底 context.Background()
	require.NoError(t, err)
	require.NoError(t, tp.ForceFlush(context.Background()))

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "GET /path", spans[0].Name)
}

func TestClientMiddleware_Enabled_ErrorAndHTTPError(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSyncer(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	otel.SetTracerProvider(tp)

	cfg := config.ObservabilityConfig{Enabled: true}
	mw := ClientMiddleware(cfg)

	wantErr := errors.New("boom")
	endpoint := mw(func(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
		return wantErr
	})

	req := &protocol.Request{}
	req.SetMethod("POST")
	req.SetRequestURI("http://example.com/err")
	resp := &protocol.Response{}

	err := endpoint(context.Background(), req, resp)
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, tp.ForceFlush(context.Background()))

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "boom", spans[0].Status.Description)
}

func TestClientMiddleware_Enabled_HTTPStatusError(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSyncer(exp))
	defer func() { _ = tp.Shutdown(context.Background()) }()
	otel.SetTracerProvider(tp)

	cfg := config.ObservabilityConfig{Enabled: true}
	mw := ClientMiddleware(cfg)

	endpoint := mw(func(ctx context.Context, req *protocol.Request, resp *protocol.Response) error {
		resp.SetStatusCode(500)
		return nil
	})

	req := &protocol.Request{}
	req.SetMethod("GET")
	req.SetRequestURI("http://example.com/fail")
	resp := &protocol.Response{}

	err := endpoint(context.Background(), req, resp)
	require.NoError(t, err)
	require.NoError(t, tp.ForceFlush(context.Background()))

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	assert.Contains(t, spans[0].Status.Description, "500")
}
