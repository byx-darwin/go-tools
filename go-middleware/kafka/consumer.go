package kafka

import (
	"context"
	"crypto/tls"

	"github.com/segmentio/kafka-go"
)

// Consumer Kafka 消息消费者。
// 封装 kafka-go Reader，支持消费者组和手动提交 offset。
type Consumer struct {
	r *kafka.Reader
}

// NewConsumer 创建 Kafka Consumer。
func NewConsumer(cfg ReaderConfig) *Consumer {
	rCfg := kafka.ReaderConfig{
		Brokers:  cfg.Broker,
		Topic:    cfg.Topic,
		GroupID:  cfg.GroupID,
		MinBytes: cfg.MinBytes,
		MaxBytes: cfg.MaxBytes,
		MaxWait:  cfg.MaxWait,
	}

	if cfg.TLS.Enable {
		rCfg.Dialer = &kafka.Dialer{
			TLS: &tls.Config{InsecureSkipVerify: cfg.TLS.InsecureSkipVerify}, //nolint:gosec // 用户可通过配置显式关闭 TLS 校验
		}
	}

	return &Consumer{r: kafka.NewReader(rCfg)}
}

// ReadMessage 读取消息（自动提交 offset）。
func (c *Consumer) ReadMessage(ctx context.Context) (kafka.Message, error) {
	return c.r.ReadMessage(ctx)
}

// FetchMessage 读取消息（不自动提交）。
func (c *Consumer) FetchMessage(ctx context.Context) (kafka.Message, error) {
	return c.r.FetchMessage(ctx)
}

// CommitMessages 手动提交 offset。
func (c *Consumer) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	return c.r.CommitMessages(ctx, msgs...)
}

// Close 关闭消费者。
func (c *Consumer) Close() error {
	return c.r.Close()
}
