package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestMutex_TryLock_Success(t *testing.T) {
	_, client := newTestRedisClient(t)
	m := NewMutex(client, "lock:trylock", WithWatchdog(false))

	ok, err := m.TryLock(context.Background())

	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NotEmpty(t, m.token)
}

func TestMutex_TryLock_AlreadyHeld(t *testing.T) {
	_, client := newTestRedisClient(t)
	first := NewMutex(client, "lock:contested", WithWatchdog(false))
	second := NewMutex(client, "lock:contested", WithWatchdog(false))

	ok1, err1 := first.TryLock(context.Background())
	require.NoError(t, err1)
	require.True(t, ok1)

	ok2, err2 := second.TryLock(context.Background())

	assert.NoError(t, err2)
	assert.False(t, ok2)
}
