package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientOption_WithTrace(t *testing.T) {
	o := &clientOptions{}
	WithTrace()(o)
	assert.True(t, o.trace)
}

func TestClientOption_WithDLQ(t *testing.T) {
	o := &clientOptions{}
	sender := &Writer{}
	WithDLQ(sender, "orders-dlq", 3)(o)
	assert.Equal(t, sender, o.dlqSender)
	assert.Equal(t, "orders-dlq", o.dlqTopic)
	assert.Equal(t, 3, o.dlqMaxAttempts)
}

func TestClientOption_WithDLQ_IgnoresNonPositiveMaxAttempts(t *testing.T) {
	o := &clientOptions{}
	WithDLQ(&Writer{}, "orders-dlq", 0)(o)
	assert.Nil(t, o.dlqSender)
	assert.Equal(t, 0, o.dlqMaxAttempts)
}

func TestClientOption_WithFailureCounter(t *testing.T) {
	o := &clientOptions{}
	counter := newMemFailureCounter()
	WithFailureCounter(counter)(o)
	assert.Equal(t, counter, o.failureCounter)
}
