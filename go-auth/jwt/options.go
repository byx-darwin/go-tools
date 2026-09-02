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
// HMAC 密钥强度要求（RFC 7518）：密钥长度必须不低于对应哈希算法的输出长度，
// 即 HS256 >= 32 字节、HS384 >= 48 字节、HS512 >= 64 字节，否则 Sign/Verify
// 返回 autherror.ErrJWTWeakSecret。使用 GenerateSecret 生成合规密钥：
//
//	secret, err := jwt.GenerateSecret(gojwt.SigningMethodHS256) // 32 字节
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
//	claims, err := jwt.Verify[UserClaims](token, secret, jwt.WithExpectedIssuer("myapp")) // 校验 issuer
//	newToken, err := jwt.Refresh[UserClaims](ctx, token, secret, revocationStore, jwt.WithExpiration(24*time.Hour))
//
//	// 非对称算法示例（RS256）：
//	token, err := jwt.Sign(UserClaims{UserUUID: "123"}, rsaPrivateKey, jwt.WithSigningMethod(gojwt.SigningMethodRS256))
//	claims, err := jwt.Verify[UserClaims](token, rsaPublicKey, jwt.WithSigningMethod(gojwt.SigningMethodRS256))
//
// 关于 Claims 中 RegisteredClaims 的嵌入方式（重要限制）：
// 默认过期时间/Issuer 填充（见 Sign）以及 ExtractJTI/Refresh 的 JTI 轮换
// 逻辑，都依赖通过反射从 Claims 中找到嵌入的 gojwt.RegisteredClaims。
// 该反射查找只支持 RegisteredClaims 被**直接、单层**嵌入到 Claims 顶层
// 结构体（如上面 UserClaims 示例）。以下两种写法都不受支持，会被**静默**
// 忽略（不返回错误、不记录日志），效果等同于 Claims 未携带
// RegisteredClaims：
//   - 使用 gojwt.MapClaims 作为 Claims 类型；
//   - RegisteredClaims 被间接/多层嵌入，例如 Claims 嵌入了另一个中间
//     结构体，RegisteredClaims 又嵌入在该中间结构体内部，而非直接嵌入
//     Claims 本身。
//
// 详见 extractRegisteredClaims/extractEmbeddedRegisteredClaims 的实现注释。
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
// issuer 仅在 Sign 时生效（设置签发的 Issuer）；expectedIssuer 仅在 Verify
// 时生效（校验 Token 的 Issuer 必须等于该值）——两者语义独立，互不影响。
type config struct {
	expiration             *time.Duration
	issuer                 string
	expectedIssuer         string
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
// 仅在 Sign 时生效，用于设置签发 Token 的 Issuer；Verify 不会读取此选项，
// 传给 Verify 时会被静默忽略。若需要在 Verify 时校验 Token 的 issuer，使用
// WithExpectedIssuer。
func WithIssuer(issuer string) Option {
	return func(c *config) {
		if issuer != "" {
			c.issuer = issuer
		}
	}
}

// WithExpectedIssuer 要求 Verify 校验 Token 的 issuer（iss claim）必须等于
// 给定值。空字符串忽略（即不做任何 issuer 校验，请确保配置项非空，否则本
// 选项等同未启用）。
// 仅在 Verify 时生效；Sign 不会读取此选项。Token 的 iss claim 缺失或与给定
// 值不匹配时，Verify 返回 autherror.ErrTokenInvalid。与 WithIssuer 语义独立
// （WithIssuer 只影响 Sign），两者可同时传给 Refresh 而不互相干扰。
func WithExpectedIssuer(issuer string) Option {
	return func(c *config) {
		if issuer != "" {
			c.expectedIssuer = issuer
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
