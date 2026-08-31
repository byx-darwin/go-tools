package httpclient

import (
	"context"
	"net/http"
)

// Request 是 Transport 处理的统一请求结构体。
type Request struct {
	Method  string
	URL     string
	Body    []byte
	Headers map[string]string
}

// Response 是 Transport 返回的统一响应结构体。
type Response struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

// Transport 是可替换的底层 HTTP 传输接口。
type Transport interface {
	// Do 发送一次 HTTP 请求，遵循 ctx 的取消/超时。
	Do(ctx context.Context, req *Request) (*Response, error)
}
