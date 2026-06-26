package middleware

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/stretchr/testify/assert"

	"github.com/byx-darwin/go-tools/go-auth/session"
)

type mockSessionStore struct {
	sessions map[string]*session.Session
}

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{sessions: make(map[string]*session.Session)}
}

func (m *mockSessionStore) Get(_ context.Context, sessionID string) (*session.Session, error) {
	s, ok := m.sessions[sessionID]
	if !ok {
		return nil, nil
	}
	return s, nil
}

func (m *mockSessionStore) Save(_ context.Context, s *session.Session) error {
	m.sessions[s.ID] = s
	return nil
}

func (m *mockSessionStore) Delete(_ context.Context, sessionID string) error {
	delete(m.sessions, sessionID)
	return nil
}

func (m *mockSessionStore) Exists(_ context.Context, sessionID string) (bool, error) {
	_, ok := m.sessions[sessionID]
	return ok, nil
}

func newSessionTestEngine() *route.Engine {
	opt := config.NewOptions([]config.Option{})
	return route.NewEngine(opt)
}

func TestSessionAuth_SuccessFromHeader(t *testing.T) {
	store := newMockSessionStore()
	s := &session.Session{
		ID:        "sess-123",
		UserUUID:  "user-1",
		Data:      map[string]any{"role": "admin"},
		ExpiresAt: time.Now().Add(time.Hour),
	}
	_ = store.Save(context.Background(), s)

	engine := newSessionTestEngine()
	engine.Use(SessionAuth(store))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		got, ok := GetSession(c)
		assert.True(t, ok)
		sess := got.(*session.Session)
		c.JSON(http.StatusOK, map[string]string{"user": sess.UserUUID})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "X-Session-Id", Value: "sess-123"})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
	assert.Contains(t, string(res.Body()), "user-1")
}

func TestSessionAuth_SuccessFromCookie(t *testing.T) {
	store := newMockSessionStore()
	s := &session.Session{
		ID:        "sess-456",
		UserUUID:  "user-2",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	_ = store.Save(context.Background(), s)

	engine := newSessionTestEngine()
	engine.Use(SessionAuth(store))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		got, ok := GetSession(c)
		assert.True(t, ok)
		sess := got.(*session.Session)
		c.JSON(http.StatusOK, map[string]string{"user": sess.UserUUID})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Cookie", Value: "session_id=sess-456"})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
	assert.Contains(t, string(res.Body()), "user-2")
}

func TestSessionAuth_HeaderPriorityOverCookie(t *testing.T) {
	store := newMockSessionStore()
	s1 := &session.Session{ID: "from-header", UserUUID: "u-h", ExpiresAt: time.Now().Add(time.Hour)}
	s2 := &session.Session{ID: "from-cookie", UserUUID: "u-c", ExpiresAt: time.Now().Add(time.Hour)}
	_ = store.Save(context.Background(), s1)
	_ = store.Save(context.Background(), s2)

	engine := newSessionTestEngine()
	engine.Use(SessionAuth(store))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		got, ok := GetSession(c)
		assert.True(t, ok)
		sess := got.(*session.Session)
		c.JSON(http.StatusOK, map[string]string{"user": sess.UserUUID})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "X-Session-Id", Value: "from-header"},
		ut.Header{Key: "Cookie", Value: "session_id=from-cookie"})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
	assert.Contains(t, string(res.Body()), "u-h")
}

func TestSessionAuth_MissingSessionID(t *testing.T) {
	store := newMockSessionStore()
	engine := newSessionTestEngine()
	engine.Use(SessionAuth(store))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", nil)
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode())
}

func TestSessionAuth_SessionNotFound(t *testing.T) {
	store := newMockSessionStore()
	engine := newSessionTestEngine()
	engine.Use(SessionAuth(store))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "X-Session-Id", Value: "nonexistent"})
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode())
}
