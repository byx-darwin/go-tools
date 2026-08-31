package httpclient

import (
	"context"
	"net/http"
	"time"

	"github.com/valyala/fasthttp"
)

type fasthttpTransport struct{}

// newFasthttpTransport 创建基于 fasthttp 的 Transport 实现（默认实现）。
func newFasthttpTransport() Transport {
	return &fasthttpTransport{}
}

func (t *fasthttpTransport) Do(ctx context.Context, req *Request) (*Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	fReq := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(fReq)
	fReq.Header.SetMethod(req.Method)
	fReq.SetRequestURI(req.URL)
	for k, v := range req.Headers {
		fReq.Header.Set(k, v)
	}
	if len(req.Body) > 0 {
		fReq.SetBody(req.Body)
	}

	fResp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(fResp)

	timeout := time.Until(deadlineOrZero(ctx))
	var err error
	if timeout > 0 {
		err = fasthttp.DoTimeout(fReq, fResp, timeout)
	} else {
		err = fasthttp.Do(fReq, fResp)
	}
	if err != nil {
		return nil, err
	}

	header := make(http.Header)
	for k, v := range fResp.Header.All() {
		header[string(k)] = append(header[string(k)], string(v))
	}
	body := make([]byte, len(fResp.Body()))
	copy(body, fResp.Body())

	return &Response{
		StatusCode: fResp.StatusCode(),
		Body:       body,
		Header:     header,
	}, nil
}

func deadlineOrZero(ctx context.Context) time.Time {
	if dl, ok := ctx.Deadline(); ok {
		return dl
	}
	return time.Time{}
}
