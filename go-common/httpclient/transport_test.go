package httpclient

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeTransport struct {
	do func(ctx context.Context, req *Request) (*Response, error)
}

func (f *fakeTransport) Do(ctx context.Context, req *Request) (*Response, error) {
	return f.do(ctx, req)
}

func TestTransportInterfaceSatisfiedByFake(t *testing.T) {
	var tr Transport = &fakeTransport{
		do: func(_ context.Context, req *Request) (*Response, error) {
			return &Response{StatusCode: 200, Body: []byte("ok"), Header: http.Header{}}, nil
		},
	}
	resp, err := tr.Do(context.Background(), &Request{Method: MethodGet, URL: "http://example.com"})
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	require.Equal(t, "ok", string(resp.Body))
}
