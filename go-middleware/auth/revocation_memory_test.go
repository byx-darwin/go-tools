package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/byx-darwin/go-tools/go-auth/revocation"
)

func TestMemoryRevocationStoreInterface(t *testing.T) {
	var _ revocation.Store = NewMemoryRevocationStore()
}

func TestMemoryRevocationStoreRevokeAndCheck(t *testing.T) {
	store := NewMemoryRevocationStore()
	ctx := context.Background()

	revoked, err := store.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	assert.False(t, revoked, "未撤销的 jti 应返回 false")

	require.NoError(t, store.Revoke(ctx, "jti-1", time.Hour))

	revoked, err = store.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	assert.True(t, revoked, "已撤销的 jti 应返回 true")
}

func TestMemoryRevocationStoreNonPositiveTTLNoop(t *testing.T) {
	store := NewMemoryRevocationStore()
	ctx := context.Background()

	require.NoError(t, store.Revoke(ctx, "jti-expired", 0))
	require.NoError(t, store.Revoke(ctx, "jti-negative", -time.Second))

	revoked, err := store.IsRevoked(ctx, "jti-expired")
	require.NoError(t, err)
	assert.False(t, revoked, "ttl<=0 视为无效撤销请求，不应写入")

	revoked, err = store.IsRevoked(ctx, "jti-negative")
	require.NoError(t, err)
	assert.False(t, revoked)
}

func TestMemoryRevocationStoreTTLExpiry(t *testing.T) {
	store := NewMemoryRevocationStore()
	ctx := context.Background()

	require.NoError(t, store.Revoke(ctx, "jti-short", 20*time.Millisecond))

	revoked, err := store.IsRevoked(ctx, "jti-short")
	require.NoError(t, err)
	assert.True(t, revoked)

	time.Sleep(60 * time.Millisecond)

	revoked, err = store.IsRevoked(ctx, "jti-short")
	require.NoError(t, err)
	assert.False(t, revoked, "TTL 过期后应自动清除撤销记录")
}
