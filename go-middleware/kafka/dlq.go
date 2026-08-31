package kafka

import (
	"context"
	"errors"
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

// failureKey 计算消息的失败计数 key：优先用业务消息键（msg.Key），为空时退化
// 为 topic/partition/offset，避免所有空 key 消息共享一个计数器。
func failureKey(msg kafka.Message) string {
	if len(msg.Key) > 0 {
		return string(msg.Key)
	}
	return msg.Topic + "/" + strconv.Itoa(msg.Partition) + "/" + strconv.FormatInt(msg.Offset, 10)
}

// HandleMessage 执行 handler 处理消息；handler 成功时清零该消息的失败计数并
// 返回 nil。handler 失败且未通过 WithDLQ 配置 DLQ 时，直接返回 handler 的 err。
// 已配置 DLQ 时，按 failureKey 累计失败次数，达到 WithDLQ 配置的 maxAttempts
// 后自动转发 DLQ 并清零计数（视为该消息已终结处理，返回 nil）；未达阈值则
// 原样返回 handler 的 err，交由调用方决定重试或丢弃。若 DLQ 转发本身失败，
// 返回的 err 通过 errors.Join 同时保留 handler 原始错误与转发错误，两者均可
// 用 errors.Is 判断。
func (c *Consumer) HandleMessage(ctx context.Context, msg kafka.Message, handler func(context.Context, kafka.Message) error) error {
	key := failureKey(msg)
	err := handler(ctx, msg)
	if err == nil {
		if c.failureCounter != nil {
			c.failureCounter.Reset(key)
		}
		return nil
	}
	if c.dlqSender == nil {
		return err
	}

	counter := c.failureCounter
	if counter.Incr(key) < c.dlqMaxAttempts {
		return err
	}

	counter.Reset(key)
	if dlqErr := c.dlqSender.SendToDLQ(ctx, c.dlqTopic, msg, err.Error()); dlqErr != nil {
		// 同时保留原始 handler 错误与 DLQ 转发错误，调用方可用 errors.Is 分别判断。
		return errors.Join(err, ErrDLQForward.Wrap(dlqErr))
	}
	return nil
}
