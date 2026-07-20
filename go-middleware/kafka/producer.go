package kafka

import (
	"context"
	"crypto/tls"

	"github.com/segmentio/kafka-go"
)

// Writer Kafka 消息生产者。
// 封装 kafka-go Writer，提供简洁的发送接口。
type Writer struct {
	w *kafka.Writer
}

// WriterError 写入失败时的错误信息
type WriterError struct {
	Err error
}

func (e *WriterError) Error() string { return e.Err.Error() }
func (e *WriterError) Unwrap() error { return e.Err }

// NewWriter 创建 Kafka Writer。
func NewWriter(cfg WriterConfig) *Writer {
	var transportTLS *tls.Config
	if cfg.TLS.Enable {
		transportTLS = &tls.Config{InsecureSkipVerify: cfg.TLS.InsecureSkipVerify} //nolint:gosec // 用户可通过配置显式关闭 TLS 校验
	}

	return &Writer{
		w: &kafka.Writer{
			Addr:                   kafka.TCP(cfg.Broker...),
			Topic:                  cfg.Topic,
			AllowAutoTopicCreation: cfg.AllowAutoTopicCreation,
			Balancer:               &kafka.LeastBytes{},
			Transport: &kafka.Transport{
				TLS: transportTLS,
			},
		},
	}
}

// WriteMessages 写入多条消息到默认主题。
func (w *Writer) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	return w.w.WriteMessages(ctx, msgs...)
}

// Send 发送单条消息。
func (w *Writer) Send(ctx context.Context, key, value []byte) error {
	return w.w.WriteMessages(ctx, kafka.Message{Key: key, Value: value})
}

// SendStr 发送字符串消息。
func (w *Writer) SendStr(ctx context.Context, key, value string) error {
	return w.Send(ctx, []byte(key), []byte(value))
}

// Close 关闭 Writer。
func (w *Writer) Close() error {
	return w.w.Close()
}
