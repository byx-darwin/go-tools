package kafka

import (
	"context"
	"crypto/tls"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Consumer Kafka 消息消费者。
// 封装 kafka-go Reader，支持消费者组和手动提交 offset。
type Consumer struct {
	r      *kafka.Reader
	tracer trace.Tracer

	dlqSender      DLQSender
	dlqTopic       string
	dlqMaxAttempts int
	failureCounter FailureCounter

	brokers []string
	topic   string
}

// NewConsumer 创建 Kafka Consumer，可选 OpenTelemetry 追踪（WithTrace）。
func NewConsumer(cfg ReaderConfig, opts ...ClientOption) *Consumer {
	o := &clientOptions{}
	for _, opt := range opts {
		opt(o)
	}

	rCfg := kafka.ReaderConfig{
		Brokers:        cfg.Broker,
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		MinBytes:       cfg.MinBytes,
		MaxBytes:       cfg.MaxBytes,
		MaxWait:        cfg.MaxWait,
		MaxAttempts:    cfg.MaxAttempts,
		ReadBackoffMin: cfg.ReadBackoffMin,
		ReadBackoffMax: cfg.ReadBackoffMax,
	}

	if cfg.TLS.Enable || cfg.SASL.Enable {
		dialer := &kafka.Dialer{
			Timeout:   kafka.DefaultDialer.Timeout,
			DualStack: kafka.DefaultDialer.DualStack,
		}
		if cfg.TLS.Enable {
			dialer.TLS = &tls.Config{InsecureSkipVerify: cfg.TLS.InsecureSkipVerify} //nolint:gosec // 用户可通过配置显式关闭 TLS 校验
		}
		if cfg.SASL.Enable {
			dialer.SASLMechanism = plain.Mechanism{Username: cfg.SASL.User, Password: cfg.SASL.Password}
		}
		rCfg.Dialer = dialer
	}

	c := &Consumer{r: kafka.NewReader(rCfg), brokers: cfg.Broker, topic: cfg.Topic}
	if o.trace {
		c.tracer = otel.Tracer(instrumentationName)
	}
	c.dlqSender = o.dlqSender
	c.dlqTopic = o.dlqTopic
	c.dlqMaxAttempts = o.dlqMaxAttempts
	c.failureCounter = o.failureCounter
	if c.dlqSender != nil && c.failureCounter == nil {
		c.failureCounter = newMemFailureCounter()
	}
	return c
}

// ReadMessage 读取消息（自动提交 offset）。
func (c *Consumer) ReadMessage(ctx context.Context) (kafka.Message, error) {
	msg, err := c.r.ReadMessage(ctx)
	if err != nil {
		return msg, ErrRead.Wrap(err)
	}
	c.traceReceive(ctx, "ReadMessage", msg)
	return msg, nil
}

// FetchMessage 读取消息（不自动提交）。
func (c *Consumer) FetchMessage(ctx context.Context) (kafka.Message, error) {
	msg, err := c.r.FetchMessage(ctx)
	if err != nil {
		return msg, ErrRead.Wrap(err)
	}
	c.traceReceive(ctx, "FetchMessage", msg)
	return msg, nil
}

// traceReceive 记录一次消息接收：起一个瞬时 span，并通过消息 Headers 里提取出的
// 生产端上下文建立 Link（而非 parent-child——生产/消费是异步关系，避免消息长时间
// 未消费甚至丢失时 span 悬挂）。span 只覆盖"收到了这条消息"这一瞬间，不包括调用方
// 后续的业务处理（拿到消息后控制权已经交还给调用方，wrapper 无法继续包住）。
func (c *Consumer) traceReceive(ctx context.Context, op string, msg kafka.Message) {
	if c.tracer == nil {
		return
	}
	producerCtx := extractTraceContext(msg.Headers)
	_, span := c.tracer.Start(ctx, "kafka."+op,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithLinks(trace.LinkFromContext(producerCtx)),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination", msg.Topic),
			attribute.Int("messaging.kafka.partition", msg.Partition),
			attribute.Int64("messaging.kafka.offset", msg.Offset),
		),
	)
	span.End()
}

// CommitMessages 手动提交 offset。
func (c *Consumer) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	if c.tracer == nil {
		if err := c.r.CommitMessages(ctx, msgs...); err != nil {
			return ErrCommit.Wrap(err)
		}
		return nil
	}

	ctx, span := c.tracer.Start(ctx, "kafka.commit", trace.WithAttributes(
		attribute.Int("messaging.batch.message_count", len(msgs)),
	))
	err := c.r.CommitMessages(ctx, msgs...)
	if err != nil {
		err = ErrCommit.Wrap(err)
	}
	endSpan(span, err)
	return err
}

// Close 关闭消费者。
func (c *Consumer) Close() error {
	return c.r.Close()
}
