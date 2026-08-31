package kafka

// ClientOption 定义 Kafka Writer/Consumer 创建选项。
type ClientOption func(*clientOptions)

type clientOptions struct {
	trace bool

	dlqSender      DLQSender
	dlqTopic       string
	dlqMaxAttempts int
	failureCounter FailureCounter
}

// WithTrace 启用 OpenTelemetry 消息追踪。
func WithTrace() ClientOption {
	return func(o *clientOptions) {
		o.trace = true
	}
}

// WithDLQ 为 Consumer 启用失败自动转发 DLQ：sender 为 DLQ 转发目标（通常是
// 另一个 *Writer），dlqTopic 为死信 topic，maxAttempts 为转发前允许的最大失败
// 次数。maxAttempts <= 0 时忽略本 Option，不启用自动转发。仅 NewConsumer 消费
// 该 Option，NewWriter 忽略。
func WithDLQ(sender DLQSender, dlqTopic string, maxAttempts int) ClientOption {
	return func(o *clientOptions) {
		if maxAttempts > 0 {
			o.dlqSender = sender
			o.dlqTopic = dlqTopic
			o.dlqMaxAttempts = maxAttempts
		}
	}
}

// WithFailureCounter 替换 Consumer 默认的内存失败计数器实现。仅 NewConsumer
// 消费该 Option，NewWriter 忽略。
func WithFailureCounter(counter FailureCounter) ClientOption {
	return func(o *clientOptions) {
		if counter != nil {
			o.failureCounter = counter
		}
	}
}
