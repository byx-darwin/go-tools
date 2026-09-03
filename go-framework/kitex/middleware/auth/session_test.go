package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/byx-darwin/go-tools/go-auth/session"
)

// memorySessionStore 是仅用于测试的最小 session.Store 实现。
type memorySessionStore struct {
	sessions map[string]*session.Session
	getErr   error
}

func (m *memorySessionStore) Get(_ context.Context, sessionID string) (*session.Session, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.sessions[sessionID], nil
}
func (m *memorySessionStore) Save(_ context.Context, s *session.Session) error {
	m.sessions[s.ID] = s
	return nil
}
func (m *memorySessionStore) Delete(_ context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}
func (m *memorySessionStore) Exists(_ context.Context, sessionID string) (bool, error) {
	_, ok := m.sessions[sessionID]
	return ok, nil
}

func TestSessionAuthServer_Success(t *testing.T) {
	store := &memorySessionStore{sessions: map[string]*session.Session{
		"sess-1": {ID: "sess-1", UserUUID: "user-1", ExpiresAt: time.Now().Add(time.Hour)},
	}}
	mw := SessionAuthServer(store)

	ctx := metainfo.WithPersistentValue(context.Background(), metaKeySessionID, "sess-1")
	ctx = metainfo.TransferForward(ctx)

	var gotCtx context.Context
	wrapped := mw(noopEndpoint(&gotCtx))
	err := wrapped(ctx, nil, nil)
	require.NoError(t, err)

	s, ok := GetSession(gotCtx)
	require.True(t, ok)
	assert.Equal(t, "user-1", s.UserUUID)
}

func TestSessionAuthServer_MissingSessionID(t *testing.T) {
	store := &memorySessionStore{sessions: map[string]*session.Session{}}
	mw := SessionAuthServer(store)

	err := mw(noopEndpoint(nil))(context.Background(), nil, nil)
	require.Error(t, err)
}

func TestSessionAuthServer_SessionNotFound(t *testing.T) {
	store := &memorySessionStore{sessions: map[string]*session.Session{}}
	mw := SessionAuthServer(store)

	ctx := metainfo.WithPersistentValue(context.Background(), metaKeySessionID, "missing")
	ctx = metainfo.TransferForward(ctx)

	err := mw(noopEndpoint(nil))(ctx, nil, nil)
	require.Error(t, err)
}

func TestSessionAuthServer_StoreError(t *testing.T) {
	store := &memorySessionStore{sessions: map[string]*session.Session{}, getErr: errors.New("store down")}
	mw := SessionAuthServer(store)

	ctx := metainfo.WithPersistentValue(context.Background(), metaKeySessionID, "sess-1")
	ctx = metainfo.TransferForward(ctx)

	err := mw(noopEndpoint(nil))(ctx, nil, nil)
	require.Error(t, err)
}

func TestSessionAuthClient_UsesProviderAndCtxReuse(t *testing.T) {
	provider := func(ctx context.Context) (string, bool) { return "provided-sess", true }
	mw := SessionAuthClient(provider)

	var gotCtx context.Context
	err := mw(noopEndpoint(&gotCtx))(context.Background(), nil, nil)
	require.NoError(t, err)

	forwarded := metainfo.TransferForward(gotCtx)
	id, ok := metainfo.GetPersistentValue(forwarded, metaKeySessionID)
	require.True(t, ok)
	assert.Equal(t, "provided-sess", id)

	// ctx 已有 session id 时不再调用 provider。
	called := false
	provider2 := func(ctx context.Context) (string, bool) { called = true; return "x", true }
	mw2 := SessionAuthClient(provider2)
	ctx := SetSessionID(context.Background(), "ctx-sess")
	err = mw2(noopEndpoint(nil))(ctx, nil, nil)
	require.NoError(t, err)
	assert.False(t, called)
}

func TestSessionAuthClient_NoSessionAvailable(t *testing.T) {
	mw := SessionAuthClient(nil)
	err := mw(noopEndpoint(nil))(context.Background(), nil, nil)
	require.Error(t, err)
}
