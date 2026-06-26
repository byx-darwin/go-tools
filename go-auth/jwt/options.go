// Package jwt 提供泛型 JWT 签发、验证和刷新工具。
//
// 基于 golang-jwt/jwt/v5，支持任意 Claims 类型。
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
package jwt

import (
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

// defaultSigningMethod 默认签名算法。
var defaultSigningMethod = gojwt.SigningMethodHS256

// config 存储 JWT 配置选项。
type config struct {
	expiration    time.Duration
	issuer        string
	signingMethod gojwt.SigningMethod
}

// Option 定义配置选项函数。
type Option func(*config)

// WithExpiration 设置 JWT 过期时间。零值忽略。
func WithExpiration(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.expiration = d
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
func WithSigningMethod(method gojwt.SigningMethod) Option {
	return func(c *config) {
		if method != nil {
			c.signingMethod = method
		}
	}
}

// applyOptions 应用选项并返回配置快照。
func applyOptions(opts []Option) config {
	cfg := config{
		signingMethod: defaultSigningMethod,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}
