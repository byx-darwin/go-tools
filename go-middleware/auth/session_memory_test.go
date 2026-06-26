package auth

import (
	"context"
	"testing"
	"time"

	"github.com/byx-darwin/go-tools/go-auth/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemorySessionStore_ImplementsInterface(t *testing.T) {
	// compile-time check (see session_memory.go var _ session.Store = ...)
	var _ session.Store = (*MemorySessionStore)(nil)
}

func TestMemorySessionStore_New(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		s := NewMemorySessionStore()
		assert.NotNil(t, s)
		assert.NotNil(t, s.cache)
		assert.Equal(t, defaultSessionTTL, s.ttl)
	})

	t.Run("custom TTL", func(t *testing.T) {
		s := NewMemorySessionStore(WithSessionTTL(5 * time.Minute))
		assert.Equal(t, 5*time.Minute, s.ttl)
	})

	t.Run("custom cache size", func(t *testing.T) {
		s := NewMemorySessionStore(WithCacheSize(512))
		assert.NotNil(t, s.cache)
	})
}

func TestMemorySessionStore_SaveAndGet(t *testing.T) {
	ctx := context.Background()
	s := NewMemorySessionStore()

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
}

func TestMemorySessionStore_GetNotFound(t *testing.T) {
	ctx := context.Background()
	s := NewMemorySessionStore()

	got, err := s.Get(ctx, "not-exist")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMemorySessionStore_SaveOverwrite(t *testing.T) {
	ctx := context.Background()
	s := NewMemorySessionStore()

	require.NoError(t, s.Save(ctx, &session.Session{ID: "sid-1", UserUUID: "user-1"}))
	require.NoError(t, s.Save(ctx, &session.Session{ID: "sid-1", UserUUID: "user-2"}))

	got, err := s.Get(ctx, "sid-1")
	require.NoError(t, err)
	assert.Equal(t, "user-2", got.UserUUID)
}

func TestMemorySessionStore_Delete(t *testing.T) {
	ctx := context.Background()
	s := NewMemorySessionStore()

	require.NoError(t, s.Save(ctx, &session.Session{ID: "sid-1", UserUUID: "user-1"}))
	require.NoError(t, s.Delete(ctx, "sid-1"))

	got, err := s.Get(ctx, "sid-1")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMemorySessionStore_DeleteNonExist(t *testing.T) {
	ctx := context.Background()
	s := NewMemorySessionStore()

	// 删除不存在的 session 不应报错。
	require.NoError(t, s.Delete(ctx, "not-exist"))
}

func TestMemorySessionStore_Exists(t *testing.T) {
	ctx := context.Background()
	s := NewMemorySessionStore()

	ok, err := s.Exists(ctx, "sid-1")
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, s.Save(ctx, &session.Session{ID: "sid-1", UserUUID: "user-1"}))

	ok, err = s.Exists(ctx, "sid-1")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestMemorySessionStore_TTLExpiry(t *testing.T) {
	ctx := context.Background()
	s := NewMemorySessionStore(WithSessionTTL(200 * time.Millisecond))

	require.NoError(t, s.Save(ctx, &session.Session{ID: "sid-1", UserUUID: "user-1"}))

	ok, err := s.Exists(ctx, "sid-1")
	require.NoError(t, err)
	assert.True(t, ok)

	time.Sleep(400 * time.Millisecond)

	ok, err = s.Exists(ctx, "sid-1")
	require.NoError(t, err)
	assert.False(t, ok, "session should expire after TTL")
}

func TestMemorySessionStore_MultipleSessions(t *testing.T) {
	ctx := context.Background()
	s := NewMemorySessionStore()

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

func TestMemorySessionStore_NilData(t *testing.T) {
	ctx := context.Background()
	s := NewMemorySessionStore()

	require.NoError(t, s.Save(ctx, &session.Session{ID: "sid-nil", UserUUID: "user-1"}))

	got, err := s.Get(ctx, "sid-nil")
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Nil(t, got.Data)
}
