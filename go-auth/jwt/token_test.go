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

	// 签发与验证必须使用相同的签名算法，否则方法不匹配会被拒绝。
	parsed, err := Verify[UserClaims](token, testSecret, WithSigningMethod(gojwt.SigningMethodHS512))
	require.NoError(t, err)
	assert.Equal(t, "user-789", parsed.UserUUID)
}

func TestSignDefaultExpiration(t *testing.T) {
	// 不传 WithExpiration 时，应自动使用默认过期时间（2h）。
	claims := UserClaims{UserUUID: "user-default-exp"}

	token, err := Sign(claims, testSecret)
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](token, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "user-default-exp", parsed.UserUUID)
	require.NotNil(t, parsed.ExpiresAt)
	assert.WithinDuration(t, time.Now().Add(2*time.Hour), parsed.ExpiresAt.Time, 5*time.Minute)
}

func TestSignExplicitExpirationOverridesDefault(t *testing.T) {
	// 显式 WithExpiration(1h) 应覆盖默认 2h。
	claims := UserClaims{UserUUID: "user-override-exp"}

	token, err := Sign(claims, testSecret, WithExpiration(time.Hour))
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](token, testSecret)
	require.NoError(t, err)
	require.NotNil(t, parsed.ExpiresAt)
	assert.WithinDuration(t, time.Now().Add(time.Hour), parsed.ExpiresAt.Time, 5*time.Minute)
}

func TestSignPreservesExplicitClaimsExpiration(t *testing.T) {
	// Claims 自带显式 ExpiresAt 时，不应被默认值覆盖。
	futureTime := time.Now().Add(48 * time.Hour)
	claims := UserClaims{
		UserUUID: "user-explicit-claims-exp",
		RegisteredClaims: gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(futureTime),
		},
	}

	token, err := Sign(claims, testSecret)
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](token, testSecret)
	require.NoError(t, err)
	require.NotNil(t, parsed.ExpiresAt)
	assert.WithinDuration(t, futureTime, parsed.ExpiresAt.Time, time.Minute)
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

func TestRefreshCarriesDefaultExpiration(t *testing.T) {
	// Refresh 不传 WithExpiration 时，新 Token 也应使用默认 2h。
	claims := UserClaims{UserUUID: "user-refresh-default"}

	token, err := Sign(claims, testSecret, WithExpiration(time.Hour))
	require.NoError(t, err)

	newToken, err := Refresh[UserClaims](token, testSecret)
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](newToken, testSecret)
	require.NoError(t, err)
	require.NotNil(t, parsed.ExpiresAt)
	assert.WithinDuration(t, time.Now().Add(2*time.Hour), parsed.ExpiresAt.Time, 5*time.Minute)
}

func TestRefreshExplicitExpirationOverridesDefault(t *testing.T) {
	// Refresh 显式 WithExpiration(24h) 必须同时覆盖旧 Token 自带的过期时间与默认 2h。
	claims := UserClaims{UserUUID: "user-refresh-override"}

	// 旧 Token 使用 30 分钟过期（与默认 2h 和新 24h 都不相等，保证三者区分）。
	token, err := Sign(claims, testSecret, WithExpiration(30*time.Minute))
	require.NoError(t, err)

	newToken, err := Refresh[UserClaims](token, testSecret, WithExpiration(24*time.Hour))
	require.NoError(t, err)
	assert.NotEmpty(t, newToken)
	assert.NotEqual(t, token, newToken)

	parsed, err := Verify[UserClaims](newToken, testSecret)
	require.NoError(t, err)
	require.NotNil(t, parsed.ExpiresAt)
	assert.WithinDuration(t, time.Now().Add(24*time.Hour), parsed.ExpiresAt.Time, 5*time.Minute)
}

