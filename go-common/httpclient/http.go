package httpclient

import (
	"context"
	"time"

	"github.com/valyala/fasthttp"
)

// Send 发送单次 HTTP 请求。
//
// Deprecated: 使用 NewClient 配合 Options 替代，例如
// NewClient().Do(ctx, method, url, body, headers)。
func Send(url, method string, body []byte, headers map[string]string, timeout time.Duration) (*fasthttp.Response, int, error) {
	c := NewClient(WithTimeout(timeout))
	resp, err := c.Do(context.Background(), method, url, body, headers)
	if err != nil {
		return nil, 0, err
	}
	return toFasthttpResponse(resp), resp.StatusCode, nil
}

// SendWithRetry 发送 HTTP 请求，失败时自动重试。
//
// Deprecated: 使用 NewClient 配合 WithMaxRetries/WithRetryInterval 替代。
func SendWithRetry(url, method string,
	body []byte,
	headers map[string]string,
	sleep, timeout time.Duration, retry int) (*fasthttp.Response, error) {
	c := NewClient(WithTimeout(timeout), WithMaxRetries(retry), WithRetryInterval(sleep))
	resp, err := c.Do(context.Background(), method, url, body, headers)
	if err != nil {
		return nil, err
	}
	return toFasthttpResponse(resp), nil
}

func toFasthttpResponse(resp *Response) *fasthttp.Response {
	fResp := &fasthttp.Response{}
	fResp.SetStatusCode(resp.StatusCode)
	fResp.SetBody(resp.Body)
	for k, values := range resp.Header {
		for _, v := range values {
			fResp.Header.Add(k, v)
		}
	}
	return fResp
}
