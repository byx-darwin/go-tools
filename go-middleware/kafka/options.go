package kafka

// ClientOption 定义 Kafka Writer/Consumer 创建选项。
type ClientOption func(*clientOptions)

type clientOptions struct {
	trace bool
}

// WithTrace 启用 OpenTelemetry 消息追踪。
func WithTrace() ClientOption {
	return func(o *clientOptions) {
		o.trace = true
	}
}
