package httpclient

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFasthttpTransportDoSuccess(t *testing.T) {
	srv := httptest.NewServer(nil)
	defer srv.Close()

	tr := newFasthttpTransport()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := tr.Do(ctx, &Request{Method: MethodGet, URL: srv.URL})
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode) // httptest.NewServer(nil) uses DefaultServeMux -> 404 for unknown path
}

func TestFasthttpTransportDoNetworkError(t *testing.T) {
	tr := newFasthttpTransport()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := tr.Do(ctx, &Request{Method: MethodGet, URL: "http://127.0.0.1:1"})
	require.Error(t, err)
}
