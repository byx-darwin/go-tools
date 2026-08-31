package tls

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewProducer_RequiresEndpoint(t *testing.T) {
	_, err := NewProducer(ProducerConfig{TopicID: "t", Region: "r"})
	assert.ErrorContains(t, err, "endpoint")
}

func TestNewProducer_RequiresTopicID(t *testing.T) {
	_, err := NewProducer(ProducerConfig{Endpoint: "e", Region: "r"})
	assert.ErrorContains(t, err, "topic_id")
}

func TestNewProducer_RequiresRegion(t *testing.T) {
	_, err := NewProducer(ProducerConfig{Endpoint: "e", TopicID: "t"})
	assert.ErrorContains(t, err, "region")
}

func TestNewProducer_Defaults(t *testing.T) {
	p, err := NewProducer(ProducerConfig{
		Endpoint:        "tls.example.com",
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
		Region:          "cn-beijing",
		TopicID:         "topic-123",
	})
	assert.NoError(t, err)
	assert.Equal(t, "go-tools", p.config.Source)
	assert.Equal(t, 10, p.config.BatchSize)
	assert.Equal(t, 5*time.Second, p.config.FlushInterval)
	// Close may error due to flush to invalid endpoint, ignore
	_ = p.Close()
}

func TestProducer_SendLog_Buffers(t *testing.T) {
	p, err := NewProducer(ProducerConfig{
		Endpoint:        "tls.example.com",
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
		Region:          "cn-beijing",
		TopicID:         "topic-123",
		BatchSize:       100, // large batch so flush is not triggered
	})
	assert.NoError(t, err)
	defer func() { _ = p.Close() }()

	// SendLog should buffer without error (no network call when below batch size)
	err = p.SendLog(context.Background(), map[string]string{"level": "info"})
	assert.NoError(t, err)

	p.mu.Lock()
	assert.Len(t, p.buf, 1)
	p.mu.Unlock()
}

func TestProducer_SendLogs_Buffers(t *testing.T) {
	p, err := NewProducer(ProducerConfig{
		Endpoint:        "tls.example.com",
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
		Region:          "cn-beijing",
		TopicID:         "topic-123",
		BatchSize:       100,
	})
	assert.NoError(t, err)
	defer func() { _ = p.Close() }()

	err = p.SendLogs(context.Background(), []map[string]string{
		{"a": "1"}, {"b": "2"}, {"c": "3"},
	})
	assert.NoError(t, err)

	p.mu.Lock()
	assert.Len(t, p.buf, 3)
	p.mu.Unlock()
}

func TestProducer_Flush_NetworkError(t *testing.T) {
	p, err := NewProducer(ProducerConfig{
		Endpoint:        "tls.example.com",
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
		Region:          "cn-beijing",
		TopicID:         "topic-123",
	})
	assert.NoError(t, err)
	defer func() { _ = p.Close() }()

	// Flush will fail because the endpoint is fake, but shouldn't panic
	_ = p.SendLog(context.Background(), map[string]string{"k": "v"})
	err = p.Flush(context.Background())
	// Network error is expected — just verify no panic
	t.Logf("expected network error: %v", err)
}

func TestProducer_Close(t *testing.T) {
	p, err := NewProducer(ProducerConfig{
		Endpoint:        "tls.example.com",
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
		Region:          "cn-beijing",
		TopicID:         "topic-123",
	})
	assert.NoError(t, err)
	// Close will attempt final flush and fail — ignore error
	_ = p.Close()
}

func TestProducer_Close_FlushesRemainingBuffer(t *testing.T) {
	p, err := NewProducer(ProducerConfig{
		Endpoint:        "tls.example.com",
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
		Region:          "cn-beijing",
		TopicID:         "topic-123",
		BatchSize:       100, // large batch so the periodic/threshold flush never fires
		FlushInterval:   time.Hour,
	})
	assert.NoError(t, err)

	err = p.SendLog(context.Background(), map[string]string{"level": "info"})
	assert.NoError(t, err)

	p.mu.Lock()
	assert.Len(t, p.buf, 1)
	p.mu.Unlock()

	// Close should attempt a final flush of the buffered-but-not-yet-sent log
	// instead of silently dropping it. The fake endpoint makes the flush fail,
	// but the buffer must still be drained (flush always resets it) and the
	// error must surface from Close.
	err = p.Close()
	assert.Error(t, err)

	p.mu.Lock()
	assert.Empty(t, p.buf)
	p.mu.Unlock()
}
