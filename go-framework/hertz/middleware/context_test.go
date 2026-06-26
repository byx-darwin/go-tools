package middleware

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/stretchr/testify/assert"
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

	s := &fakeSession{ID: "s1", UserUUID: "u1"}
	SetSession(c, s)
	got, ok = GetSession(c)
	assert.True(t, ok)
	assert.Equal(t, s, got)
}

type fakeSession struct {
	ID       string
	UserUUID string
}
