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

	"github.com/byx-darwin/go-tools/go-auth/device"
	authjwt "github.com/byx-darwin/go-tools/go-auth/jwt"
)

type deviceTestClaims struct {
	DeviceID string `json:"device_id"`
	JTI      string `json:"jti"`
	gojwt.RegisteredClaims
}

type mockDeviceStore struct {
	devices map[string]map[string]string
}

func newMockDeviceStore() *mockDeviceStore {
	return &mockDeviceStore{devices: make(map[string]map[string]string)}
}

func (m *mockDeviceStore) AddDevice(_ context.Context, userUUID, deviceID, jti string, _ int) ([]device.Device, error) {
	if m.devices[userUUID] == nil {
		m.devices[userUUID] = make(map[string]string)
	}
	m.devices[userUUID][deviceID] = jti
	return nil, nil
}

func (m *mockDeviceStore) CheckDevice(_ context.Context, userUUID, deviceID, jti string) (bool, error) {
	devs, ok := m.devices[userUUID]
	if !ok {
		return false, nil
	}
	storedJTI, ok := devs[deviceID]
	if !ok {
		return false, nil
	}
	return storedJTI == jti, nil
}

func (m *mockDeviceStore) RemoveDevice(_ context.Context, userUUID, deviceID string) error {
	if devs, ok := m.devices[userUUID]; ok {
		delete(devs, deviceID)
	}
	return nil
}

func (m *mockDeviceStore) RemoveAllDevices(_ context.Context, userUUID string) error {
	delete(m.devices, userUUID)
	return nil
}

func (m *mockDeviceStore) ListDevices(_ context.Context, userUUID string) ([]device.Device, error) {
	return nil, nil
}

func issueDeviceToken(t *testing.T, secret []byte, userUUID, deviceID, jti string) string { //nolint:unparam // userUUID 保留灵活性供未来测试使用
	t.Helper()
	claims := deviceTestClaims{
		DeviceID: deviceID,
		JTI:      jti,
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   userUUID,
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token, err := authjwt.Sign(claims, secret)
	require.NoError(t, err)
	return token
}

func newDeviceTestEngine() *route.Engine {
	opt := config.NewOptions([]config.Option{})
	return route.NewEngine(opt)
}

func TestDeviceAuth_Success(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	store := newMockDeviceStore()
	_, _ = store.AddDevice(context.Background(), "user-1", "dev-1", "jti-1", 5)

	token := issueDeviceToken(t, secret, "user-1", "dev-1", "jti-1")

	extract := func(claims any) (string, string, string) {
		c := claims.(*deviceTestClaims)
		return c.Subject, c.DeviceID, c.JTI
	}

	engine := newDeviceTestEngine()
	engine.Use(JWTAuth[deviceTestClaims](secret))
	engine.Use(DeviceAuth(store, 5, extract))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
}

func TestDeviceAuth_DeviceNotFound(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	store := newMockDeviceStore()

	token := issueDeviceToken(t, secret, "user-1", "dev-unknown", "jti-1")

	extract := func(claims any) (string, string, string) {
		c := claims.(*deviceTestClaims)
		return c.Subject, c.DeviceID, c.JTI
	}

	engine := newDeviceTestEngine()
	engine.Use(JWTAuth[deviceTestClaims](secret))
	engine.Use(DeviceAuth(store, 5, extract))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode())
}

func TestDeviceAuth_JTIMismatch(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	store := newMockDeviceStore()
	_, _ = store.AddDevice(context.Background(), "user-1", "dev-1", "jti-original", 5)

	token := issueDeviceToken(t, secret, "user-1", "dev-1", "jti-different")

	extract := func(claims any) (string, string, string) {
		c := claims.(*deviceTestClaims)
		return c.Subject, c.DeviceID, c.JTI
	}

	engine := newDeviceTestEngine()
	engine.Use(JWTAuth[deviceTestClaims](secret))
	engine.Use(DeviceAuth(store, 5, extract))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode())
}

func TestDeviceAuth_NoClaimsInContext(t *testing.T) {
	store := newMockDeviceStore()

	extract := func(claims any) (string, string, string) {
		c := claims.(*deviceTestClaims)
		return c.Subject, c.DeviceID, c.JTI
	}

	engine := newDeviceTestEngine()
	engine.Use(DeviceAuth(store, 5, extract))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", nil)
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode())
}

func TestDeviceAuth_ExtractReturnsEmptyFields(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	store := newMockDeviceStore()

	token := issueDeviceToken(t, secret, "user-1", "dev-1", "jti-1")

	extract := func(_ any) (string, string, string) {
		return "", "", ""
	}

	engine := newDeviceTestEngine()
	engine.Use(JWTAuth[deviceTestClaims](secret))
	engine.Use(DeviceAuth(store, 5, extract))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})
	res := w.Result()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode())
}
