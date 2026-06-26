package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestSessionStruct 验证 Session 结构体字段存在且类型正确。
func TestSessionStruct(t *testing.T) {
	now := time.Now()
	s := Session{
		ID:        "session-123",
		UserUUID:  "user-456",
		Data:      map[string]any{"key": "value"},
		ExpiresAt: now,
	}

	assert.Equal(t, "session-123", s.ID)
	assert.Equal(t, "user-456", s.UserUUID)
	assert.Equal(t, map[string]any{"key": "value"}, s.Data)
	assert.Equal(t, now, s.ExpiresAt)
}

// TestSessionDataMapNil 验证 Data 字段可以是 nil。
func TestSessionDataMapNil(t *testing.T) {
	s := Session{
		ID:       "session-nil-data",
		UserUUID: "user-nil-data",
	}
	assert.Nil(t, s.Data)
}

// ── Store 接口编译期检查 ──

type mockStore struct {
	getFn    func(ctx context.Context, sessionID string) (*Session, error)
	saveFn   func(ctx context.Context, session *Session) error
	deleteFn func(ctx context.Context, sessionID string) error
	existsFn func(ctx context.Context, sessionID string) (bool, error)
}

func (m *mockStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	return m.getFn(ctx, sessionID)
}

func (m *mockStore) Save(ctx context.Context, session *Session) error {
	return m.saveFn(ctx, session)
}

func (m *mockStore) Delete(ctx context.Context, sessionID string) error {
	return m.deleteFn(ctx, sessionID)
}

func (m *mockStore) Exists(ctx context.Context, sessionID string) (bool, error) {
	return m.existsFn(ctx, sessionID)
}

// TestStoreInterface 验证 Store 接口契约。
func TestStoreInterface(t *testing.T) {
	t.Run("Get returns session", func(t *testing.T) {
		store := &mockStore{
			getFn: func(_ context.Context, id string) (*Session, error) {
				return &Session{ID: id, UserUUID: "user-1"}, nil
			},
		}
		s, err := store.Get(context.Background(), "sid-1")
		assert.NoError(t, err)
		assert.Equal(t, "sid-1", s.ID)
		assert.Equal(t, "user-1", s.UserUUID)
	})

	t.Run("Get returns nil when not found", func(t *testing.T) {
		store := &mockStore{
			getFn: func(_ context.Context, _ string) (*Session, error) {
				return nil, nil
			},
		}
		s, err := store.Get(context.Background(), "not-exist")
		assert.NoError(t, err)
		assert.Nil(t, s)
	})

	t.Run("Get returns error", func(t *testing.T) {
		store := &mockStore{
			getFn: func(_ context.Context, _ string) (*Session, error) {
				return nil, errors.New("storage error")
			},
		}
		s, err := store.Get(context.Background(), "sid-1")
		assert.Error(t, err)
		assert.Nil(t, s)
	})

	t.Run("Save succeeds", func(t *testing.T) {
		var saved *Session
		store := &mockStore{
			saveFn: func(_ context.Context, s *Session) error {
				saved = s
				return nil
			},
		}
		s := &Session{ID: "sid-save", UserUUID: "user-save"}
		err := store.Save(context.Background(), s)
		assert.NoError(t, err)
		assert.Equal(t, s, saved)
	})

	t.Run("Save returns error", func(t *testing.T) {
		store := &mockStore{
			saveFn: func(_ context.Context, _ *Session) error {
				return errors.New("save error")
			},
		}
		err := store.Save(context.Background(), &Session{ID: "sid-err"})
		assert.Error(t, err)
	})

	t.Run("Delete succeeds", func(t *testing.T) {
		var deletedID string
		store := &mockStore{
			deleteFn: func(_ context.Context, id string) error {
				deletedID = id
				return nil
			},
		}
		err := store.Delete(context.Background(), "sid-del")
		assert.NoError(t, err)
		assert.Equal(t, "sid-del", deletedID)
	})

	t.Run("Delete returns error", func(t *testing.T) {
		store := &mockStore{
			deleteFn: func(_ context.Context, _ string) error {
				return errors.New("delete error")
			},
		}
		err := store.Delete(context.Background(), "sid-err")
		assert.Error(t, err)
	})

	t.Run("Exists returns true", func(t *testing.T) {
		store := &mockStore{
			existsFn: func(_ context.Context, id string) (bool, error) {
				return id == "sid-exist", nil
			},
		}
		ok, err := store.Exists(context.Background(), "sid-exist")
		assert.NoError(t, err)
		assert.True(t, ok)

		ok, err = store.Exists(context.Background(), "not-exist")
		assert.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("Exists returns error", func(t *testing.T) {
		store := &mockStore{
			existsFn: func(_ context.Context, _ string) (bool, error) {
				return false, errors.New("exists error")
			},
		}
		ok, err := store.Exists(context.Background(), "sid-err")
		assert.Error(t, err)
		assert.False(t, ok)
	})
}
