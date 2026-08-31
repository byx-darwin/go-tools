package kafka

import (
	"context"
	"crypto/tls"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Writer Kafka 消息生产者。
// 封装 kafka-go Writer，提供简洁的发送接口。
type Writer struct {
	w      *kafka.Writer
	tracer trace.Tracer
}

// NewWriter 创建 Kafka Writer，可选 OpenTelemetry 追踪（WithTrace）。
func NewWriter(cfg WriterConfig, opts ...ClientOption) *Writer {
	o := &clientOptions{}
	for _, opt := range opts {
		opt(o)
	}

	var transportTLS *tls.Config
	if cfg.TLS.Enable {
		transportTLS = &tls.Config{InsecureSkipVerify: cfg.TLS.InsecureSkipVerify} //nolint:gosec // 用户可通过配置显式关闭 TLS 校验
	}

	var mechanism sasl.Mechanism
	if cfg.SASL.Enable {
		mechanism = plain.Mechanism{Username: cfg.SASL.User, Password: cfg.SASL.Password}
	}

	w := &Writer{
		w: &kafka.Writer{
			Addr:                   kafka.TCP(cfg.Broker...),
			Topic:                  cfg.Topic,
			AllowAutoTopicCreation: cfg.AllowAutoTopicCreation,
			Balancer:               &kafka.LeastBytes{},
			MaxAttempts:            cfg.MaxAttempts,
			WriteBackoffMin:        cfg.WriteBackoffMin,
			WriteBackoffMax:        cfg.WriteBackoffMax,
			Transport: &kafka.Transport{
				TLS:  transportTLS,
				SASL: mechanism,
			},
		},
	}
	if o.trace {
		w.tracer = otel.Tracer(instrumentationName)
	}
	return w
}

// WriteMessages 写入多条消息到默认主题。
func (w *Writer) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	if w.tracer == nil {
		if err := w.w.WriteMessages(ctx, msgs...); err != nil {
			return ErrWrite.Wrap(err)
		}
		return nil
	}

	topic := w.w.Topic
	if topic == "" && len(msgs) > 0 {
		topic = msgs[0].Topic
	}
	ctx, span := w.tracer.Start(ctx, "kafka.send",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", topic),
			attribute.Int("messaging.batch.message_count", len(msgs)),
		),
	)
	for i := range msgs {
		injectTraceContext(ctx, &msgs[i].Headers)
	}

	err := w.w.WriteMessages(ctx, msgs...)
	if err != nil {
		err = ErrWrite.Wrap(err)
	}
	endSpan(span, err)
	return err
}

// Send 发送单条消息。
func (w *Writer) Send(ctx context.Context, key, value []byte) error {
	return w.WriteMessages(ctx, kafka.Message{Key: key, Value: value})
}

// SendStr 发送字符串消息。
func (w *Writer) SendStr(ctx context.Context, key, value string) error {
	return w.Send(ctx, []byte(key), []byte(value))
}

// Close 关闭 Writer。
func (w *Writer) Close() error {
	return w.w.Close()
}
