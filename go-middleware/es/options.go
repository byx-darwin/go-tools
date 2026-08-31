package es

// ClientOption 定义 Elasticsearch 客户端创建选项。
type ClientOption func(*clientOptions)

type clientOptions struct {
	trace bool
}

// WithTrace 启用 OpenTelemetry HTTP 传输层追踪。
func WithTrace() ClientOption {
	return func(o *clientOptions) {
		o.trace = true
	}
}
