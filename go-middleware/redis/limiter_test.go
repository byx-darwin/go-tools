package redis

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestLimiter_AllowN_WithinBurst(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	l := NewLimiter(client, "limiter:burst", 1, 3)

	for i := 0; i < 3; i++ {
		ok, err := l.Allow(ctx)
		require.NoError(t, err)
		assert.True(t, ok, "request %d within burst should be allowed", i)
	}

	ok, err := l.Allow(ctx)
	require.NoError(t, err)
	assert.False(t, ok, "request beyond burst should be rejected")
}

func TestLimiter_AllowN_RejectsWhenInsufficientTokens(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	l := NewLimiter(client, "limiter:allown", 1, 5)

	ok, err := l.AllowN(ctx, 3)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = l.AllowN(ctx, 3)
	require.NoError(t, err)
	assert.False(t, ok, "only 2 tokens remain, requesting 3 must fail")
}

func TestLimiter_Refill_OverTime(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	l := NewLimiter(client, "limiter:refill", 10, 1) // 10 tokens/sec, burst=1

	ok, err := l.Allow(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = l.Allow(ctx)
	require.NoError(t, err)
	require.False(t, ok, "bucket should be empty immediately after consuming the single token")

	// miniredis.FastForward only decrements existing key TTLs; it does not
	// advance any clock the Lua script or client observes (time.Now() /
	// redis TIME), so it cannot simulate refill here. Backdate
	// last_refill_ts directly to deterministically simulate elapsed time:
	// 10 tokens/sec * 0.2s = 2 tokens refilled, capped at burst=1.
	past := time.Now().Add(-200 * time.Millisecond).UnixMilli()
	require.NoError(t, client.HSet(ctx, "limiter:refill", "last_refill_ts", past).Err())

	ok, err = l.Allow(ctx)
	require.NoError(t, err)
	assert.True(t, ok, "token should have refilled after 200ms at rate=10/s")
}

func TestLimiter_AllowN_ZeroRate(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	l := NewLimiter(client, "limiter:zerorate", 0, 5)

	// Burst-many requests succeed from the initial token pool (never refilled
	// since rate=0), then every subsequent request is denied. Critically,
	// none of these calls should error: ttlMillis() must not divide by zero
	// and produce a pathological PEXPIRE value.
	for i := 0; i < 5; i++ {
		ok, err := l.Allow(ctx)
		require.NoError(t, err)
		assert.True(t, ok, "request %d within initial burst should be allowed", i)
	}

	ok, err := l.Allow(ctx)
	require.NoError(t, err)
	assert.False(t, ok, "zero-rate limiter must deny once the initial burst is exhausted")
}

func TestLimiter_Wait_SucceedsAfterRefill(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	l := NewLimiter(client, "limiter:wait", 100, 1, WithWaitPollInterval(5*time.Millisecond)) // fast refill for test speed

	ok, err := l.Allow(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	waitCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	err = l.Wait(waitCtx)

	assert.NoError(t, err, "Wait should succeed once tokens refill at rate=100/s")
}

func TestLimiter_Wait_ContextTimeout(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	// rate=0: refill is always exactly 0 regardless of elapsed time, so tokens
	// never leave the hash in scientific-notation form (e.g. "7e-06"), which
	// miniredis's embedded Lua interpreter fails to parse back via tonumber()
	// for integer-mantissa exponents — that parser gap would otherwise reset
	// the bucket to full and make Wait spuriously succeed.
	l := NewLimiter(client, "limiter:wait-timeout", 0, 1, WithWaitPollInterval(5*time.Millisecond)) // never refills

	ok, err := l.Allow(ctx)
	require.NoError(t, err)
	require.True(t, ok)

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel()

	err = l.Wait(waitCtx)

	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
