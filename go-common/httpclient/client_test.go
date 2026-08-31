package httpclient

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewClientDefaults(t *testing.T) {
	c := NewClient()
	require.NotNil(t, c.transport)
	require.Equal(t, 0, c.maxRetries)
	require.Equal(t, DefaultSleep, c.retryInterval)
	require.Equal(t, defaultUserAgent, c.userAgent)
}

func TestNewClientWithOptions(t *testing.T) {
	tr := &fakeTransport{do: nil}
	c := NewClient(
		WithTransport(tr),
		WithMaxRetries(3),
		WithRetryInterval(10*time.Millisecond),
		WithTimeout(time.Second),
		WithUserAgent("custom-ua"),
	)
	require.Same(t, Transport(tr), c.transport)
	require.Equal(t, 3, c.maxRetries)
	require.Equal(t, 10*time.Millisecond, c.retryInterval)
	require.Equal(t, time.Second, c.timeout)
	require.Equal(t, "custom-ua", c.userAgent)
}

func TestWithMaxRetriesIgnoresNegative(t *testing.T) {
	c := NewClient(WithMaxRetries(-1))
	require.Equal(t, 0, c.maxRetries)
}

func TestWithUserAgentIgnoresEmpty(t *testing.T) {
	c := NewClient(WithUserAgent(""))
	require.Equal(t, defaultUserAgent, c.userAgent)
}

func TestClientDoSuccessFirstAttempt(t *testing.T) {
	calls := 0
	tr := &fakeTransport{do: func(_ context.Context, _ *Request) (*Response, error) {
		calls++
		return &Response{StatusCode: 200, Header: http.Header{}}, nil
	}}
	c := NewClient(WithTransport(tr), WithMaxRetries(3), WithRetryInterval(time.Millisecond))

	resp, err := c.Do(context.Background(), MethodGet, "http://example.com", nil, nil)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, 1, calls)
}

func TestClientDoRetriesOnNetworkError(t *testing.T) {
	calls := 0
	tr := &fakeTransport{do: func(_ context.Context, _ *Request) (*Response, error) {
		calls++
		if calls < 3 {
			return nil, errors.New("dial: connection refused")
		}
		return &Response{StatusCode: 200, Header: http.Header{}}, nil
	}}
	c := NewClient(WithTransport(tr), WithMaxRetries(3), WithRetryInterval(time.Millisecond))

	resp, err := c.Do(context.Background(), MethodGet, "http://example.com", nil, nil)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, 3, calls)
}

func TestClientDoRetriesOn5xx(t *testing.T) {
	calls := 0
	tr := &fakeTransport{do: func(_ context.Context, _ *Request) (*Response, error) {
		calls++
		if calls < 2 {
			return &Response{StatusCode: 503, Header: http.Header{}}, nil
		}
		return &Response{StatusCode: 200, Header: http.Header{}}, nil
	}}
	c := NewClient(WithTransport(tr), WithMaxRetries(3), WithRetryInterval(time.Millisecond))

	resp, err := c.Do(context.Background(), MethodGet, "http://example.com", nil, nil)
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, 2, calls)
}

func TestClientDoDoesNotRetryOn4xx(t *testing.T) {
	calls := 0
	tr := &fakeTransport{do: func(_ context.Context, _ *Request) (*Response, error) {
		calls++
		return &Response{StatusCode: 400, Header: http.Header{}}, nil
	}}
	c := NewClient(WithTransport(tr), WithMaxRetries(3), WithRetryInterval(time.Millisecond))

	resp, err := c.Do(context.Background(), MethodGet, "http://example.com", nil, nil)
	require.NoError(t, err)
	require.Equal(t, 400, resp.StatusCode)
	require.Equal(t, 1, calls)
}

func TestClientDoRetriesExhausted(t *testing.T) {
	calls := 0
	tr := &fakeTransport{do: func(_ context.Context, _ *Request) (*Response, error) {
		calls++
		return &Response{StatusCode: 500, Header: http.Header{}}, nil
	}}
	c := NewClient(WithTransport(tr), WithMaxRetries(2), WithRetryInterval(time.Millisecond))

	resp, err := c.Do(context.Background(), MethodGet, "http://example.com", nil, nil)
	require.NoError(t, err)
	require.Equal(t, 500, resp.StatusCode)
	require.Equal(t, 3, calls) // 1 initial + 2 retries
}

func TestClientDoCtxCanceledDuringRetryWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	tr := &fakeTransport{do: func(_ context.Context, _ *Request) (*Response, error) {
		calls++
		if calls == 1 {
			cancel()
		}
		return nil, errors.New("network error")
	}}
	c := NewClient(WithTransport(tr), WithMaxRetries(5), WithRetryInterval(50*time.Millisecond))

	_, err := c.Do(ctx, MethodGet, "http://example.com", nil, nil)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, calls)
}

func TestClientDoClonesHeadersWithoutMutatingCaller(t *testing.T) {
	var seenUA string
	tr := &fakeTransport{do: func(_ context.Context, req *Request) (*Response, error) {
		seenUA = req.Headers["User-Agent"]
		return &Response{StatusCode: 200, Header: http.Header{}}, nil
	}}
	c := NewClient(WithTransport(tr), WithUserAgent("test-ua"))

	headers := map[string]string{"X-Foo": "bar"}
	_, err := c.Do(context.Background(), MethodGet, "http://example.com", nil, headers)
	require.NoError(t, err)
	require.Equal(t, "test-ua", seenUA)
	_, hasUA := headers["User-Agent"]
	require.False(t, hasUA, "caller's original headers map must not be mutated")
}
