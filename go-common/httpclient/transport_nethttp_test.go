package httpclient

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNetHTTPTransportDoSuccess(t *testing.T) {
	srv := httptest.NewServer(nil)
	defer srv.Close()

	tr := NewNetHTTPTransport()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := tr.Do(ctx, &Request{Method: MethodGet, URL: srv.URL})
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode)
}

func TestNetHTTPTransportDoCtxCanceled(t *testing.T) {
	tr := NewNetHTTPTransport()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := tr.Do(ctx, &Request{Method: MethodGet, URL: "http://example.com"})
	require.Error(t, err)
}
