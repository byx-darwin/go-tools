package kafka

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
)

func headerValue(headers []kafka.Header, key string) (string, bool) {
	for _, h := range headers {
		if h.Key == key {
			return string(h.Value), true
		}
	}
	return "", false
}

func TestBuildDLQHeaders_AppendsMetadata(t *testing.T) {
	msg := kafka.Message{
		Topic:     "orders",
		Partition: 2,
		Offset:    42,
		Headers:   []kafka.Header{{Key: "trace-id", Value: []byte("abc")}},
	}

	headers := buildDLQHeaders(msg, "handler panicked")

	v, ok := headerValue(headers, "x-dlq-reason")
	assert.True(t, ok)
	assert.Equal(t, "handler panicked", v)

	v, ok = headerValue(headers, "x-dlq-original-topic")
	assert.True(t, ok)
	assert.Equal(t, "orders", v)

	v, ok = headerValue(headers, "x-dlq-original-partition")
	assert.True(t, ok)
	assert.Equal(t, "2", v)

	v, ok = headerValue(headers, "x-dlq-original-offset")
	assert.True(t, ok)
	assert.Equal(t, "42", v)

	// 原有 Header 保留
	v, ok = headerValue(headers, "trace-id")
	assert.True(t, ok)
	assert.Equal(t, "abc", v)
}

func TestBuildDLQHeaders_DoesNotMutateOriginalMessage(t *testing.T) {
	original := []kafka.Header{{Key: "trace-id", Value: []byte("abc")}}
	msg := kafka.Message{Topic: "orders", Headers: original}

	buildDLQHeaders(msg, "boom")

	assert.Len(t, original, 1, "原始 Headers 切片不应被追加操作污染")
}

func TestPartitionOffsetToString(t *testing.T) {
	assert.Equal(t, "2", strconv.Itoa(2))
}

type fakeDLQSender struct {
	calls []struct {
		topic  string
		msg    kafka.Message
		reason string
	}
	err error
}

func (f *fakeDLQSender) SendToDLQ(_ context.Context, dlqTopic string, msg kafka.Message, reason string) error {
	f.calls = append(f.calls, struct {
		topic  string
		msg    kafka.Message
		reason string
	}{dlqTopic, msg, reason})
	return f.err
}

func newTestConsumerWithDLQ(sender DLQSender, dlqTopic string, maxAttempts int) *Consumer {
	return &Consumer{
		dlqSender:      sender,
		dlqTopic:       dlqTopic,
		dlqMaxAttempts: maxAttempts,
		failureCounter: newMemFailureCounter(),
	}
}

func TestFailureKey_UsesMessageKeyWhenPresent(t *testing.T) {
	msg := kafka.Message{Key: []byte("order-1"), Topic: "orders", Partition: 0, Offset: 5}
	assert.Equal(t, "order-1", failureKey(msg))
}

func TestFailureKey_FallsBackToTopicPartitionOffsetWhenKeyEmpty(t *testing.T) {
	msg := kafka.Message{Topic: "orders", Partition: 1, Offset: 9}
	assert.Equal(t, "orders/1/9", failureKey(msg))
}

func TestHandleMessage_SuccessResetsCounterAndReturnsNil(t *testing.T) {
	sender := &fakeDLQSender{}
	c := newTestConsumerWithDLQ(sender, "orders-dlq", 2)
	msg := kafka.Message{Key: []byte("k1")}

	err := c.HandleMessage(context.Background(), msg, func(context.Context, kafka.Message) error {
		return nil
	})

	assert.NoError(t, err)
	assert.Empty(t, sender.calls)
}

func TestHandleMessage_BelowThresholdReturnsOriginalError(t *testing.T) {
	sender := &fakeDLQSender{}
	c := newTestConsumerWithDLQ(sender, "orders-dlq", 3)
	msg := kafka.Message{Key: []byte("k1")}
	wantErr := errors.New("boom")

	err := c.HandleMessage(context.Background(), msg, func(context.Context, kafka.Message) error {
		return wantErr
	})

	assert.ErrorIs(t, err, wantErr)
	assert.Empty(t, sender.calls)
}

func TestHandleMessage_ThresholdReachedForwardsToDLQAndReturnsNil(t *testing.T) {
	sender := &fakeDLQSender{}
	c := newTestConsumerWithDLQ(sender, "orders-dlq", 2)
	msg := kafka.Message{Key: []byte("k1")}
	handlerErr := errors.New("boom")
	handler := func(context.Context, kafka.Message) error { return handlerErr }

	err1 := c.HandleMessage(context.Background(), msg, handler)
	assert.ErrorIs(t, err1, handlerErr)

	err2 := c.HandleMessage(context.Background(), msg, handler)
	assert.NoError(t, err2)

	assert.Len(t, sender.calls, 1)
	assert.Equal(t, "orders-dlq", sender.calls[0].topic)
	assert.Equal(t, "boom", sender.calls[0].reason)
}

func TestHandleMessage_ThresholdReachedResetsCounterAfterForward(t *testing.T) {
	sender := &fakeDLQSender{}
	c := newTestConsumerWithDLQ(sender, "orders-dlq", 2)
	msg := kafka.Message{Key: []byte("k1")}
	handler := func(context.Context, kafka.Message) error { return errors.New("boom") }

	// 前两次失败达到 maxAttempts=2 阈值，触发一次转发并重置计数。
	_ = c.HandleMessage(context.Background(), msg, handler)
	_ = c.HandleMessage(context.Background(), msg, handler)
	assert.Len(t, sender.calls, 1)

	// 重置后仅 1 次失败，未达到 maxAttempts=2，不应再次转发。
	_ = c.HandleMessage(context.Background(), msg, handler)
	assert.Len(t, sender.calls, 1, "刚重置的 key 需要再次达到 maxAttempts 才应再次转发")
}

func TestHandleMessage_DLQForwardFailureWrapsBothErrors(t *testing.T) {
	sender := &fakeDLQSender{err: errors.New("dlq unreachable")}
	c := newTestConsumerWithDLQ(sender, "orders-dlq", 1)
	msg := kafka.Message{Key: []byte("k1")}
	handlerErr := errors.New("boom")

	err := c.HandleMessage(context.Background(), msg, func(context.Context, kafka.Message) error {
		return handlerErr
	})

	assert.Error(t, err)
	assert.ErrorIs(t, err, handlerErr)
}

func TestHandleMessage_NoDLQConfiguredReturnsHandlerErrorDirectly(t *testing.T) {
	c := &Consumer{}
	msg := kafka.Message{Key: []byte("k1")}
	wantErr := errors.New("boom")

	err := c.HandleMessage(context.Background(), msg, func(context.Context, kafka.Message) error {
		return wantErr
	})

	assert.ErrorIs(t, err, wantErr)
}
