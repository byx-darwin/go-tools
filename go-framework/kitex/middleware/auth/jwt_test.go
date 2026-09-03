package auth

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gopkg/cloud/metainfo"
	gojwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
	gojwt "github.com/byx-darwin/go-tools/go-auth/jwt"
	"github.com/byx-darwin/go-tools/go-framework/kitex/rpcerror"
)

type jwtTestClaims struct {
	gojwtlib.RegisteredClaims
	UserID string
}

var jwtTestSecret = []byte("01234567890123456789012345678901") // 33 bytes, >= HS256 min

func signTestToken(t *testing.T, userID string) string {
	t.Helper()
	claims := jwtTestClaims{
		RegisteredClaims: gojwtlib.RegisteredClaims{
			ExpiresAt: gojwtlib.NewNumericDate(time.Now().Add(time.Hour)),
		},
		UserID: userID,
	}
	token, err := gojwt.Sign(claims, jwtTestSecret)
	require.NoError(t, err)
	return token
}

func noopEndpoint(gotCtx *context.Context) func(ctx context.Context, req, resp any) error { //nolint:gocritic // *context.Context 用作测试输出参数，捕获中间件传递给 next 的 ctx，非常规业务代码

	return func(ctx context.Context, req, resp any) error {
		if gotCtx != nil {
			*gotCtx = ctx
		}
		return nil
	}
}

func TestJWTAuthServer_Success(t *testing.T) {
	mw := JWTAuthServer[jwtTestClaims](jwtTestSecret)

	token := signTestToken(t, "user-1")
	ctx := metainfo.WithPersistentValue(context.Background(), metaKeyJWTToken, token)
	ctx = metainfo.TransferForward(ctx) // 模拟经过一次 RPC 传输后到达 server

	var gotCtx context.Context
	wrapped := mw(noopEndpoint(&gotCtx))
	err := wrapped(ctx, nil, nil)
	require.NoError(t, err)

	claims, ok := GetClaims[jwtTestClaims](gotCtx)
	require.True(t, ok)
	assert.Equal(t, "user-1", claims.UserID)

	gotToken, ok := GetJWTToken(gotCtx)
	require.True(t, ok)
	assert.Equal(t, token, gotToken)
}

func TestJWTAuthServer_MissingToken(t *testing.T) {
	mw := JWTAuthServer[jwtTestClaims](jwtTestSecret)
	wrapped := mw(noopEndpoint(nil))

	err := wrapped(context.Background(), nil, nil)
	require.Error(t, err)

	adapter, ok := err.(*rpcerror.OopsStatusAdapter)
	require.True(t, ok)
	assert.Equal(t, int32(10020), adapter.BizStatusCode()) // frameworkerror.CodeTokenMissing
}

func TestJWTAuthServer_InvalidToken(t *testing.T) {
	mw := JWTAuthServer[jwtTestClaims](jwtTestSecret)
	ctx := metainfo.WithPersistentValue(context.Background(), metaKeyJWTToken, "not-a-jwt")
	ctx = metainfo.TransferForward(ctx)

	wrapped := mw(noopEndpoint(nil))
	err := wrapped(ctx, nil, nil)
	require.Error(t, err)

	adapter, ok := err.(*rpcerror.OopsStatusAdapter)
	require.True(t, ok)
	assert.Equal(t, int32(autherror.CodeTokenInvalid), adapter.BizStatusCode())
}

func TestJWTAuthClient_UsesTokenProvider(t *testing.T) {
	provider := func(ctx context.Context) (string, bool) { return "provided-token", true }
	mw := JWTAuthClient[jwtTestClaims](provider)

	var gotCtx context.Context
	wrapped := mw(noopEndpoint(&gotCtx))
	err := wrapped(context.Background(), nil, nil)
	require.NoError(t, err)

	// 经过一次模拟传输后，下游应能读到 persistent token。
	forwarded := metainfo.TransferForward(gotCtx)
	token, ok := metainfo.GetPersistentValue(forwarded, metaKeyJWTToken)
	require.True(t, ok)
	assert.Equal(t, "provided-token", token)
}

func TestJWTAuthClient_ReusesCtxTokenOverProvider(t *testing.T) {
	called := false
	provider := func(ctx context.Context) (string, bool) {
		called = true
		return "provider-token", true
	}
	mw := JWTAuthClient[jwtTestClaims](provider)

	ctx := SetJWTToken(context.Background(), "ctx-token")
	var gotCtx context.Context
	wrapped := mw(noopEndpoint(&gotCtx))
	err := wrapped(ctx, nil, nil)
	require.NoError(t, err)
	assert.False(t, called, "provider must not be called when ctx already has a verified token")

	forwarded := metainfo.TransferForward(gotCtx)
	token, ok := metainfo.GetPersistentValue(forwarded, metaKeyJWTToken)
	require.True(t, ok)
	assert.Equal(t, "ctx-token", token)
}

func TestJWTAuthClient_NoTokenAvailable(t *testing.T) {
	mw := JWTAuthClient[jwtTestClaims](nil)
	wrapped := mw(noopEndpoint(nil))

	err := wrapped(context.Background(), nil, nil)
	require.Error(t, err)
}