func TestVerifyWithExplicitSigningMethod(t *testing.T) {
	// 合法的非默认算法流程，必须在调用方显式 pin 期望方法时继续工作。
	claims := UserClaims{UserUUID: "user-hs512-explicit"}

	// 用 HS512 签发。
	token, err := Sign(claims, testSecret,
		WithExpiration(time.Hour),
		WithSigningMethod(gojwt.SigningMethodHS512),
	)
	require.NoError(t, err)

	// 显式声明期望 HS512 应当成功，Claims 完整返回。
	parsed, err := Verify[UserClaims](token, testSecret, WithSigningMethod(gojwt.SigningMethodHS512))
	require.NoError(t, err)
	assert.Equal(t, "user-hs512-explicit", parsed.UserUUID)
}

func TestVerifyAlgorithmConfusion(t *testing.T) {
	claims := UserClaims{UserUUID: "user-confusion"}

	// 攻击场景：Token 实际用 HS256 签发，但调用方期望 RS256。
	// keyfunc 必须在任何 RSA/PEM 解析之前拒绝方法不匹配。
	token, err := Sign(claims, testSecret, WithExpiration(time.Hour))
	require.NoError(t, err)

	_, err = Verify[UserClaims](token, testSecret, WithSigningMethod(gojwt.SigningMethodRS256))
	require.Error(t, err)

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeTokenInvalid, code)
}

func TestVerifyMethodMismatchHS512(t *testing.T) {
	// 默认 pin 为 HS256：HS512 Token 必须被拒绝，防御静默算法降级。
	claims := UserClaims{UserUUID: "user-mismatch"}

	// 用 HS512 签发。
	token, err := Sign(claims, testSecret,
		WithExpiration(time.Hour),
		WithSigningMethod(gojwt.SigningMethodHS512),
	)
	require.NoError(t, err)

	// 裸 Verify 默认期望 HS256，HS512 Token 应被拒绝。
	_, err = Verify[UserClaims](token, testSecret)
	require.Error(t, err)

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeTokenInvalid, code)
}

func TestRefreshWithSigningMethod(t *testing.T) {
	// Refresh 将 opts 透传给 Verify，确保非默认签名算法在刷新往返中存活。
	claims := UserClaims{UserUUID: "user-refresh-hs512"}

	// 用 HS512 签发短期 Token。
	token, err := Sign(claims, testSecret,
		WithExpiration(30*time.Minute),
		WithSigningMethod(gojwt.SigningMethodHS512),
	)
	require.NoError(t, err)

	// Refresh 透传 WithSigningMethod，验证通过后续签。
	newToken, err := Refresh[UserClaims](token, testSecret,
		WithExpiration(24*time.Hour),
		WithSigningMethod(gojwt.SigningMethodHS512),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, newToken)
	assert.NotEqual(t, token, newToken)

	// 新 Token 用 HS512 验证通过。
	parsed, err := Verify[UserClaims](newToken, testSecret, WithSigningMethod(gojwt.SigningMethodHS512))
	require.NoError(t, err)
	assert.Equal(t, "user-refresh-hs512", parsed.UserUUID)
}

// ── Options 防御 ──

func TestWithExpirationZeroIgnored(t *testing.T) {
	// 零值 WithExpiration(0) 被忽略 → 默认 2h 生效。
	claims := UserClaims{UserUUID: "user-zero-exp"}

	token, err := Sign(claims, testSecret, WithExpiration(0))
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](token, testSecret)
	require.NoError(t, err)
	require.NotNil(t, parsed.ExpiresAt)
	assert.WithinDuration(t, time.Now().Add(2*time.Hour), parsed.ExpiresAt.Time, 5*time.Minute)
}

func TestWithExpirationNegativeIgnored(t *testing.T) {
	// 负值 WithExpiration(-time.Hour) 被忽略 → 默认 2h 生效。
	claims := UserClaims{UserUUID: "user-neg-exp"}

	token, err := Sign(claims, testSecret, WithExpiration(-time.Hour))
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](token, testSecret)
	require.NoError(t, err)
	require.NotNil(t, parsed.ExpiresAt)
	assert.WithinDuration(t, time.Now().Add(2*time.Hour), parsed.ExpiresAt.Time, 5*time.Minute)
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
