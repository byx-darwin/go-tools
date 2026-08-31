package kafka

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWriterConfig_Defaults(t *testing.T) {
	c := WriterConfig{}
	assert.Empty(t, c.Broker)
	assert.Empty(t, c.Topic)
	assert.False(t, c.TLS.Enable)
}

func TestWriterConfig_Full(t *testing.T) {
	c := WriterConfig{
		Broker: []string{"k1:9092", "k2:9092"},
		Topic:  "events",
	}
	c.TLS.Enable = true
	c.SASL.User = "admin"

	assert.Equal(t, "events", c.Topic)
	assert.True(t, c.TLS.Enable)
	assert.Equal(t, "admin", c.SASL.User)
}

func TestReaderConfig_Defaults(t *testing.T) {
	c := ReaderConfig{}
	assert.Empty(t, c.Broker)
	assert.Empty(t, c.GroupID)
	assert.False(t, c.TLS.Enable)
}

func TestReaderConfig_Full(t *testing.T) {
	c := ReaderConfig{
		Broker:  []string{"k1:9092"},
		Topic:   "orders",
		GroupID: "order-group",
	}
	c.TLS.Enable = true

	assert.Equal(t, "order-group", c.GroupID)
	assert.True(t, c.TLS.Enable)
}

func TestNewWriter(t *testing.T) {
	w := NewWriter(WriterConfig{
		Broker: []string{"localhost:9092"},
		Topic:  "test",
	})
	assert.NotNil(t, w)
	assert.NotNil(t, w.w)
	_ = w.Close()
}

func TestNewWriter_WithTLS(t *testing.T) {
	cfg := WriterConfig{Broker: []string{"localhost:9092"}}
	cfg.TLS.Enable = true
	w := NewWriter(cfg)
	assert.NotNil(t, w)
	_ = w.Close()
}

func TestNewConsumer(t *testing.T) {
	c := NewConsumer(ReaderConfig{
		Broker: []string{"localhost:9092"},
		Topic:  "test",
	})
	assert.NotNil(t, c)
	_ = c.Close()
}

func TestWriter_Close(t *testing.T) {
	w := NewWriter(WriterConfig{
		Broker: []string{"localhost:9092"},
	})
	err := w.Close()
	if err != nil {
		t.Logf("Close error (expected without Kafka): %v", err)
	}
}

func TestNewWriter_RetryBackoffWired(t *testing.T) {
	w := NewWriter(WriterConfig{
		Broker:          []string{"localhost:9092"},
		MaxAttempts:     5,
		WriteBackoffMin: 50 * time.Millisecond,
		WriteBackoffMax: 2 * time.Second,
	})
	defer func() { _ = w.Close() }()

	assert.Equal(t, 5, w.w.MaxAttempts)
	assert.Equal(t, 50*time.Millisecond, w.w.WriteBackoffMin)
	assert.Equal(t, 2*time.Second, w.w.WriteBackoffMax)
}

func TestNewConsumer_RetryBackoffWired(t *testing.T) {
	c := NewConsumer(ReaderConfig{
		Broker:         []string{"localhost:9092"},
		Topic:          "test",
		MaxAttempts:    7,
		ReadBackoffMin: 20 * time.Millisecond,
		ReadBackoffMax: 3 * time.Second,
	})
	defer func() { _ = c.Close() }()

	cfg := c.r.Config()
	assert.Equal(t, 7, cfg.MaxAttempts)
	assert.Equal(t, 20*time.Millisecond, cfg.ReadBackoffMin)
	assert.Equal(t, 3*time.Second, cfg.ReadBackoffMax)
}
