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
		WithMutexTTL(5*time.Second),
		WithMutexRetryInterval(20*time.Millisecond),
		WithWatchdog(false),
	)

	assert.Equal(t, 5*time.Second, m.ttl)
	assert.Equal(t, 20*time.Millisecond, m.retryInterval)
	assert.False(t, m.watchdog)
}

func TestMutexOption_IgnoresInvalidValues(t *testing.T) {
	_, client := newTestRedisClient(t)

	m := NewMutex(client, "lock:test", WithMutexTTL(0), WithMutexRetryInterval(-1))

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
	holder := NewMutex(client, "lock:wait", WithWatchdog(false), WithMutexRetryInterval(10*time.Millisecond))
	waiter := NewMutex(client, "lock:wait", WithWatchdog(false), WithMutexRetryInterval(10*time.Millisecond))

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
	waiter := NewMutex(client, "lock:cancel", WithWatchdog(false), WithMutexRetryInterval(10*time.Millisecond))

	ok, err := holder.TryLock(context.Background())
	require.NoError(t, err)
	require.True(t, ok)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	err = waiter.Lock(ctx)

	code, _ := goerror.Extract(err)
	assert.Equal(t, CodeLockAcquire, code)
}

func TestMutex_Watchdog_RenewsBeforeExpiry(t *testing.T) {
	mr, client := newTestRedisClient(t)
	ctx := context.Background()
	m := NewMutex(client, "lock:watchdog", WithMutexTTL(90*time.Millisecond), WithWatchdog(true))

	ok, err := m.TryLock(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	// 90ms TTL，续期间隔 = ttl/3 = 30ms。累计推进 200ms（> 原始 TTL），
	// 若续期生效，key 应仍然存在。
	//
	// NOTE: mock（miniredis）时钟与真实 watchdog ticker 使用的 wall-clock 是两条独立的时间线。
	// 为了让 watchdog 的第一次续期（真实时间 ~30ms 触发）赶在 mock 时间到达原始 TTL（90ms）
	// 之前发生，每次循环推进的 mock 时间必须与真实 sleep 时间保持 1:1 量级（而不是原始草稿中
	// 4:1 的比例，那样 mock 时间会在 watchdog 第一次真实触发前就已超过 TTL，导致必然失败）。
	// 这里改为 20 次 * 10ms，保持 ttl(90ms) > 续期间隔(30ms) > 单次粒度(10ms) 的相对顺序不变。
	for i := 0; i < 20; i++ {
		mr.FastForward(10 * time.Millisecond)
		time.Sleep(10 * time.Millisecond) // 让 watchdog goroutine 有机会真实执行一次续期
	}

	exists, err := client.Exists(ctx, "lock:watchdog").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists, "watchdog should have renewed the lock before its original TTL")

	require.NoError(t, m.Unlock(ctx))
}

func TestMutex_Watchdog_StopsAfterUnlock(t *testing.T) {
	mr, client := newTestRedisClient(t)
	ctx := context.Background()
	m := NewMutex(client, "lock:watchdog-stop", WithMutexTTL(50*time.Millisecond), WithWatchdog(true))

	ok, err := m.TryLock(ctx)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, m.Unlock(ctx))

	mr.FastForward(200 * time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	exists, err := client.Exists(ctx, "lock:watchdog-stop").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists, "key must not be recreated after Unlock stopped the watchdog")
}

func TestMutex_Watchdog_Disabled_LockExpires(t *testing.T) {
	mr, client := newTestRedisClient(t)
	ctx := context.Background()
	m := NewMutex(client, "lock:no-watchdog", WithMutexTTL(50*time.Millisecond), WithWatchdog(false))

	ok, err := m.TryLock(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	mr.FastForward(60 * time.Millisecond)

	exists, err := client.Exists(ctx, "lock:no-watchdog").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists)
}
