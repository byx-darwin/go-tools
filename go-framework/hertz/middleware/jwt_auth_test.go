package middleware

import (
	"context"
	"errors"
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

func issueTestToken(t *testing.T, secret []byte, expiresAt time.Time) string {
	t.Helper()
	const userUUID = "user-123"
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
	token := issueTestToken(t, secret, time.Now().Add(time.Hour))

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
	token := issueTestToken(t, wrongSecret, time.Now().Add(time.Hour))

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
	token := issueTestToken(t, secret, time.Now().Add(-time.Hour))

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

// ── Revocation checker ──

type revocationTestClaims struct {
	gojwt.RegisteredClaims
}

func issueTestTokenWithJTI(t *testing.T, secret []byte, jti string) string {
	t.Helper()
	claims := revocationTestClaims{
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   "user-1",
			ID:        jti,
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token, err := authjwt.Sign(claims, secret)
	require.NoError(t, err)
	return token
}

type mockRevocationChecker struct {
	revoked map[string]bool
	err     error
	calls   int
}

func (m *mockRevocationChecker) IsRevoked(_ context.Context, jti string) (bool, error) {
	m.calls++
	if m.err != nil {
		return false, m.err
	}
	return m.revoked[jti], nil
}

func TestJWTAuth_RevocationChecker_Revoked(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	token := issueTestTokenWithJTI(t, secret, "jti-revoked")
	checker := &mockRevocationChecker{revoked: map[string]bool{"jti-revoked": true}}

	engine := newTestEngine()
	engine.Use(JWTAuth[revocationTestClaims](secret, WithRevocationChecker(checker)))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode())
}

func TestJWTAuth_RevocationChecker_NotRevoked(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	token := issueTestTokenWithJTI(t, secret, "jti-active")
	checker := &mockRevocationChecker{revoked: map[string]bool{}}

	engine := newTestEngine()
	engine.Use(JWTAuth[revocationTestClaims](secret, WithRevocationChecker(checker)))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
}

func TestJWTAuth_RevocationChecker_QueryError(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	token := issueTestTokenWithJTI(t, secret, "jti-x")
	checker := &mockRevocationChecker{err: errors.New("redis down")}

	engine := newTestEngine()
	engine.Use(JWTAuth[revocationTestClaims](secret, WithRevocationChecker(checker)))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	res := w.Result()
	assert.Equal(t, http.StatusInternalServerError, res.StatusCode())
}

func TestJWTAuth_NoRevocationChecker_BehaviorUnchanged(t *testing.T) {
	// 未配置 WithRevocationChecker 时，行为应与旧版完全一致（回归测试）。
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	token := issueTestToken(t, secret, time.Now().Add(time.Hour))

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

func TestJWTAuth_RevocationChecker_NoJTI_FailOpen(t *testing.T) {
	// 安全告诫回归测试：Claims 未设置 jti 时，撤销检查被跳过（fail-open），
	// 请求正常放行，且 checker 完全不会被调用。见 WithRevocationChecker godoc。
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	token := issueTestToken(t, secret, time.Now().Add(time.Hour)) // testClaims 不设置 jti
	checker := &mockRevocationChecker{revoked: map[string]bool{}}

	engine := newTestEngine()
	engine.Use(JWTAuth[testClaims](secret, WithRevocationChecker(checker)))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
	assert.Equal(t, 0, checker.calls, "checker 不应在缺失 jti 时被调用")
}
