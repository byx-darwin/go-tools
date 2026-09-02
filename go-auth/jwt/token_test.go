package jwt

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
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

var testSecret = []byte("test-secret-key-for-jwt-tests-must-be-at-least-64-bytes-long-okk")

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

	wrongSecret := []byte("wrong-secret-key-must-also-be-at-least-32-bytes-long")
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
	newToken, err := Refresh[UserClaims](context.Background(), token, testSecret, newFakeRevocationStore(), WithExpiration(24*time.Hour))
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

	_, err = Refresh[UserClaims](context.Background(), token, testSecret, newFakeRevocationStore(), WithExpiration(time.Hour))
	require.Error(t, err)
}

func TestRefreshWithIssuer(t *testing.T) {
	claims := UserClaims{
		UserUUID: "user-issuer-refresh",
	}

	token, err := Sign(claims, testSecret, WithExpiration(time.Hour))
	require.NoError(t, err)

	newToken, err := Refresh[UserClaims](context.Background(), token, testSecret, newFakeRevocationStore(),
		WithExpiration(24*time.Hour),
		WithIssuer("new-issuer"),
	)
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](newToken, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "new-issuer", parsed.Issuer)
	assert.Equal(t, "user-issuer-refresh", parsed.UserUUID)
}

func TestRefreshWithIssuerAndExpectedIssuerCoexist(t *testing.T) {
	// WithIssuer（作用于 Refresh 内部 Sign）与 WithExpectedIssuer（作用于
	// Refresh 内部 Verify）语义独立：同时传入不应互相干扰。
	claims := UserClaims{UserUUID: "user-issuer-coexist"}

	oldToken, err := Sign(claims, testSecret, WithExpiration(time.Hour), WithIssuer("old-app"))
	require.NoError(t, err)

	newToken, err := Refresh[UserClaims](context.Background(), oldToken, testSecret, newFakeRevocationStore(),
		WithExpectedIssuer("old-app"),
		WithIssuer("new-app"),
	)
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](newToken, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "new-app", parsed.Issuer)
	assert.Equal(t, "user-issuer-coexist", parsed.UserUUID)
}

func TestRefreshWithExpectedIssuerMismatchFails(t *testing.T) {
	// WithExpectedIssuer 校验的是旧 Token 的 issuer；不匹配时 Refresh 内部
	// Verify 先失败。Refresh 用 oops.With("jwt.Refresh").Code(CodeJWTRefreshFailed).Wrap(err)
	// 包装该错误，但 goerror.Extract 沿错误链返回最内层（原始）错误的 Code，
	// 而不是外层 Wrap 时设置的 Code——因此顶层观测到的仍是 Verify 产生的
	// CodeTokenInvalid，而非 CodeJWTRefreshFailed。此行为此前未被任何测试
	// 验证过，本用例把它钉住。
	claims := UserClaims{UserUUID: "user-issuer-coexist-mismatch"}

	oldToken, err := Sign(claims, testSecret, WithExpiration(time.Hour), WithIssuer("old-app"))
	require.NoError(t, err)

	_, err = Refresh[UserClaims](context.Background(), oldToken, testSecret, newFakeRevocationStore(),
		WithExpectedIssuer("wrong-app"),
		WithIssuer("new-app"),
	)
	require.Error(t, err)

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeTokenInvalid, code)
}

func TestRefreshCarriesDefaultExpiration(t *testing.T) {
	// Refresh 不传 WithExpiration 时，新 Token 也应使用默认 2h。
	claims := UserClaims{UserUUID: "user-refresh-default"}

	token, err := Sign(claims, testSecret, WithExpiration(time.Hour))
	require.NoError(t, err)

	newToken, err := Refresh[UserClaims](context.Background(), token, testSecret, newFakeRevocationStore())
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

	newToken, err := Refresh[UserClaims](context.Background(), token, testSecret, newFakeRevocationStore(), WithExpiration(24*time.Hour))
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
	newToken, err := Refresh[UserClaims](context.Background(), token, testSecret, newFakeRevocationStore(),
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

func TestVerifyWithExpectedIssuerMatch(t *testing.T) {
	claims := UserClaims{UserUUID: "user-issuer-match"}

	token, err := Sign(claims, testSecret, WithExpiration(time.Hour), WithIssuer("myapp"))
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](token, testSecret, WithExpectedIssuer("myapp"))
	require.NoError(t, err)
	assert.Equal(t, "myapp", parsed.Issuer)
	assert.Equal(t, "user-issuer-match", parsed.UserUUID)
}

func TestVerifyWithExpectedIssuerMismatch(t *testing.T) {
	claims := UserClaims{UserUUID: "user-issuer-mismatch"}

	token, err := Sign(claims, testSecret, WithExpiration(time.Hour), WithIssuer("other-app"))
	require.NoError(t, err)

	_, err = Verify[UserClaims](token, testSecret, WithExpectedIssuer("myapp"))
	require.Error(t, err)

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeTokenInvalid, code)
}

