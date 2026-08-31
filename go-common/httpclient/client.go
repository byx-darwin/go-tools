package httpclient

import (
	"context"
	"math/rand"
	"net/http"
	"time"
)

const defaultUserAgent = "sznc-fasthttp-client-" + FasthttpVersion

// Client 是可配置的 HTTP 客户端，通过 Option 定制传输层、重试与超时策略。
type Client struct {
	transport     Transport
	maxRetries    int
	retryInterval time.Duration
	timeout       time.Duration
	userAgent     string
}

// Option 定义 Client 的配置选项函数。
type Option func(*Client)

// WithTransport 设置底层传输实现，默认使用基于 fasthttp 的实现。
func WithTransport(transport Transport) Option {
	return func(c *Client) {
		if transport != nil {
			c.transport = transport
		}
	}
}

// WithMaxRetries 设置最大重试次数，默认 0（不重试）。负数被忽略。
func WithMaxRetries(maxRetries int) Option {
	return func(c *Client) {
		if maxRetries >= 0 {
			c.maxRetries = maxRetries
		}
	}
}

// WithRetryInterval 设置首次重试等待间隔（后续按指数退避+抖动增长），默认 DefaultSleep。
func WithRetryInterval(interval time.Duration) Option {
	return func(c *Client) {
		if interval > 0 {
			c.retryInterval = interval
		}
	}
}

// WithTimeout 设置请求默认超时；若调用 Do 时传入的 ctx 已带 deadline，以 ctx 为准。
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if timeout > 0 {
			c.timeout = timeout
		}
	}
}

// WithUserAgent 设置自定义 User-Agent，默认 "sznc-fasthttp-client-<version>"。
func WithUserAgent(userAgent string) Option {
	return func(c *Client) {
		if userAgent != "" {
			c.userAgent = userAgent
		}
	}
}

// NewClient 创建 HTTP 客户端，支持 Options 配置。
// 默认配置：
//   - transport: 基于 fasthttp 的实现
//   - maxRetries: 0（不重试）
//   - retryInterval: DefaultSleep（500ms）
//   - userAgent: "sznc-fasthttp-client-<version>"
func NewClient(opts ...Option) *Client {
	c := &Client{
		transport:     newFasthttpTransport(),
		maxRetries:    0,
		retryInterval: DefaultSleep,
		userAgent:     defaultUserAgent,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Do 发送一次 HTTP 请求，按 Client 配置的 maxRetries 自动重试。
// 网络层错误（err != nil）或 5xx 状态码触发重试；4xx 不重试。
// 重试等待期间响应 ctx 取消/超时。
func (c *Client) Do(ctx context.Context, method, url string, body []byte, headers map[string]string) (*Response, error) {
	reqHeaders := make(map[string]string, len(headers)+1)
	for k, v := range headers {
		reqHeaders[k] = v
	}
	reqHeaders["User-Agent"] = c.userAgent

	callCtx := ctx
	var cancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	req := &Request{Method: method, URL: url, Body: body, Headers: reqHeaders}

	interval := c.retryInterval
	attempts := c.maxRetries + 1
	var lastResp *Response
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		lastResp, lastErr = c.transport.Do(callCtx, req)
		if !shouldRetry(lastResp, lastErr) {
			return lastResp, lastErr
		}
		if attempt == attempts-1 {
			break
		}

		wait := interval + time.Duration(rand.Int63n(int64(interval)+1))/2
		select {
		case <-callCtx.Done():
			return nil, callCtx.Err()
		case <-time.After(wait):
		}
		interval *= 2
	}
	return lastResp, lastErr
}

func shouldRetry(resp *Response, err error) bool {
	if err != nil {
		return true
	}
	return resp.StatusCode >= http.StatusInternalServerError
}
