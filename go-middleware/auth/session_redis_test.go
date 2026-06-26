package auth

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/byx-darwin/go-tools/go-auth/session"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRedisClient(t *testing.T) (*miniredis.Miniredis, redis.UniversalClient) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mr, client
}

func TestRedisSessionStore_ImplementsInterface(t *testing.T) {
	var _ session.Store = (*RedisSessionStore)(nil)
}

func TestRedisSessionStore_New(t *testing.T) {
	_, client := newTestRedisClient(t)

	t.Run("defaults", func(t *testing.T) {
		s := NewRedisSessionStore(client)
		assert.NotNil(t, s)
		assert.Equal(t, defaultSessionTTL, s.ttl)
		assert.Equal(t, "", s.prefix)
	})

	t.Run("custom options", func(t *testing.T) {
		s := NewRedisSessionStore(client,
			WithSessionTTL(5*time.Minute),
			WithKeyPrefix("myprefix:"),
		)
		assert.Equal(t, 5*time.Minute, s.ttl)
		assert.Equal(t, "myprefix:", s.prefix)
	})
}

func TestRedisSessionStore_SaveAndGet(t *testing.T) {
	mr, client := newTestRedisClient(t)
	ctx := context.Background()
	s := NewRedisSessionStore(client)

	sess := &session.Session{
		ID:       "sid-1",
		UserUUID: "user-1",
		Data:     map[string]any{"role": "admin"},
	}

	require.NoError(t, s.Save(ctx, sess))

	got, err := s.Get(ctx, "sid-1")
	require.NoError(t, err)
	assert.Equal(t, "sid-1", got.ID)
	assert.Equal(t, "user-1", got.UserUUID)
	assert.Equal(t, map[string]any{"role": "admin"}, got.Data)

	// 验证 Redis key 格式。
	keys := mr.Keys()
	assert.Contains(t, keys, "session:sid-1")
}

func TestRedisSessionStore_GetNotFound(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	s := NewRedisSessionStore(client)

	got, err := s.Get(ctx, "not-exist")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestRedisSessionStore_SaveOverwrite(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	s := NewRedisSessionStore(client)

	require.NoError(t, s.Save(ctx, &session.Session{ID: "sid-1", UserUUID: "user-1"}))
	require.NoError(t, s.Save(ctx, &session.Session{ID: "sid-1", UserUUID: "user-2"}))

	got, err := s.Get(ctx, "sid-1")
	require.NoError(t, err)
	assert.Equal(t, "user-2", got.UserUUID)
}

func TestRedisSessionStore_Delete(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	s := NewRedisSessionStore(client)

	require.NoError(t, s.Save(ctx, &session.Session{ID: "sid-1", UserUUID: "user-1"}))
	require.NoError(t, s.Delete(ctx, "sid-1"))

	got, err := s.Get(ctx, "sid-1")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestRedisSessionStore_DeleteNonExist(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	s := NewRedisSessionStore(client)

	// 删除不存在的 session 不应报错。
	require.NoError(t, s.Delete(ctx, "not-exist"))
}

func TestRedisSessionStore_Exists(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	s := NewRedisSessionStore(client)

	ok, err := s.Exists(ctx, "sid-1")
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, s.Save(ctx, &session.Session{ID: "sid-1", UserUUID: "user-1"}))

	ok, err = s.Exists(ctx, "sid-1")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestRedisSessionStore_TTLExpiry(t *testing.T) {
	mr, client := newTestRedisClient(t)
	ctx := context.Background()
	s := NewRedisSessionStore(client, WithSessionTTL(5*time.Second))

	require.NoError(t, s.Save(ctx, &session.Session{ID: "sid-1", UserUUID: "user-1"}))

	ok, err := s.Exists(ctx, "sid-1")
	require.NoError(t, err)
	assert.True(t, ok)

	// 快进 miniredis 时间。
	mr.FastForward(6 * time.Second)

	ok, err = s.Exists(ctx, "sid-1")
	require.NoError(t, err)
	assert.False(t, ok, "session should expire after TTL")
}

func TestRedisSessionStore_NilData(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	s := NewRedisSessionStore(client)

	require.NoError(t, s.Save(ctx, &session.Session{ID: "sid-nil", UserUUID: "user-1"}))

	got, err := s.Get(ctx, "sid-nil")
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Nil(t, got.Data)
}

func TestRedisSessionStore_WithKeyPrefix(t *testing.T) {
	mr, client := newTestRedisClient(t)
	ctx := context.Background()
	s := NewRedisSessionStore(client, WithKeyPrefix("app:"))

	require.NoError(t, s.Save(ctx, &session.Session{ID: "sid-1", UserUUID: "user-1"}))

	keys := mr.Keys()
	assert.Contains(t, keys, "app:session:sid-1")

	got, err := s.Get(ctx, "sid-1")
	require.NoError(t, err)
	assert.Equal(t, "user-1", got.UserUUID)
}

func TestRedisSessionStore_MultipleSessions(t *testing.T) {
	_, client := newTestRedisClient(t)
	ctx := context.Background()
	s := NewRedisSessionStore(client)

	for i := 0; i < 10; i++ {
		require.NoError(t, s.Save(ctx, &session.Session{
			ID:       "sid-" + string(rune('a'+i)),
			UserUUID: "user-1",
		}))
	}

	for i := 0; i < 10; i++ {
		ok, err := s.Exists(ctx, "sid-"+string(rune('a'+i)))
		require.NoError(t, err)
		assert.True(t, ok)
	}
}