func TestVerifyWithExpectedIssuerMissingClaim(t *testing.T) {
	// Token signed without WithIssuer carries no iss claim at all.
	claims := UserClaims{UserUUID: "user-issuer-missing"}

	token, err := Sign(claims, testSecret, WithExpiration(time.Hour))
	require.NoError(t, err)

	_, err = Verify[UserClaims](token, testSecret, WithExpectedIssuer("myapp"))
	require.Error(t, err)

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeTokenInvalid, code)
}

func TestVerifyWithoutExpectedIssuerIgnoresTokenIssuer(t *testing.T) {
	// Backward-compat regression: without WithExpectedIssuer, Verify must not
	// enforce any issuer check, regardless of what issuer the token carries.
	claims := UserClaims{UserUUID: "user-issuer-unchecked"}

	token, err := Sign(claims, testSecret, WithExpiration(time.Hour), WithIssuer("some-app"))
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](token, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "some-app", parsed.Issuer)
	assert.Equal(t, "user-issuer-unchecked", parsed.UserUUID)
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

// ── ExtractJTI ──

type extractJTIClaims struct {
	gojwt.RegisteredClaims
}

func TestExtractJTI(t *testing.T) {
	t.Run("direct embed with ID", func(t *testing.T) {
		claims := &extractJTIClaims{RegisteredClaims: gojwt.RegisteredClaims{ID: "jti-123"}}
		jti, ok := ExtractJTI(claims)
		assert.True(t, ok)
		assert.Equal(t, "jti-123", jti)
	})

	t.Run("no ID set", func(t *testing.T) {
		claims := &extractJTIClaims{}
		jti, ok := ExtractJTI(claims)
		assert.False(t, ok)
		assert.Empty(t, jti)
	})

	t.Run("not a claims type", func(t *testing.T) {
		jti, ok := ExtractJTI("not-claims")
		assert.False(t, ok)
		assert.Empty(t, jti)
	})

	t.Run("nil claims", func(t *testing.T) {
		jti, ok := ExtractJTI(nil)
		assert.False(t, ok)
		assert.Empty(t, jti)
	})

	t.Run("MapClaims not supported", func(t *testing.T) {
		mc := gojwt.MapClaims{"jti": "abc"}
		jti, ok := ExtractJTI(&mc)
		assert.False(t, ok)
		assert.Empty(t, jti)
	})
}

// ── 非对称算法（RS256/ES256）真实签发/验证 ──

func generateTestRSAKey(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key, &key.PublicKey
}

func generateTestECDSAKey(t *testing.T) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return key, &key.PublicKey
}

