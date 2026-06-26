package jwt

import (
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// UserClaims 自定义 Claims，用于测试。
type UserClaims struct {
	UserUUID string `json:"user_uuid"`
	Role     string `json:"role,omitempty"`
	gojwt.RegisteredClaims
}

var testSecret = []byte("test-secret-key-for-jwt")

// ── Sign + Verify 正常流程 ──

func TestSignAndVerify(t *testing.T) {
	claims := UserClaims{
		UserUUID: "user-123",
		Role:     "admin",
	}

	token, err := Sign(claims, testSecret, WithExpiration(time.Hour))
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	parsed, err := Verify[UserClaims](token, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "user-123", parsed.UserUUID)
	assert.Equal(t, "admin", parsed.Role)
	assert.NotNil(t, parsed.ExpiresAt)
}

func TestSignWithIssuer(t *testing.T) {
	claims := UserClaims{UserUUID: "user-456"}

	token, err := Sign(claims, testSecret,
		WithExpiration(time.Hour),
		WithIssuer("go-auth"),
	)
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](token, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "go-auth", parsed.Issuer)
	assert.Equal(t, "user-456", parsed.UserUUID)
}

func TestSignWithCustomSigningMethod(t *testing.T) {
	claims := UserClaims{UserUUID: "user-789"}

	token, err := Sign(claims, testSecret,
		WithExpiration(time.Hour),
		WithSigningMethod(gojwt.SigningMethodHS512),
	)
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](token, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "user-789", parsed.UserUUID)
}

func TestSignWithoutExpiration(t *testing.T) {
	claims := UserClaims{UserUUID: "user-noexp"}

	token, err := Sign(claims, testSecret)
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](token, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "user-noexp", parsed.UserUUID)
	// 没设过期时间时 ExpiresAt 为 nil。
	assert.Nil(t, parsed.ExpiresAt)
}

// ── Verify 失败场景 ──

func TestVerifyExpiredToken(t *testing.T) {
	claims := UserClaims{
		UserUUID: "user-exp",
		RegisteredClaims: gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}

	token, err := Sign(claims, testSecret)
	require.NoError(t, err)

	_, err = Verify[UserClaims](token, testSecret)
	require.Error(t, err)

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeTokenExpired, code)
}

func TestVerifySignatureMismatch(t *testing.T) {
	claims := UserClaims{UserUUID: "user-badsig"}

	token, err := Sign(claims, testSecret, WithExpiration(time.Hour))
	require.NoError(t, err)

	wrongSecret := []byte("wrong-secret-key")
	_, err = Verify[UserClaims](token, wrongSecret)
	require.Error(t, err)

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeTokenInvalid, code)
}

func TestVerifyInvalidToken(t *testing.T) {
	_, err := Verify[UserClaims]("not-a-valid-token", testSecret)
	require.Error(t, err)

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeTokenInvalid, code)
}

func TestVerifyEmptyToken(t *testing.T) {
	_, err := Verify[UserClaims]("", testSecret)
	require.Error(t, err)
}

// ── Refresh ──

func TestRefresh(t *testing.T) {
	claims := UserClaims{
		UserUUID: "user-refresh",
		Role:     "user",
	}

	// 先签发一个短期 Token。
	token, err := Sign(claims, testSecret, WithExpiration(30*time.Minute))
	require.NoError(t, err)

	// 刷新为长期 Token。
	newToken, err := Refresh[UserClaims](token, testSecret, WithExpiration(24*time.Hour))
	require.NoError(t, err)
	assert.NotEmpty(t, newToken)
	assert.NotEqual(t, token, newToken)

	// 验证新 Token 的 Claims 数据保持不变。
	parsed, err := Verify[UserClaims](newToken, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "user-refresh", parsed.UserUUID)
	assert.Equal(t, "user", parsed.Role)
	assert.NotNil(t, parsed.ExpiresAt)
}

func TestRefreshExpiredToken(t *testing.T) {
	claims := UserClaims{
		UserUUID: "user-exp-refresh",
		RegisteredClaims: gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}

	token, err := Sign(claims, testSecret)
	require.NoError(t, err)

	_, err = Refresh[UserClaims](token, testSecret, WithExpiration(time.Hour))
	require.Error(t, err)
}

func TestRefreshWithIssuer(t *testing.T) {
	claims := UserClaims{
		UserUUID: "user-issuer-refresh",
	}

	token, err := Sign(claims, testSecret, WithExpiration(time.Hour))
	require.NoError(t, err)

	newToken, err := Refresh[UserClaims](token, testSecret,
		WithExpiration(24*time.Hour),
		WithIssuer("new-issuer"),
	)
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](newToken, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "new-issuer", parsed.Issuer)
	assert.Equal(t, "user-issuer-refresh", parsed.UserUUID)
}

// ── Options 防御 ──

func TestWithExpirationZeroIgnored(t *testing.T) {
	claims := UserClaims{UserUUID: "user-zero-exp"}

	token, err := Sign(claims, testSecret, WithExpiration(0))
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](token, testSecret)
	require.NoError(t, err)
	assert.Nil(t, parsed.ExpiresAt)
}

func TestWithExpirationNegativeIgnored(t *testing.T) {
	claims := UserClaims{UserUUID: "user-neg-exp"}

	token, err := Sign(claims, testSecret, WithExpiration(-time.Hour))
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](token, testSecret)
	require.NoError(t, err)
	assert.Nil(t, parsed.ExpiresAt)
}

func TestWithIssuerEmptyIgnored(t *testing.T) {
	claims := UserClaims{UserUUID: "user-empty-issuer"}

	token, err := Sign(claims, testSecret, WithIssuer(""))
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](token, testSecret)
	require.NoError(t, err)
	assert.Empty(t, parsed.Issuer)
}

func TestWithSigningMethodNilIgnored(t *testing.T) {
	claims := UserClaims{UserUUID: "user-nil-method"}

	token, err := Sign(claims, testSecret, WithSigningMethod(nil))
	require.NoError(t, err)

	// 应使用默认 HS256 签发。
	parsed, err := Verify[UserClaims](token, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "user-nil-method", parsed.UserUUID)
}

// ── Sign 不支持的类型 ──

func TestSignNonClaimsType(t *testing.T) {
	// int 不实现 jwt.Claims，应返回错误。
	_, err := Sign(42, testSecret)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not implement jwt.Claims")
}
