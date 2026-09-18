package kafka

import (
	"context"
	"testing"
	"time"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestNewConsumer_WithTLSAndSASL(t *testing.T) {
	cfg := ReaderConfig{
		Broker: []string{"127.0.0.1:1"},
		Topic:  "test",
	}
	cfg.TLS.Enable = true
	cfg.TLS.InsecureSkipVerify = true
	cfg.SASL.Enable = true
	cfg.SASL.User = "user"
	cfg.SASL.Password = "pass"

	c := NewConsumer(cfg)
	require.NotNil(t, c)
	defer func() { _ = c.Close() }()

	assert.NotNil(t, c.r.Config().Dialer)
}

func TestConsumer_ReadMessage_CanceledContextReturnsWrappedError(t *testing.T) {
	c := NewConsumer(ReaderConfig{
		Broker: []string{"127.0.0.1:1"},
		Topic:  "test",
	})
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.ReadMessage(ctx)
	assert.Error(t, err)
	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeRead, code)
}

func TestConsumer_FetchMessage_CanceledContextReturnsWrappedError(t *testing.T) {
	c := NewConsumer(ReaderConfig{
		Broker: []string{"127.0.0.1:1"},
		Topic:  "test",
	})
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.FetchMessage(ctx)
	assert.Error(t, err)
	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeRead, code)
}

func TestConsumer_CommitMessages_NoGroupReturnsWrappedError(t *testing.T) {
	c := NewConsumer(ReaderConfig{
		Broker: []string{"127.0.0.1:1"},
		Topic:  "test",
	})
	defer func() { _ = c.Close() }()

	err := c.CommitMessages(context.Background(), kafka.Message{})
	assert.Error(t, err)
	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeCommit, code)
}

func TestConsumer_CommitMessages_WithTrace_NoGroupReturnsWrappedError(t *testing.T) {
	c := NewConsumer(ReaderConfig{
		Broker: []string{"127.0.0.1:1"},
		Topic:  "test",
	}, WithTrace())
	defer func() { _ = c.Close() }()

	err := c.CommitMessages(context.Background(), kafka.Message{})
	assert.Error(t, err)
	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeCommit, code)
}

func TestWriter_Send_UnreachableBrokerReturnsWrappedError(t *testing.T) {
	w := NewWriter(WriterConfig{
		Broker: []string{"127.0.0.1:1"},
		Topic:  "test",
	})
	defer func() { _ = w.Close() }()

	err := w.Send(context.Background(), []byte("k"), []byte("v"))
	assert.Error(t, err)
	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeWrite, code)
}

func TestWriter_SendStr_UnreachableBrokerReturnsWrappedError(t *testing.T) {
	w := NewWriter(WriterConfig{
		Broker: []string{"127.0.0.1:1"},
		Topic:  "test",
	})
	defer func() { _ = w.Close() }()

	err := w.SendStr(context.Background(), "k", "v")
	assert.Error(t, err)
	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeWrite, code)
}

func TestWriter_SendToDLQ_UnreachableBrokerReturnsWrappedError(t *testing.T) {
	w := NewWriter(WriterConfig{
		Broker: []string{"127.0.0.1:1"},
	})
	defer func() { _ = w.Close() }()

	msg := kafka.Message{Topic: "orders", Key: []byte("k"), Value: []byte("v")}
	err := w.SendToDLQ(context.Background(), "orders-dlq", msg, "handler failed")
	assert.Error(t, err)
	// oops.Code() 返回错误链中最深层的错误码（此处是 WriteMessages 内部的
	// ErrWrite），而非 SendToDLQ 自身 Wrap 使用的 ErrDLQForward，符合
	// oops "deepest error code" 的既定语义（见 samber/oops getDeepestErrorCode）。
	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeWrite, code)
}

func TestWriter_WriteMessages_WithTrace_UnreachableBrokerReturnsWrappedError(t *testing.T) {
	w := NewWriter(WriterConfig{
		Broker: []string{"127.0.0.1:1"},
	}, WithTrace())
	defer func() { _ = w.Close() }()

	err := w.WriteMessages(context.Background(), kafka.Message{Topic: "test", Value: []byte("v")})
	assert.Error(t, err)
	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeWrite, code)
}