func TestSignAndVerifyRS256(t *testing.T) {
	priv, pub := generateTestRSAKey(t)
	claims := UserClaims{UserUUID: "user-rs256"}

	token, err := Sign(claims, priv,
		WithExpiration(time.Hour),
		WithSigningMethod(gojwt.SigningMethodRS256),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	parsed, err := Verify[UserClaims](token, pub, WithSigningMethod(gojwt.SigningMethodRS256))
	require.NoError(t, err)
	assert.Equal(t, "user-rs256", parsed.UserUUID)
}

func TestSignAndVerifyES256(t *testing.T) {
	priv, pub := generateTestECDSAKey(t)
	claims := UserClaims{UserUUID: "user-es256"}

	token, err := Sign(claims, priv,
		WithExpiration(time.Hour),
		WithSigningMethod(gojwt.SigningMethodES256),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	parsed, err := Verify[UserClaims](token, pub, WithSigningMethod(gojwt.SigningMethodES256))
	require.NoError(t, err)
	assert.Equal(t, "user-es256", parsed.UserUUID)
}

// ── 密钥类型不匹配 ──

func TestSignKeyTypeMismatch(t *testing.T) {
	// RS256 要求 *rsa.PrivateKey，传入 []byte 应返回明确错误而非 panic。
	claims := UserClaims{UserUUID: "user-mismatch-sign"}

	_, err := Sign(claims, testSecret, WithSigningMethod(gojwt.SigningMethodRS256))
	require.Error(t, err)

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeJWTKeyTypeMismatch, code)
}

func TestVerifyKeyTypeMismatch(t *testing.T) {
	// Token 确实用 RS256 签发、调用方也声明期望 RS256，
	// 但传入了错误类型的密钥（[]byte 而非 *rsa.PublicKey）。
	priv, _ := generateTestRSAKey(t)
	claims := UserClaims{UserUUID: "user-mismatch-verify"}

	token, err := Sign(claims, priv,
		WithExpiration(time.Hour),
		WithSigningMethod(gojwt.SigningMethodRS256),
	)
	require.NoError(t, err)

	_, err = Verify[UserClaims](token, testSecret, WithSigningMethod(gojwt.SigningMethodRS256))
	require.Error(t, err)

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeJWTKeyTypeMismatch, code)
}

// ── Refresh 轮换与复用检测 ──

// fakeRevocationStore 是 revocation.Store 的内存测试替身，避免 go-auth 反向依赖
// go-middleware。
type fakeRevocationStore struct {
	revoked map[string]bool

	isRevokedErr error
	revokeErr    error
}

func newFakeRevocationStore() *fakeRevocationStore {
	return &fakeRevocationStore{revoked: make(map[string]bool)}
}

func (s *fakeRevocationStore) IsRevoked(_ context.Context, jti string) (bool, error) {
	if s.isRevokedErr != nil {
		return false, s.isRevokedErr
	}
	return s.revoked[jti], nil
}

func (s *fakeRevocationStore) Revoke(_ context.Context, jti string, _ time.Duration) error {
	if s.revokeErr != nil {
		return s.revokeErr
	}
	s.revoked[jti] = true
	return nil
}

type refreshRotationClaims struct {
	UserUUID string `json:"user_uuid"`
	gojwt.RegisteredClaims
}

func TestRefreshRotatesJTI(t *testing.T) {
	store := newFakeRevocationStore()
	claims := refreshRotationClaims{
		UserUUID:         "user-rotate",
		RegisteredClaims: gojwt.RegisteredClaims{ID: "jti-original"},
	}

	token, err := Sign(claims, testSecret, WithExpiration(30*time.Minute))
	require.NoError(t, err)

	newToken, err := Refresh[refreshRotationClaims](context.Background(), token, testSecret, store, WithExpiration(time.Hour))
	require.NoError(t, err)
	assert.NotEmpty(t, newToken)

	parsed, err := Verify[refreshRotationClaims](newToken, testSecret)
	require.NoError(t, err)
	assert.Equal(t, "user-rotate", parsed.UserUUID)
	assert.NotEqual(t, "jti-original", parsed.ID, "刷新后必须生成新 JTI")
	assert.True(t, store.revoked["jti-original"], "旧 JTI 必须被标记为已撤销")
}

func TestRefreshReuseDetection(t *testing.T) {
	store := newFakeRevocationStore()
	claims := refreshRotationClaims{
		UserUUID:         "user-reuse",
		RegisteredClaims: gojwt.RegisteredClaims{ID: "jti-reuse"},
	}

	token, err := Sign(claims, testSecret, WithExpiration(30*time.Minute))
	require.NoError(t, err)

	_, err = Refresh[refreshRotationClaims](context.Background(), token, testSecret, store, WithExpiration(time.Hour))
	require.NoError(t, err)

	// 用同一个旧 token 再次 Refresh：旧 JTI 已被撤销，必须判定为复用攻击。
	_, err = Refresh[refreshRotationClaims](context.Background(), token, testSecret, store, WithExpiration(time.Hour))
	require.Error(t, err)

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeTokenRevoked, code)
}

