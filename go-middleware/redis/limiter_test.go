package redis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewLimiter_Defaults(t *testing.T) {
	_, client := newTestRedisClient(t)

	l := NewLimiter(client, "limiter:test", 10, 5)

	assert.Equal(t, "limiter:test", l.key)
	assert.InDelta(t, 10.0, l.rate, 0.0001)
	assert.Equal(t, 5, l.burst)
	assert.Equal(t, defaultWaitPollInterval, l.waitPollInterval)
}

func TestNewLimiter_CustomOptions(t *testing.T) {
	_, client := newTestRedisClient(t)

	l := NewLimiter(client, "limiter:test", 10, 5, WithWaitPollInterval(200*time.Millisecond))

	assert.Equal(t, 200*time.Millisecond, l.waitPollInterval)
}

func TestLimiterOption_IgnoresInvalidValues(t *testing.T) {
	_, client := newTestRedisClient(t)

	l := NewLimiter(client, "limiter:test", 10, 5, WithWaitPollInterval(0))

	assert.Equal(t, defaultWaitPollInterval, l.waitPollInterval)
}
