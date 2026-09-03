package sse

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
)

func TestNewWriter_SetsSSEHeaders(t *testing.T) {
	addr, stop := startTestServer(t, func(ctx context.Context, c *app.RequestContext) {
		w := NewWriter(ctx, c)
		require.NoError(t, w.Close())
	})
	defer stop()

	conn := rawHTTPGet(t, addr, "/sse")
	defer func() { _ = conn.Close() }()
	out := readAllWithTimeout(t, conn, 2*time.Second)

	assert.Contains(t, out, "Content-Type: text/event-stream; charset=utf-8")
	assert.Contains(t, out, "Cache-Control: no-cache")
}

func TestWriter_WriteEvent_Success(t *testing.T) {
	addr, stop := startTestServer(t, func(ctx context.Context, c *app.RequestContext) {
		w := NewWriter(ctx, c)
		require.NoError(t, w.WriteEvent("1", "message", []byte("hello")))
		require.NoError(t, w.Close())
	})
	defer stop()

	conn := rawHTTPGet(t, addr, "/sse")
	defer func() { _ = conn.Close() }()
	body := readAllWithTimeout(t, conn, 2*time.Second)

	assert.Contains(t, body, "id: 1\n")
	assert.Contains(t, body, "event: message\n")
	assert.Contains(t, body, "data: hello\n")
}

func TestWriter_WriteEvent_AfterClose_ReturnsErrWriterClosed(t *testing.T) {
	addr, stop := startTestServer(t, func(ctx context.Context, c *app.RequestContext) {
		w := NewWriter(ctx, c)
		require.NoError(t, w.Close())

		err := w.WriteEvent("1", "message", []byte("hello"))
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrWriterClosed))
	})
	defer stop()

	conn := rawHTTPGet(t, addr, "/sse")
	defer func() { _ = conn.Close() }()
	_ = readAllWithTimeout(t, conn, 2*time.Second)
}

func TestWriter_Close_Idempotent(t *testing.T) {
	addr, stop := startTestServer(t, func(ctx context.Context, c *app.RequestContext) {
		w := NewWriter(ctx, c)
		require.NoError(t, w.Close())
		require.NoError(t, w.Close()) // second Close must not panic or error
	})
	defer stop()

	conn := rawHTTPGet(t, addr, "/sse")
	defer func() { _ = conn.Close() }()
	_ = readAllWithTimeout(t, conn, 2*time.Second)
}

func TestNewWriter_RequestIDRequiresResponderMiddleware(t *testing.T) {
	// Without Responder.Middleware() having run, hertz.RequestIDFrom(ctx)
	// returns "" — NewWriter must not panic and must degrade silently.
	addr, stop := startTestServer(t, func(ctx context.Context, c *app.RequestContext) {
		w := NewWriter(ctx, c)
		require.NotNil(t, w)
		assert.Equal(t, "", hertzresp.RequestIDFrom(c))
		require.NoError(t, w.Close())
	})
	defer stop()

	conn := rawHTTPGet(t, addr, "/sse")
	defer func() { _ = conn.Close() }()
	_ = readAllWithTimeout(t, conn, 2*time.Second)
}
