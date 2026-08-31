package kafka

import (
	"context"
	"testing"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/stretchr/testify/assert"
)

func TestPartitionOffsets_EmptyBrokersReturnsWrappedError(t *testing.T) {
	_, err := PartitionOffsets(context.Background(), nil, "orders")
	assert.Error(t, err)
	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeOffsetQuery, code)
}

func TestPartitionOffsets_UnreachableBrokerReturnsWrappedError(t *testing.T) {
	_, err := PartitionOffsets(context.Background(), []string{"127.0.0.1:1"}, "orders")
	assert.Error(t, err)
	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeOffsetQuery, code)
}

func TestConsumer_Lag_UnreachableBrokerReturnsWrappedError(t *testing.T) {
	c := NewConsumer(ReaderConfig{
		Broker: []string{"127.0.0.1:1"},
		Topic:  "orders",
	})
	defer func() { _ = c.Close() }()

	_, err := c.Lag(context.Background())
	assert.Error(t, err)
	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeOffsetQuery, code)
}

func TestConsumer_Seek_GroupModeReturnsWrappedError(t *testing.T) {
	c := NewConsumer(ReaderConfig{
		Broker:  []string{"127.0.0.1:1"},
		Topic:   "orders",
		GroupID: "orders-group",
	})
	defer func() { _ = c.Close() }()

	err := c.Seek(context.Background(), 10)
	assert.Error(t, err)
	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeSeek, code)
}
