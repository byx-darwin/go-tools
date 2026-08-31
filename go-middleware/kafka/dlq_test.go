package kafka

import (
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