func TestRefreshWithoutJTISkipsRotation(t *testing.T) {
	// Claims 未携带 JTI 时，行为与变更前一致：不触碰 store，不报错。
	store := newFakeRevocationStore()
	claims := UserClaims{UserUUID: "user-no-jti"}

	token, err := Sign(claims, testSecret, WithExpiration(30*time.Minute))
	require.NoError(t, err)

	newToken, err := Refresh[UserClaims](context.Background(), token, testSecret, store, WithExpiration(time.Hour))
	require.NoError(t, err)
	assert.NotEmpty(t, newToken)
	assert.Empty(t, store.revoked)
}

func TestRefreshIsRevokedError(t *testing.T) {
	store := newFakeRevocationStore()
	store.isRevokedErr = assert.AnError
	claims := refreshRotationClaims{
		UserUUID:         "user-isrevoked-err",
		RegisteredClaims: gojwt.RegisteredClaims{ID: "jti-isrevoked-err"},
	}

	token, err := Sign(claims, testSecret, WithExpiration(30*time.Minute))
	require.NoError(t, err)

	_, err = Refresh[refreshRotationClaims](context.Background(), token, testSecret, store, WithExpiration(time.Hour))
	require.Error(t, err, "IsRevoked 出错必须 fail-closed，拒绝刷新")
}

func TestRefreshRevokeError(t *testing.T) {
	store := newFakeRevocationStore()
	store.revokeErr = assert.AnError
	claims := refreshRotationClaims{
		UserUUID:         "user-revoke-err",
		RegisteredClaims: gojwt.RegisteredClaims{ID: "jti-revoke-err"},
	}

	token, err := Sign(claims, testSecret, WithExpiration(30*time.Minute))
	require.NoError(t, err)

	_, err = Refresh[refreshRotationClaims](context.Background(), token, testSecret, store, WithExpiration(time.Hour))
	require.Error(t, err, "Revoke 出错必须 fail-closed，拒绝刷新")
}

func TestRefreshNilStoreWithJTIFailsClosed(t *testing.T) {
	// Claims 携带 JTI 但 store 为 nil：必须返回错误而非 panic，且不应签发新 Token。
	claims := refreshRotationClaims{
		UserUUID:         "user-nil-store",
		RegisteredClaims: gojwt.RegisteredClaims{ID: "jti-nil-store"},
	}

	token, err := Sign(claims, testSecret, WithExpiration(30*time.Minute))
	require.NoError(t, err)

	newToken, err := Refresh[refreshRotationClaims](context.Background(), token, testSecret, nil, WithExpiration(time.Hour))
	require.Error(t, err, "携带 JTI 但 store 为 nil 时必须 fail-closed，而不是 panic")
	assert.Empty(t, newToken, "拒绝刷新时不应签发新 Token")

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeJWTRefreshFailed, code)
}

func TestRefreshMissingExpiresAtFailsClosed(t *testing.T) {
	// exp 并非 JWT 规范强制字段。构造一个带 JTI 但不带 ExpiresAt 的 Token：
	// 必须绕过本包 Sign() 的默认过期填充逻辑，直接用 gojwt 签发。
	claims := &refreshRotationClaims{
		UserUUID:         "user-no-exp",
		RegisteredClaims: gojwt.RegisteredClaims{ID: "jti-no-exp"},
	}
	require.Nil(t, claims.ExpiresAt)

	rawToken := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	token, err := rawToken.SignedString(testSecret)
	require.NoError(t, err)

	store := newFakeRevocationStore()
	newToken, err := Refresh[refreshRotationClaims](context.Background(), token, testSecret, store, WithExpiration(time.Hour))
	require.Error(t, err, "缺少 ExpiresAt 时必须 fail-closed，而不是 panic")
	assert.Empty(t, newToken, "拒绝刷新时不应签发新 Token")
	assert.Empty(t, store.revoked, "Revoke 不应被调用")
}

// ── HMAC 密钥强度校验 ──

func TestSignRejectsWeakHMACSecret(t *testing.T) {
	claims := UserClaims{UserUUID: "user-weak-secret"}
	weakSecret := []byte("too-short")

	_, err := Sign(claims, weakSecret)
	require.Error(t, err)

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeJWTWeakSecret, code)
}

