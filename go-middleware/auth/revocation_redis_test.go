package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/byx-darwin/go-tools/go-auth/revocation"
)

func newTestRevocationRedisClient(t *testing.T) (*miniredis.Miniredis, redis.UniversalClient) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, client
}

func TestRedisRevocationStore_ImplementsInterface(t *testing.T) {
	var _ revocation.Store = (*RedisRevocationStore)(nil)
}

func TestRedisRevocationStore_RevokeAndCheck(t *testing.T) {
	_, client := newTestRevocationRedisClient(t)
	ctx := context.Background()
	s := NewRedisRevocationStore(client)

	revoked, err := s.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	assert.False(t, revoked)

	require.NoError(t, s.Revoke(ctx, "jti-1", time.Hour))

	revoked, err = s.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	assert.True(t, revoked)
}

func TestRedisRevocationStore_TTLExpiry(t *testing.T) {
	mr, client := newTestRevocationRedisClient(t)
	ctx := context.Background()
	s := NewRedisRevocationStore(client)

	require.NoError(t, s.Revoke(ctx, "jti-1", 5*time.Second))

	revoked, err := s.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	assert.True(t, revoked)

	mr.FastForward(6 * time.Second)

	revoked, err = s.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	assert.False(t, revoked, "revocation record should expire after TTL")
}

func TestRedisRevocationStore_ZeroTTLNoop(t *testing.T) {
	_, client := newTestRevocationRedisClient(t)
	ctx := context.Background()
	s := NewRedisRevocationStore(client)

	require.NoError(t, s.Revoke(ctx, "jti-1", 0))

	revoked, err := s.IsRevoked(ctx, "jti-1")
	require.NoError(t, err)
	assert.False(t, revoked)
}

func TestRedisRevocationStore_WithKeyPrefix(t *testing.T) {
	mr, client := newTestRevocationRedisClient(t)
	ctx := context.Background()
	s := NewRedisRevocationStore(client, WithKeyPrefix("app:"))

	require.NoError(t, s.Revoke(ctx, "jti-1", time.Hour))

	keys := mr.Keys()
	assert.Contains(t, keys, "app:revoked:jti-1")
}
