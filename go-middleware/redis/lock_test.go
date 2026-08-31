package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
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

func TestMutex_Unlock_Success(t *testing.T) {
	_, client := newTestRedisClient(t)
	m := NewMutex(client, "lock:unlock", WithWatchdog(false))
	ctx := context.Background()

	ok, err := m.TryLock(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	err = m.Unlock(ctx)

	assert.NoError(t, err)

	exists, err := client.Exists(ctx, "lock:unlock").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists)
}

func TestMutex_Unlock_NotHeld(t *testing.T) {
	_, client := newTestRedisClient(t)
	m := NewMutex(client, "lock:notheld", WithWatchdog(false))

	err := m.Unlock(context.Background())

	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeLockRelease, code)
}

func TestMutex_Unlock_HeldByOther(t *testing.T) {
	_, client := newTestRedisClient(t)
	owner := NewMutex(client, "lock:other", WithWatchdog(false))
	intruder := NewMutex(client, "lock:other", WithWatchdog(false))
	ctx := context.Background()

	ok, err := owner.TryLock(ctx)
	require.NoError(t, err)
	require.True(t, ok)
	intruder.mu.Lock()
	intruder.token = "not-the-real-token"
	intruder.mu.Unlock()

	err = intruder.Unlock(ctx)

	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeLockRelease, code)

	exists, err := client.Exists(ctx, "lock:other").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists, "owner's lock must remain untouched")
}

func TestMutex_Lock_WaitsForRelease(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	holder := NewMutex(client, "lock:wait", WithWatchdog(false), WithRetryInterval(10*time.Millisecond))
	waiter := NewMutex(client, "lock:wait", WithWatchdog(false), WithRetryInterval(10*time.Millisecond))

	ok, err := holder.TryLock(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	done := make(chan error, 1)
	go func() {
		done <- waiter.Lock(ctx)
	}()

	time.Sleep(30 * time.Millisecond)
	require.NoError(t, holder.Unlock(ctx))

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("waiter.Lock did not return after holder released the lock")
	}
}

func TestMutex_Lock_ContextCanceled(t *testing.T) {
	_, client := newTestRedisClient(t)
	holder := NewMutex(client, "lock:cancel", WithWatchdog(false))
	waiter := NewMutex(client, "lock:cancel", WithWatchdog(false), WithRetryInterval(10*time.Millisecond))

	ok, err := holder.TryLock(context.Background())
	require.NoError(t, err)
	require.True(t, ok)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	err = waiter.Lock(ctx)

	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeLockAcquire, code)
}
