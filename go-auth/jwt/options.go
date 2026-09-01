// Package jwt 提供泛型 JWT 签发、验证和刷新工具。
//
// 基于 golang-jwt/jwt/v5，支持任意 Claims 类型，密钥参数为 any，
// 具体类型由签名算法决定：
//   - HMAC 族（HS256/384/512，默认）：secret 为 []byte 共享密钥
//   - RSA 族（RS256/384/512、PS256/384/512）：签名用 *rsa.PrivateKey，验证用 *rsa.PublicKey
//   - ECDSA 族（ES256/384/512）：签名用 *ecdsa.PrivateKey，验证用 *ecdsa.PublicKey
//   - EdDSA：签名用 ed25519.PrivateKey，验证用 ed25519.PublicKey
//
// 密钥类型与签名算法不匹配时，Sign/Verify 返回 autherror.ErrJWTKeyTypeMismatch，
// 而非在底层库中触发运行时类型断言错误。
//
// 用法：
//
//	type UserClaims struct {
//	    UserUUID string `json:"user_uuid"`
//	    jwt.RegisteredClaims
//	}
//
//	token, err := jwt.Sign(UserClaims{UserUUID: "123"}, secret, jwt.WithExpiration(time.Hour))
//	claims, err := jwt.Verify[UserClaims](token, secret)
//	newToken, err := jwt.Refresh[UserClaims](token, secret, jwt.WithExpiration(24*time.Hour))
//
//	// 非对称算法示例（RS256）：
//	token, err := jwt.Sign(UserClaims{UserUUID: "123"}, rsaPrivateKey, jwt.WithSigningMethod(gojwt.SigningMethodRS256))
//	claims, err := jwt.Verify[UserClaims](token, rsaPublicKey, jwt.WithSigningMethod(gojwt.SigningMethodRS256))
package jwt

import (
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

// defaultSigningMethod 默认签名算法。
var defaultSigningMethod = gojwt.SigningMethodHS256

const (
	// defaultExpiration 默认 Token 过期时间（2 小时）。
	defaultExpiration = 2 * time.Hour
)

// config 存储 JWT 配置选项。
// expiration 为指针类型以区分"未显式设置"（nil，使用默认 2h）
// 与"显式覆盖"（非 nil，覆盖 Claims 自带值）。
// ignoreClaimsExpiration 仅在 Refresh 路径为 true，表示忽略 Claims 自带的
// ExpiresAt，强制以默认或显式 expiration 重新设定（Refresh 语义：新 Token
// 使用新的过期时间，不复用旧 Token 的剩余有效期）。
type config struct {
	expiration             *time.Duration
	issuer                 string
	signingMethod          gojwt.SigningMethod
	ignoreClaimsExpiration bool
}

// Option 定义配置选项函数。
type Option func(*config)

// WithExpiration 设置 JWT 过期时间。默认 2 小时。
// 显式传入正值覆盖默认（并覆盖 Claims 自带的 ExpiresAt）；
// 零值或负值被忽略，保留默认。
func WithExpiration(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.expiration = &d
		}
	}
}

// WithIssuer 设置 JWT 签发者。空字符串忽略。
func WithIssuer(issuer string) Option {
	return func(c *config) {
		if issuer != "" {
			c.issuer = issuer
		}
	}
}

// WithSigningMethod 设置 JWT 签名算法。nil 值忽略。
// 默认值为 jwt.SigningMethodHS256。
// 切换到 RS256/ES256/EdDSA 等非对称算法时，Sign/Verify 的 secret 参数
// 必须传入对应的私钥/公钥类型（见包注释），否则返回 autherror.ErrJWTKeyTypeMismatch。
func WithSigningMethod(method gojwt.SigningMethod) Option {
	return func(c *config) {
		if method != nil {
			c.signingMethod = method
		}
	}
}

// withIgnoreClaimsExpiration 内部选项，供 Refresh 使用：
// 强制忽略 Claims 自带的 ExpiresAt，以默认或显式 expiration 重新设定。
func withIgnoreClaimsExpiration() Option {
	return func(c *config) {
		c.ignoreClaimsExpiration = true
	}
}

// applyOptions 应用选项并返回配置快照。
// 默认值：signingMethod = HS256；expiration = nil（由 setClaimsDefaults 按 2h 默认处理）。
func applyOptions(opts []Option) config {
	cfg := config{
		signingMethod: defaultSigningMethod,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}
