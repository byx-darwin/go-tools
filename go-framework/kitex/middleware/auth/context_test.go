package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/byx-darwin/go-tools/go-auth/session"
	"github.com/byx-darwin/go-tools/go-framework/kitex/rpcerror"
)

type testClaims struct {
	Subject string
}

func TestSetGetClaims(t *testing.T) {
	ctx := context.Background()

	got, ok := GetClaims[testClaims](ctx)
	assert.False(t, ok)
	assert.Nil(t, got)

	claims := &testClaims{Subject: "user-1"}
	ctx = SetClaims(ctx, claims)

	got, ok = GetClaims[testClaims](ctx)
	assert.True(t, ok)
	assert.Equal(t, claims, got)
}

func TestSetGetJWTToken(t *testing.T) {
	ctx := context.Background()

	got, ok := GetJWTToken(ctx)
	assert.False(t, ok)
	assert.Empty(t, got)

	ctx = SetJWTToken(ctx, "token-abc")

	got, ok = GetJWTToken(ctx)
	assert.True(t, ok)
	assert.Equal(t, "token-abc", got)
}

func TestSetGetSession(t *testing.T) {
	ctx := context.Background()

	got, ok := GetSession(ctx)
	assert.False(t, ok)
	assert.Nil(t, got)

	s := &session.Session{ID: "sess-1", UserUUID: "user-1", ExpiresAt: time.Now()}
	ctx = SetSession(ctx, s)

	got, ok = GetSession(ctx)
	assert.True(t, ok)
	assert.Equal(t, s, got)
}

func TestSetGetSessionID(t *testing.T) {
	ctx := context.Background()

	got, ok := GetSessionID(ctx)
	assert.False(t, ok)
	assert.Empty(t, got)

	ctx = SetSessionID(ctx, "sess-1")

	got, ok = GetSessionID(ctx)
	assert.True(t, ok)
	assert.Equal(t, "sess-1", got)
}

func TestBizError_WrapsAsOopsStatusAdapter(t *testing.T) {
	inner := errors.New("inner error")
	err := bizError(inner)

	adapter, ok := err.(*rpcerror.OopsStatusAdapter)
	assert.True(t, ok)
	assert.Equal(t, inner, adapter.Err)
}
