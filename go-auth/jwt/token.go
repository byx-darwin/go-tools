package jwt

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"errors"
	"fmt"
	"reflect"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/samber/oops"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
	"github.com/byx-darwin/go-tools/go-auth/revocation"
)

// Sign 签发 JWT，支持任意 Claims 类型。
// claims 必须实现 jwt.Claims 接口（通常通过嵌入 jwt.RegisteredClaims）。
// secret 的具体类型由 WithSigningMethod 指定的算法决定（默认 HS256 需要 []byte，
// 见包注释）；类型不匹配返回 autherror.ErrJWTKeyTypeMismatch。
// 默认过期时间为 2 小时，可通过 WithExpiration 覆盖；
// 若 Claims 已自带 ExpiresAt，则优先保留 Claims 中的显式值。
// 当 opts 中设置了 WithIssuer 时，自动设置 Issuer。
func Sign[T any](claims T, secret any, opts ...Option) (string, error) {
	cfg := applyOptions(opts)

	// 使用指针以便修改 RegisteredClaims（设置过期时间、签发者）。
	jwtClaims, ok := any(&claims).(gojwt.Claims)
	if !ok {
		return "", oops.With("jwt.Sign").
			Code(autherror.CodeJWTSignFailed).
			Errorf("claims type %T does not implement jwt.Claims", claims)
	}

	if err := validateKeyType(cfg.signingMethod, secret, true); err != nil {
		return "", err
	}

	setClaimsDefaults(jwtClaims, cfg)

	token := gojwt.NewWithClaims(cfg.signingMethod, jwtClaims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", oops.With("jwt.Sign").
			Code(autherror.CodeJWTSignFailed).
			Wrap(err)
	}

	return signed, nil
}

// Verify 验证 JWT，返回指定类型的 Claims 指针。
// 验证失败时返回认证错误（TokenInvalid 或 TokenExpired）。
// 支持通过 opts 指定期望的签名算法（默认 HS256），防止算法混淆攻击。
// 使用 WithSigningMethod 覆盖默认算法（如 RS256、ES256）；secret 的具体类型
// 由该算法决定（见包注释），类型不匹配返回 autherror.ErrJWTKeyTypeMismatch。
func Verify[T any](tokenStr string, secret any, opts ...Option) (*T, error) {
	var zero T
	cfg := applyOptions(opts)

	// 通过 any 进行运行时接口检查。
	claims, ok := any(&zero).(gojwt.Claims)
	if !ok {
		return nil, oops.With("jwt.Verify").
			Code(autherror.CodeJWTVerifyFailed).
			Errorf("claims type %T does not implement jwt.Claims", zero)
	}

	var keyTypeErr error
	token, err := gojwt.ParseWithClaims(tokenStr, claims, func(tok *gojwt.Token) (any, error) {
		// 验证签名算法，防止算法混淆攻击（如 RS256→HS256）。
		// 必须先于密钥类型校验执行：算法不匹配是安全防御，优先级高于
		// 调用方的密钥类型配置错误（保持 CodeTokenInvalid 语义不变）。
		if tok.Method != cfg.signingMethod {
			return nil, fmt.Errorf("unexpected signing method: got %v, want %v", tok.Header["alg"], cfg.signingMethod.Alg())
		}
		if err := validateKeyType(cfg.signingMethod, secret, false); err != nil {
			keyTypeErr = err
			return nil, err
		}
		return secret, nil
	})
	if err != nil {
		if keyTypeErr != nil {
			return nil, keyTypeErr
		}
		return nil, mapJWTError(err)
	}

	// 通过 any 进行运行时类型断言（编译器无法证明 *T 实现 jwt.Claims）。
	if result, ok := any(token.Claims).(*T); ok && token.Valid {
		return result, nil
	}

	return nil, oops.With("jwt.Verify").
		Code(autherror.CodeJWTVerifyFailed).
		Errorf("invalid claims type")
}

