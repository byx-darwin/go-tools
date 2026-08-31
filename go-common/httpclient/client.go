package httpclient

import "time"

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
