package http_client

import (
	"github.com/valyala/fasthttp"
	"time"
)

func Send(url, method string, body []byte, headers map[string]string, timeout time.Duration) (*fasthttp.Response, int, error) {
	headers["User-Agent"] = "sznc-fasthttp-client-" + FasthttpVersion
	return doSend(url, method, body, headers, timeout)
}

func SendWithRetry(url, method string,
	body []byte,
	headers map[string]string,
	sleep, timeout time.Duration, retry int) (*fasthttp.Response, error) {
	headers["User-Agent"] = "sznc-retry-fasthttp-client-" + FasthttpVersion
	response, _, err := Retry(retry, sleep, func() (*fasthttp.Response, int, error) {
		return doSend(url, method, body, headers, timeout)
	})
	return response, err
}

func doSend(url, method string, body []byte, headers map[string]string, timeout time.Duration) (*fasthttp.Response, int, error) {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	req.Header.SetMethod(method)
	req.SetRequestURI(url)
	for header, v := range headers {
		req.Header.Set(header, v)
	}
	if len(body) > 0 {
		req.SetBody(body)
	}
	rsp := fasthttp.AcquireResponse()
	if err := fasthttp.DoTimeout(req, rsp, timeout); err != nil {
		return nil, 0, err
	}
	return rsp, rsp.StatusCode(), nil
}