// validateKeyType 校验密钥类型是否匹配签名算法的族。
// forSigning 为 true 时按签名场景校验（HMAC 共享密钥或非对称私钥），
// 为 false 时按验证场景校验（HMAC 共享密钥或非对称公钥）。
// 密钥类型不匹配时返回 autherror.ErrJWTKeyTypeMismatch，而非交由
// golang-jwt 在签名/验证阶段做运行时类型断言 panic 或返回底层错误。
func validateKeyType(method gojwt.SigningMethod, key any, forSigning bool) error {
	var ok bool
	var expected string

	switch method.(type) {
	case *gojwt.SigningMethodHMAC:
		_, ok = key.([]byte)
		expected = "[]byte"
	case *gojwt.SigningMethodRSAPSS:
		if forSigning {
			_, ok = key.(*rsa.PrivateKey)
			expected = "*rsa.PrivateKey"
		} else {
			_, ok = key.(*rsa.PublicKey)
			expected = "*rsa.PublicKey"
		}
	case *gojwt.SigningMethodRSA:
		if forSigning {
			_, ok = key.(*rsa.PrivateKey)
			expected = "*rsa.PrivateKey"
		} else {
			_, ok = key.(*rsa.PublicKey)
			expected = "*rsa.PublicKey"
		}
	case *gojwt.SigningMethodECDSA:
		if forSigning {
			_, ok = key.(*ecdsa.PrivateKey)
			expected = "*ecdsa.PrivateKey"
		} else {
			_, ok = key.(*ecdsa.PublicKey)
			expected = "*ecdsa.PublicKey"
		}
	case *gojwt.SigningMethodEd25519:
		if forSigning {
			_, ok = key.(ed25519.PrivateKey)
			expected = "ed25519.PrivateKey"
		} else {
			_, ok = key.(ed25519.PublicKey)
			expected = "ed25519.PublicKey"
		}
	default:
		// 未识别的签名算法族（如用户自定义 SigningMethod），跳过前置校验，
		// 交由 golang-jwt 自身的签名/验证逻辑处理。
		return nil
	}

	if !ok {
		return autherror.ErrJWTKeyTypeMismatch.Errorf(
			"signing method %s expects key type %s, got %T", method.Alg(), expected, key)
	}

	return nil
}

// Refresh 刷新 JWT（延长过期时间，保留原有 Claims 数据），并对 refresh token 做
// 一次性轮换与复用检测：
//   - 若 Claims 携带 JTI（jti），成功刷新后旧 JTI 会通过 store.Revoke 标记为已
//     撤销，新 Token 携带全新生成的 JTI；同一旧 JTI 被再次用于 Refresh 时视为
//     复用攻击，返回 autherror.ErrTokenRevoked，且不签发新 Token。
//   - 若 Claims 未携带 JTI（ExtractJTI 返回 false），跳过轮换与复用检测，行为
//     与未启用该机制前完全一致（向后兼容）。
//   - store 的 IsRevoked/Revoke 调用失败时按 fail-closed 处理：直接返回错误，
//     不签发新 Token，避免存储故障导致复用检测被绕过。
//   - 检测到复用后是否触发全设备登出由调用方决定（例如收到 ErrTokenRevoked 后
//     调用 device.Store.RemoveAllDevices），本函数不感知 device 包。
//
// 当 Claims 携带 JTI 时，返回的 Token 不仅是新的 access 数据，其本身就是新的
// refresh token（携带全新 JTI）：调用方必须用它替换旧 token 并持久化保存，
// 丢弃旧 token —— 旧 JTI 此时已被 Revoke 标记为撤销，再次使用会被判定为复用。
//
// IsRevoked 与 Revoke 这两步撤销检查并非原子操作：并发对同一旧 token 发起的
// 两次 Refresh（例如合法用户与攻击者的竞态）可能都在对方完成 Revoke 之前通过
// IsRevoked 检查，导致两者都成功刷新并各自拿到独立的新 JTI，复用检测无法捕获
// 这种窄时间窗口内的竞态。这是当前两步式 revocation.Store 接口（缺少
// compare-and-set 原语）的已知局限，非本函数实现所能单独解决。
//
// secret 的类型要求与 Sign/Verify 一致，由当前签名算法决定。
// 原 Claims 中的 ExpiresAt、Issuer 等会被 opts 中的值覆盖；
// 未显式指定 WithExpiration 时，使用默认 2 小时过期。
// Refresh 仅支持对称（HMAC）签名算法，secret 同时作为验证密钥和签名密钥；
// 非对称算法（RS256/ES256/EdDSA）不受本函数支持。
func Refresh[T any](ctx context.Context, tokenStr string, secret any, store revocation.Store, opts ...Option) (string, error) {
	// 先验证原 Token，提取 Claims。opts 透传给 Verify 以复用签名算法校验。
	claims, err := Verify[T](tokenStr, secret, opts...)
	if err != nil {
		return "", oops.With("jwt.Refresh").
			Code(autherror.CodeJWTRefreshFailed).
			Wrap(err)
	}

	if jti, ok := ExtractJTI(claims); ok {
		if store == nil {
			return "", oops.With("jwt.Refresh").
				Code(autherror.CodeJWTRefreshFailed).
				Errorf("revocation store is required for tokens carrying jti")
		}

		revoked, err := store.IsRevoked(ctx, jti)
		if err != nil {
			return "", oops.With("jwt.Refresh").
				Code(autherror.CodeJWTRefreshFailed).
				Wrap(err)
		}
		if revoked {
			return "", autherror.ErrTokenRevoked.Errorf("jti %s already used for refresh (reuse detected)", jti)
		}

		rc := extractRegisteredClaims(any(claims).(gojwt.Claims))
		if rc.ExpiresAt == nil {
			// exp 并非 JWT 规范强制字段；没有 exp 就无法计算撤销记录的 ttl，
			// fail-closed 拒绝刷新，而不是对 rc.ExpiresAt.Time 做空指针解引用。
			return "", oops.With("jwt.Refresh").
				Code(autherror.CodeJWTRefreshFailed).
				Errorf("jti %s has no exp claim, cannot compute revocation ttl", jti)
		}
		if err := store.Revoke(ctx, jti, time.Until(rc.ExpiresAt.Time)); err != nil {
			return "", oops.With("jwt.Refresh").
				Code(autherror.CodeJWTRefreshFailed).
				Wrap(err)
		}

		rc.ID = uuid.NewString()
	}

	// Refresh 语义：新 Token 不复用旧 Token 的剩余有效期，强制刷新 ExpiresAt。
	signOpts := append([]Option{withIgnoreClaimsExpiration()}, opts...)
	return Sign(*claims, secret, signOpts...)
}

