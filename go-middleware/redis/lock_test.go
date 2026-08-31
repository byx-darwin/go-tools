package redis

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func newTestRedisClient(t *testing.T) (*miniredis.Miniredis, goredis.UniversalClient) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, client
}

func TestNewMutex_Defaults(t *testing.T) {
	_, client := newTestRedisClient(t)

	m := NewMutex(client, "lock:test")

	assert.Equal(t, "lock:test", m.key)
	assert.Equal(t, defaultMutexTTL, m.ttl)
	assert.Equal(t, defaultRetryInterval, m.retryInterval)
	assert.True(t, m.watchdog)
}

func TestNewMutex_CustomOptions(t *testing.T) {
	_, client := newTestRedisClient(t)

	m := NewMutex(client, "lock:test",
		WithTTL(5*time.Second),
		WithRetryInterval(20*time.Millisecond),
		WithWatchdog(false),
	)

	assert.Equal(t, 5*time.Second, m.ttl)
	assert.Equal(t, 20*time.Millisecond, m.retryInterval)
	assert.False(t, m.watchdog)
}

func TestMutexOption_IgnoresInvalidValues(t *testing.T) {
	_, client := newTestRedisClient(t)

	m := NewMutex(client, "lock:test", WithTTL(0), WithRetryInterval(-1))

	assert.Equal(t, defaultMutexTTL, m.ttl)
	assert.Equal(t, defaultRetryInterval, m.retryInterval)
}
