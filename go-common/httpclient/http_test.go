package httpclient

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConstants(t *testing.T) {
	require.Equal(t, 500*time.Millisecond, DefaultSleep)
	require.Equal(t, "GET", MethodGet)
	require.Equal(t, "POST", MethodPost)
}

func TestSendReturnsFasthttpResponse(t *testing.T) {
	srv := httptest.NewServer(nil)
	defer srv.Close()

	headers := map[string]string{"X-Test": "1"}
	resp, status, err := Send(srv.URL, MethodGet, nil, headers, 2*time.Second)
	require.NoError(t, err)
	require.Equal(t, 404, status)
	require.Equal(t, 404, resp.StatusCode())
	require.Equal(t, "1", headers["X-Test"]) // original map untouched beyond caller's own keys
}

func TestSendWithRetryEventuallySucceeds(t *testing.T) {
	srv := httptest.NewServer(nil)
	defer srv.Close()

	headers := map[string]string{}
	resp, err := SendWithRetry(srv.URL, MethodGet, nil, headers, time.Millisecond, 2*time.Second, 1)
	require.NoError(t, err)
	require.Equal(t, 404, resp.StatusCode())
}
