package sse

import (
	"context"
	"errors"
	"strings"
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

	conn := rawHTTPGet(t, addr)
	defer func() { _ = conn.Close() }()
	out := readAllWithTimeout(t, conn)

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

	conn := rawHTTPGet(t, addr)
	defer func() { _ = conn.Close() }()
	body := readAllWithTimeout(t, conn)

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

	conn := rawHTTPGet(t, addr)
	defer func() { _ = conn.Close() }()
	_ = readAllWithTimeout(t, conn)
}

func TestWriter_Close_Idempotent(t *testing.T) {
	addr, stop := startTestServer(t, func(ctx context.Context, c *app.RequestContext) {
		w := NewWriter(ctx, c)
		require.NoError(t, w.Close())
		require.NoError(t, w.Close()) // second Close must not panic or error
	})
	defer stop()

	conn := rawHTTPGet(t, addr)
	defer func() { _ = conn.Close() }()
	_ = readAllWithTimeout(t, conn)
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

	conn := rawHTTPGet(t, addr)
	defer func() { _ = conn.Close() }()
	_ = readAllWithTimeout(t, conn)
}

func TestWriter_Run_HandlerCompletesNormally(t *testing.T) {
	addr, stop := startTestServer(t, func(ctx context.Context, c *app.RequestContext) {
		w := NewWriter(ctx, c, WithHeartbeatInterval(0))
		err := w.Run(func(w *Writer) error {
			return w.WriteEvent("1", "message", []byte("hi"))
		})
		require.NoError(t, err)
	})
	defer stop()

	conn := rawHTTPGet(t, addr)
	defer func() { _ = conn.Close() }()
	body := readAllWithTimeout(t, conn)
	assert.Contains(t, body, "data: hi\n")
}

func TestWriter_Run_HandlerError_Propagates(t *testing.T) {
	sentinel := errors.New("client gone")
	addr, stop := startTestServer(t, func(ctx context.Context, c *app.RequestContext) {
		w := NewWriter(ctx, c, WithHeartbeatInterval(0))
		err := w.Run(func(w *Writer) error { return sentinel })
		assert.ErrorIs(t, err, sentinel)
	})
	defer stop()

	conn := rawHTTPGet(t, addr)
	defer func() { _ = conn.Close() }()
	_ = readAllWithTimeout(t, conn)
}

func TestWriter_Run_PanicRecovered_WritesErrorEvent(t *testing.T) {
	var recovered any
	addr, stop := startTestServer(t, func(ctx context.Context, c *app.RequestContext) {
		w := NewWriter(ctx, c,
			WithHeartbeatInterval(0),
			WithRecoverHandler(func(rec any) { recovered = rec }),
		)
		err := w.Run(func(w *Writer) error {
			panic("boom")
		})
		require.NoError(t, err) // Run itself must not propagate the panic as an error
	})
	defer stop()

	conn := rawHTTPGet(t, addr)
	defer func() { _ = conn.Close() }()
	body := readAllWithTimeout(t, conn)

	assert.Contains(t, body, "event: error\n")
	assert.Contains(t, body, `"code":500`)
	assert.Equal(t, "boom", recovered)
}

func TestWriter_Run_HeartbeatWritesKeepAlive(t *testing.T) {
	addr, stop := startTestServer(t, func(ctx context.Context, c *app.RequestContext) {
		w := NewWriter(ctx, c, WithHeartbeatInterval(5*time.Millisecond))
		err := w.Run(func(w *Writer) error {
			time.Sleep(30 * time.Millisecond) // let >=1 heartbeat tick fire
			return nil
		})
		require.NoError(t, err)
	})
	defer stop()

	conn := rawHTTPGet(t, addr)
	defer func() { _ = conn.Close() }()
	body := readAllWithTimeout(t, conn)
	assert.True(t, strings.Contains(body, ":keep-alive\n"), "expected at least one keep-alive comment, got: %q", body)
}

func TestWriter_Run_ContextCancel_ClosesWriter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var w *Writer
	handlerStarted := make(chan struct{})
	done := make(chan struct{})

	addr, stop := startTestServer(t, func(_ context.Context, c *app.RequestContext) {
		w = NewWriter(ctx, c, WithHeartbeatInterval(5*time.Millisecond))
		close(handlerStarted)
		_ = w.Run(func(w *Writer) error {
			<-done // block until the test cancels ctx and observes the effect
			return nil
		})
	})
	defer stop()

	conn := rawHTTPGet(t, addr)
	defer func() { _ = conn.Close() }()

	<-handlerStarted
	time.Sleep(20 * time.Millisecond) // let Run start the heartbeat goroutine
	cancel()

	require.Eventually(t, func() bool {
		return w.closed.Load()
	}, time.Second, 5*time.Millisecond, "writer should be closed after ctx cancellation")
	close(done)
}