// setClaimsDefaults 根据配置设置 Claims 的默认字段。
//   - Sign 路径（ignoreClaimsExpiration == false）：
//     若未显式指定 expiration，仅在 Claims 未自带 ExpiresAt 时以默认 2h 填充，
//     保留调用方在 Claims 中显式设置的过期时间。
//   - Refresh 路径（ignoreClaimsExpiration == true）：
//     忽略 Claims 自带的 ExpiresAt，总是以默认 2h 或显式 WithExpiration 重新设定，
//     保证新 Token 不复用旧 Token 的剩余有效期。
func setClaimsDefaults(claims gojwt.Claims, cfg config) {
	rc := extractRegisteredClaims(claims)
	if rc == nil {
		return
	}

	switch {
	case cfg.expiration != nil:
		// 显式 WithExpiration 覆盖一切。
		rc.ExpiresAt = gojwt.NewNumericDate(time.Now().Add(*cfg.expiration))
	case cfg.ignoreClaimsExpiration:
		// Refresh 路径：强制刷新为默认 2h。
		rc.ExpiresAt = gojwt.NewNumericDate(time.Now().Add(defaultExpiration))
	case rc.ExpiresAt == nil:
		// Sign 默认路径：Claims 未自带 ExpiresAt 时填充默认。
		rc.ExpiresAt = gojwt.NewNumericDate(time.Now().Add(defaultExpiration))
	}

	if cfg.issuer != "" {
		rc.Issuer = cfg.issuer
	}
}

// ExtractJTI 从已验证的 Claims 中提取 JWT ID（jti）。
// claims 通常是 Verify 返回的 *T 指针（或任何实现 jwt.Claims 且嵌入了
// gojwt.RegisteredClaims 的结构体指针）。未找到 RegisteredClaims 或
// ID 为空时返回 ("", false)。
func ExtractJTI(claims any) (string, bool) {
	jwtClaims, ok := claims.(gojwt.Claims)
	if !ok {
		return "", false
	}

	rc := extractRegisteredClaims(jwtClaims)
	if rc == nil || rc.ID == "" {
		return "", false
	}

	return rc.ID, true
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
	if v.Kind() == reflect.Pointer {
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
