package tls

import "go.opentelemetry.io/otel"

// ProducerOption 定义 NewProducer 的配置选项函数。
type ProducerOption func(*Producer)

// WithTrace 启用 OpenTelemetry 追踪：为每次 flush（批量发送）起一个 span，
// 并把触发本次 flush 的各 SendLog 调用方 span（若携带有效 trace 上下文）
// 通过 Link 关联到该 span。未启用时 Producer 行为与现状完全一致。
func WithTrace() ProducerOption {
	return func(p *Producer) {
		p.tracer = otel.Tracer(instrumentationName)
	}
}
