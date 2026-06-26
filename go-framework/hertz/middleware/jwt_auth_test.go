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
	"github.com/stretchr/testify/require"

	gojwt "github.com/golang-jwt/jwt/v5"

	authjwt "github.com/byx-darwin/go-tools/go-auth/jwt"
)

type testClaims struct {
	UserUUID string `json:"user_uuid"`
	gojwt.RegisteredClaims
}

func issueTestToken(t *testing.T, secret []byte, userUUID string, expiresAt time.Time) string {
	t.Helper()
	claims := testClaims{
		UserUUID: userUUID,
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   userUUID,
			ExpiresAt: gojwt.NewNumericDate(expiresAt),
		},
	}
	token, err := authjwt.Sign(claims, secret)
	require.NoError(t, err)
	return token
}

func newTestEngine() *route.Engine {
	opt := config.NewOptions([]config.Option{})
	return route.NewEngine(opt)
}

func TestJWTAuth_Success(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	token := issueTestToken(t, secret, "user-123", time.Now().Add(time.Hour))

	engine := newTestEngine()
	engine.Use(JWTAuth[testClaims](secret))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		claims, ok := GetClaims[testClaims](c)
		assert.True(t, ok)
		c.JSON(http.StatusOK, map[string]string{"user": claims.UserUUID})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
	assert.Contains(t, string(res.Body()), "user-123")
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	engine := newTestEngine()
	engine.Use(JWTAuth[testClaims](secret))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", nil)
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode())
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	engine := newTestEngine()
	engine.Use(JWTAuth[testClaims](secret))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer invalid.token.value"})
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode())
}

func TestJWTAuth_WrongSecret(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	wrongSecret := []byte("wrong-secret-key-32bytes-long!!!")
	token := issueTestToken(t, wrongSecret, "user-123", time.Now().Add(time.Hour))

	engine := newTestEngine()
	engine.Use(JWTAuth[testClaims](secret))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode())
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	token := issueTestToken(t, secret, "user-123", time.Now().Add(-time.Hour))

	engine := newTestEngine()
	engine.Use(JWTAuth[testClaims](secret))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode())
}

func TestJWTAuth_NonBearerPrefix(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	engine := newTestEngine()
	engine.Use(JWTAuth[testClaims](secret))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Basic dXNlcjpwYXNz"})
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode())
}

func TestExtractBearerToken(t *testing.T) {
	c := app.NewContext(0)

	c.Request.Header.Set("Authorization", "Bearer my-token")
	assert.Equal(t, "my-token", extractBearerToken(c))

	c.Request.Header.Set("Authorization", "Bearer   trimmed  ")
	assert.Equal(t, "trimmed", extractBearerToken(c))

	c.Request.Header.Set("Authorization", "Basic abc")
	assert.Equal(t, "", extractBearerToken(c))

	c.Request.Header.Set("Authorization", "Bearer ")
	assert.Equal(t, "", extractBearerToken(c))

	c.Request.Header.Set("Authorization", "")
	assert.Equal(t, "", extractBearerToken(c))
}
