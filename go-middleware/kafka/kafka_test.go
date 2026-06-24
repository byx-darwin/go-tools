package kafka

import (
	"testing"

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
