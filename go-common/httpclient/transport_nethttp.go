package httpclient

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

type nethttpTransport struct {
	client *http.Client
}

// NewNetHTTPTransport 创建基于标准库 net/http 的 Transport 实现，
// 可通过 WithTransport 注入 Client 以替代默认的 fasthttp 实现。
func NewNetHTTPTransport() Transport {
	return &nethttpTransport{client: &http.Client{}}
}

func (t *nethttpTransport) Do(ctx context.Context, req *Request) (*Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, bytes.NewReader(req.Body))
	if err != nil {
		return nil, err
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	httpResp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = httpResp.Body.Close() }()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	return &Response{
		StatusCode: httpResp.StatusCode,
		Body:       body,
		Header:     httpResp.Header,
	}, nil
}
