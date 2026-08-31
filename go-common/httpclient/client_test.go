package httpclient

import (
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
