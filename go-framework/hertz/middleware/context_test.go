package middleware

import (
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/stretchr/testify/assert"

	"github.com/byx-darwin/go-tools/go-auth/session"
)

func TestSetGetClaims(t *testing.T) {
	type testClaims struct {
		Name string
	}

	c := app.NewContext(0)
	claims := &testClaims{Name: "alice"}

	got, ok := GetClaims[testClaims](c)
	assert.False(t, ok)
	assert.Nil(t, got)

	SetClaims(c, claims)
	got, ok = GetClaims[testClaims](c)
	assert.True(t, ok)
	assert.Equal(t, "alice", got.Name)
}

func TestSetGetSession(t *testing.T) {
	c := app.NewContext(0)

	got, ok := GetSession(c)
	assert.False(t, ok)
	assert.Nil(t, got)

	s := &session.Session{ID: "s1", UserUUID: "u1", ExpiresAt: time.Now().Add(time.Hour)}
	SetSession(c, s)
	got, ok = GetSession(c)
	assert.True(t, ok)
	assert.Equal(t, s, got)
}