func TestVerifyRejectsWeakHMACSecret(t *testing.T) {
	// 用合规密钥签发，但调用方随后误用弱密钥去验证：必须在类型/长度校验阶段
	// 就拒绝，而不是先尝试验证签名。
	claims := UserClaims{UserUUID: "user-weak-secret-verify"}

	token, err := Sign(claims, testSecret, WithExpiration(time.Hour))
	require.NoError(t, err)

	weakSecret := []byte("too-short")
	_, err = Verify[UserClaims](token, weakSecret)
	require.Error(t, err)

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeJWTWeakSecret, code)
}

func TestSignAcceptsHMACSecretAtExactMinimumLength(t *testing.T) {
	// 边界值：长度恰好等于哈希输出长度（HS256=32 字节）应被接受，而不是被拒绝。
	claims := UserClaims{UserUUID: "user-min-len-secret"}
	minSecret := make([]byte, 32)

	token, err := Sign(claims, minSecret, WithExpiration(time.Hour))
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](token, minSecret)
	require.NoError(t, err)
	assert.Equal(t, "user-min-len-secret", parsed.UserUUID)
}

func TestSignRejectsHMACSecretOneByteBelowMinimum(t *testing.T) {
	// 边界值：长度比哈希输出长度少 1 字节（31 字节）应被拒绝。
	claims := UserClaims{UserUUID: "user-below-min-secret"}
	tooShort := make([]byte, 31)

	_, err := Sign(claims, tooShort)
	require.Error(t, err)

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeJWTWeakSecret, code)
}

func TestSignRejectsWeakHS512Secret(t *testing.T) {
	// HS512 要求 >=64 字节；32 字节对 HS256 合规，但对 HS512 不合规。
	claims := UserClaims{UserUUID: "user-weak-hs512"}
	secret32 := make([]byte, 32)

	_, err := Sign(claims, secret32, WithSigningMethod(gojwt.SigningMethodHS512))
	require.Error(t, err)

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeJWTWeakSecret, code)
}

// ── GenerateSecret ──

func TestGenerateSecretLengths(t *testing.T) {
	tests := []struct {
		name       string
		method     *gojwt.SigningMethodHMAC
		wantLength int
	}{
		{"HS256", gojwt.SigningMethodHS256, 32},
		{"HS384", gojwt.SigningMethodHS384, 48},
		{"HS512", gojwt.SigningMethodHS512, 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secret, err := GenerateSecret(tt.method)
			require.NoError(t, err)
			assert.Len(t, secret, tt.wantLength)
		})
	}
}

func TestGenerateSecretUsableForSignAndVerify(t *testing.T) {
	secret, err := GenerateSecret(gojwt.SigningMethodHS256)
	require.NoError(t, err)

	claims := UserClaims{UserUUID: "user-generated-secret"}
	token, err := Sign(claims, secret, WithExpiration(time.Hour))
	require.NoError(t, err)

	parsed, err := Verify[UserClaims](token, secret)
	require.NoError(t, err)
	assert.Equal(t, "user-generated-secret", parsed.UserUUID)
}

func TestGenerateSecretNilMethod(t *testing.T) {
	_, err := GenerateSecret(nil)
	require.Error(t, err)
}

// ── 不可用哈希（自定义 HMAC 方法未设置 Hash） ──

func TestSignVerifyWithUnavailableHashSigningMethodDoesNotPanic(t *testing.T) {
	// 自定义 HMAC 签名方法，Hash 字段保持零值（未注册的哈希算法），
	// 模拟调用方通过 WithSigningMethod 传入一个未正确初始化的自定义方法。
	customMethod := &gojwt.SigningMethodHMAC{Name: "HS-custom-unavailable"}
	secret := make([]byte, 64)

	require.NotPanics(t, func() {
		claims := UserClaims{UserUUID: "user-unavailable-hash"}
		_, err := Sign(claims, secret, WithSigningMethod(customMethod))
		require.Error(t, err)
	})

	require.NotPanics(t, func() {
		_, err := Verify[UserClaims]("dummy.token.value", secret, WithSigningMethod(customMethod))
		require.Error(t, err)
	})
}

func TestGenerateSecretWithUnavailableHashReturnsError(t *testing.T) {
	customMethod := &gojwt.SigningMethodHMAC{Name: "HS-custom-unavailable"}

	_, err := GenerateSecret(customMethod)
	require.Error(t, err)

	code, _ := goerror.Extract(err)
	assert.Equal(t, autherror.CodeJWTSignFailed, code)
}
