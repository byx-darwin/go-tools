package kafka

import (
	"context"
	"strconv"

	"github.com/segmentio/kafka-go"
)

// DLQSender 定义 DLQ 转发能力，*Writer 天然实现该接口。抽出接口是为了让
// Consumer.HandleMessage（见 Task 5）在单元测试中可以注入 fake 实现，
// 不需要连接真实 Kafka broker。
type DLQSender interface {
	SendToDLQ(ctx context.Context, dlqTopic string, msg kafka.Message, reason string) error
}

// buildDLQHeaders 在原始消息 Headers 基础上追加 DLQ 元信息，不修改入参 msg。
func buildDLQHeaders(msg kafka.Message, reason string) []kafka.Header {
	headers := make([]kafka.Header, 0, len(msg.Headers)+4)
	headers = append(headers, msg.Headers...)
	headers = append(headers,
		kafka.Header{Key: "x-dlq-reason", Value: []byte(reason)},
		kafka.Header{Key: "x-dlq-original-topic", Value: []byte(msg.Topic)},
		kafka.Header{Key: "x-dlq-original-partition", Value: []byte(strconv.Itoa(msg.Partition))},
		kafka.Header{Key: "x-dlq-original-offset", Value: []byte(strconv.FormatInt(msg.Offset, 10))},
	)
	return headers
}

// SendToDLQ 将消息转发到死信 topic dlqTopic，并附加失败原因（x-dlq-reason）
// 与原始消息元信息（x-dlq-original-topic/partition/offset）作为 Header。
// dlqTopic 由调用方显式指定，Writer 实例不绑定固定的 DLQ topic。
func (w *Writer) SendToDLQ(ctx context.Context, dlqTopic string, msg kafka.Message, reason string) error {
	dlqMsg := kafka.Message{
		Topic:   dlqTopic,
		Key:     msg.Key,
		Value:   msg.Value,
		Headers: buildDLQHeaders(msg, reason),
	}
	if err := w.WriteMessages(ctx, dlqMsg); err != nil {
		return ErrDLQForward.Wrap(err)
	}
	return nil
}
