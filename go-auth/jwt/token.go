package jwt

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
)

// Sign 签发 JWT，支持任意 Claims 类型。
// claims 必须实现 jwt.Claims 接口（通常通过嵌入 jwt.RegisteredClaims）。
// 当 opts 中设置了 WithExpiration 时，自动设置 ExpiresAt。
// 当 opts 中设置了 WithIssuer 时，自动设置 Issuer。
func Sign[T any](claims T, secret []byte, opts ...Option) (string, error) {
	cfg := applyOptions(opts)

	// 使用指针以便修改 RegisteredClaims（设置过期时间、签发者）。
	jwtClaims, ok := any(&claims).(gojwt.Claims)
	if !ok {
		return "", fmt.Errorf("jwt.Sign: claims type %T does not implement jwt.Claims", claims)
	}

	setClaimsDefaults(jwtClaims, cfg)

	token := gojwt.NewWithClaims(cfg.signingMethod, jwtClaims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("jwt.Sign: failed to sign token: %w", err)
	}

	return signed, nil
}

// Verify 验证 JWT，返回指定类型的 Claims 指针。
// 验证失败时返回认证错误（TokenInvalid 或 TokenExpired）。
func Verify[T any](tokenStr string, secret []byte) (*T, error) {
	var zero T

	// 通过 any 进行运行时接口检查。
	claims, ok := any(&zero).(gojwt.Claims)
	if !ok {
		return nil, fmt.Errorf("jwt.Verify: claims type %T does not implement jwt.Claims", zero)
	}

	token, err := gojwt.ParseWithClaims(tokenStr, claims, func(_ *gojwt.Token) (any, error) {
		return secret, nil
	})
	if err != nil {
		return nil, mapJWTError(err)
	}

	// 通过 any 进行运行时类型断言（编译器无法证明 *T 实现 jwt.Claims）。
	if result, ok := any(token.Claims).(*T); ok && token.Valid {
		return result, nil
	}

	return nil, autherror.ErrTokenInvalid.Wrap(fmt.Errorf("jwt.Verify: invalid claims type"))
}

// Refresh 刷新 JWT（延长过期时间，保留原有 Claims 数据）。
// 先验证原 Token 有效性，再使用新选项重新签发。
// 原 Claims 中的 ExpiresAt、Issuer 等会被 opts 中的值覆盖。
func Refresh[T any](tokenStr string, secret []byte, opts ...Option) (string, error) {
	// 先验证原 Token，提取 Claims。
	claims, err := Verify[T](tokenStr, secret)
	if err != nil {
		return "", fmt.Errorf("jwt.Refresh: %w", err)
	}

	// 使用原 Claims 重新签发，应用新选项。
	return Sign(*claims, secret, opts...)
}

// setClaimsDefaults 根据配置设置 Claims 的默认字段。
func setClaimsDefaults(claims gojwt.Claims, cfg config) {
	rc := extractRegisteredClaims(claims)
	if rc == nil {
		return
	}

	if cfg.expiration > 0 {
		rc.ExpiresAt = gojwt.NewNumericDate(time.Now().Add(cfg.expiration))
	}

	if cfg.issuer != "" {
		rc.Issuer = cfg.issuer
	}
}

// extractRegisteredClaims 从 Claims 中提取嵌入的 RegisteredClaims 指针。
// 支持以下类型：
//   - *gojwt.RegisteredClaims（直接返回）
//   - *gojwt.MapClaims（不支持，返回 nil）
//   - 任何嵌入了 RegisteredClaims 的结构体指针（通过反射提取）
func extractRegisteredClaims(claims gojwt.Claims) *gojwt.RegisteredClaims {
	if claims == nil {
		return nil
	}

	//nolint:gocritic // 类型断言链是必要的
	switch c := claims.(type) {
	case *gojwt.RegisteredClaims:
		return c
	case *gojwt.MapClaims:
		return nil
	default:
		return extractEmbeddedRegisteredClaims(claims)
	}
}

// extractEmbeddedRegisteredClaims 通过反射查找嵌入的 RegisteredClaims 字段。
func extractEmbeddedRegisteredClaims(claims gojwt.Claims) *gojwt.RegisteredClaims {
	v := reflect.ValueOf(claims)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	rcType := reflect.TypeFor[gojwt.RegisteredClaims]()
	t := v.Type()

	for i := range t.NumField() {
		if t.Field(i).Type != rcType {
			continue
		}
		field := v.Field(i)
		if !field.CanAddr() {
			return nil
		}
		return field.Addr().Interface().(*gojwt.RegisteredClaims)
	}

	return nil
}

// mapJWTError 将 golang-jwt 的错误映射为认证错误。
func mapJWTError(err error) error {
	if err == nil {
		return nil
	}

	// 检查过期错误。
	if errors.Is(err, gojwt.ErrTokenExpired) {
		return autherror.ErrTokenExpired.Wrap(err)
	}

	// 其他所有验证错误归为 TokenInvalid。
	return autherror.ErrTokenInvalid.Wrap(err)
}
