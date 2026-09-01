# JWT 密钥类型支持非对称算法 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `go-auth/jwt` 的 `Sign`/`Verify`/`Refresh` 密钥参数从 `[]byte` 改为 `any`，并在内部对密钥类型与签名算法族做前置校验，使 RS256/ES256 等非对称算法真正可用，同时保持算法混淆防御测试的错误码语义不变。

**Architecture:** 新增内部函数 `validateKeyType`，对 golang-jwt/jwt/v5 已支持的五个算法族（HMAC/RSA/RSAPSS/ECDSA/Ed25519）做类型开关校验。`Sign` 在签名前直接调用；`Verify` 把校验放进 `keyfunc` 内部、且在现有 `tok.Method != cfg.signingMethod` 比较之后才执行，确保算法混淆攻击继续优先命中现有防御分支。`Refresh` 通过复用 `Sign`/`Verify` 自动获得校验能力，无需改动。

**Tech Stack:** Go 1.25, `github.com/golang-jwt/jwt/v5` v5.3.1, `github.com/samber/oops`, `github.com/stretchr/testify`

**Spec:** `docs/superpowers/specs/2026-09-01-jwt-key-type-any-design.md`

## Global Constraints

- 密钥参数类型变更（`[]byte` → `any`）对现有调用方非破坏性，不修改任何其他文件中的调用点。
- 密钥类型校验绝不能改变 `TestVerifyAlgorithmConfusion` 断言的 `autherror.CodeTokenInvalid`。
- 新增错误码 `CodeJWTKeyTypeMismatch` 必须落在 go-auth 认证错误码范围 `40000-40099` 内。
- 所有导出符号必须有 `// Name ...` 格式的 godoc 注释（`.claude/rules/go.md` § 8.3）。
- 代码须通过 `gofmt`；错误必须处理（不静默吞错）。

---

### Task 1: 新增 CodeJWTKeyTypeMismatch 错误码

**Files:**
- Modify: `go-auth/error/error.go`
- Test: `go-auth/error/error_test.go`

**Interfaces:**
- Produces: `autherror.CodeJWTKeyTypeMismatch` (`int`，值 `40010`)，`autherror.ErrJWTKeyTypeMismatch` (`goerror.Builder`，用于 `.Errorf(...)` 构造最终 `error`)

- [ ] **Step 1: 写失败测试 —— 错误码范围断言**

在 `go-auth/error/error_test.go` 的 `TestCodeConstants` 测试表中追加一行：

```go
		{"CodeJWTSignFailed", CodeJWTSignFailed},
		{"CodeJWTVerifyFailed", CodeJWTVerifyFailed},
		{"CodeJWTRefreshFailed", CodeJWTRefreshFailed},
		{"CodeJWTKeyTypeMismatch", CodeJWTKeyTypeMismatch},
	}
```

在 `TestPredefinedErrors` 测试表中追加一行：

```go
		{"ErrJWTRefreshFailed", ErrJWTRefreshFailed, CodeJWTRefreshFailed, "jwt_refresh_failed"},
		{"ErrJWTKeyTypeMismatch", ErrJWTKeyTypeMismatch, CodeJWTKeyTypeMismatch, "jwt_key_type_mismatch"},
	}
```

在 `TestHTTPStatusRegistration` 测试表中追加一行：

```go
		{"jwt refresh failed", ErrJWTRefreshFailed.Wrap(errors.New("x")), 500},
		{"jwt key type mismatch", ErrJWTKeyTypeMismatch.Wrap(errors.New("x")), 500},
	}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd go-auth && go test ./error/... -run 'TestCodeConstants|TestPredefinedErrors|TestHTTPStatusRegistration' -v`
Expected: FAIL，编译错误 `undefined: CodeJWTKeyTypeMismatch` / `undefined: ErrJWTKeyTypeMismatch`

- [ ] **Step 3: 实现**

编辑 `go-auth/error/error.go`：

```go
// 认证错误码 40000-40099。
const (
	CodeTokenInvalid       = 40001 // Token 无效
	CodeTokenExpired       = 40002 // Token 已过期
	CodeTokenRevoked       = 40003 // Token 已撤销
	CodeDeviceKicked       = 40004 // 设备已被踢出
	CodeSessionInvalid     = 40005 // Session 无效
	CodeSessionExpired     = 40006 // Session 已过期
	CodeJWTSignFailed      = 40007 // JWT 签名失败
	CodeJWTVerifyFailed    = 40008 // JWT 验证失败
	CodeJWTRefreshFailed   = 40009 // JWT 刷新失败
	CodeJWTKeyTypeMismatch = 40010 // JWT 密钥类型与签名算法不匹配
)

// 预定义认证错误构造器。
var (
	ErrTokenInvalid       = goerror.Code(CodeTokenInvalid).Public("token_invalid")               // Token 无效
	ErrTokenExpired       = goerror.Code(CodeTokenExpired).Public("token_expired")               // Token 已过期
	ErrTokenRevoked       = goerror.Code(CodeTokenRevoked).Public("token_revoked")               // Token 已撤销
	ErrDeviceKicked       = goerror.Code(CodeDeviceKicked).Public("device_kicked")               // 设备已被踢出
	ErrSessionInvalid     = goerror.Code(CodeSessionInvalid).Public("session_invalid")           // Session 无效
	ErrSessionExpired     = goerror.Code(CodeSessionExpired).Public("session_expired")           // Session 已过期
	ErrJWTSignFailed      = goerror.Code(CodeJWTSignFailed).Public("jwt_sign_failed")            // JWT 签名失败
	ErrJWTVerifyFailed    = goerror.Code(CodeJWTVerifyFailed).Public("jwt_verify_failed")        // JWT 验证失败
	ErrJWTRefreshFailed   = goerror.Code(CodeJWTRefreshFailed).Public("jwt_refresh_failed")      // JWT 刷新失败
	ErrJWTKeyTypeMismatch = goerror.Code(CodeJWTKeyTypeMismatch).Public("jwt_key_type_mismatch") // JWT 密钥类型与签名算法不匹配
)

func init() {
	goerror.RegisterHTTPStatuses(map[int]int{
		CodeTokenInvalid:       401,
		CodeTokenExpired:       401,
		CodeTokenRevoked:       401,
		CodeDeviceKicked:       403,
		CodeSessionInvalid:     401,
		CodeSessionExpired:     401,
		CodeJWTSignFailed:      500,
		CodeJWTVerifyFailed:    500,
		CodeJWTRefreshFailed:   500,
		CodeJWTKeyTypeMismatch: 500,
	})
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd go-auth && go test ./error/... -v`
Expected: PASS

- [ ] **Step 5: gofmt + 提交**

```bash
gofmt -l go-auth/error/error.go go-auth/error/error_test.go
git add go-auth/error/error.go go-auth/error/error_test.go
git commit -m "feat(go-auth): add CodeJWTKeyTypeMismatch error code"
```

---

### Task 2: validateKeyType 内部函数 + Sign/Verify/Refresh 密钥参数改为 any

**Files:**
- Modify: `go-auth/jwt/token.go`
- Test: `go-auth/jwt/token_test.go`

**Interfaces:**
- Consumes: `autherror.CodeJWTKeyTypeMismatch` / `autherror.ErrJWTKeyTypeMismatch`（Task 1 产出）
- Produces:
  - `func Sign[T any](claims T, secret any, opts ...Option) (string, error)`
  - `func Verify[T any](tokenStr string, secret any, opts ...Option) (*T, error)`
  - `func Refresh[T any](tokenStr string, secret any, opts ...Option) (string, error)`
  - `func validateKeyType(method gojwt.SigningMethod, key any, forSigning bool) error`（包内私有，供 Sign/Verify 使用）

#### Step 1: 写失败测试 —— RS256 真实签发/验证成功往返

在 `go-auth/jwt/token_test.go` 顶部 import 块中新增：

```go
import (
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
```

在文件末尾追加测试辅助函数与新测试组：

```go
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
```

#### Step 2: 运行测试确认失败

Run: `cd go-auth && go test ./jwt/... -run 'TestSignAndVerifyRS256|TestSignAndVerifyES256|TestSignKeyTypeMismatch|TestVerifyKeyTypeMismatch' -v`
Expected: 编译失败（`Sign`/`Verify` 仍要求 `secret []byte`，`*rsa.PrivateKey`/`*rsa.PublicKey` 无法隐式转换）

#### Step 3: 实现 —— 修改 token.go

编辑 `go-auth/jwt/token.go`，在 import 块中新增（`reflect` 已存在，无需改动）：

```go
import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"errors"
	"fmt"
	"reflect"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/samber/oops"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
)
```

在 `Sign` 函数内、`token.SignedString(secret)` 之前插入校验：

```go
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
```

修改 `Verify` 函数——把密钥类型校验放进 `keyfunc`、且在方法比较**之后**：

```go
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
```

修改 `Refresh` 的签名（函数体不变，仅参数类型）：

```go
func Refresh[T any](tokenStr string, secret any, opts ...Option) (string, error) {
```

在文件中新增 `validateKeyType` 函数（放在 `mapJWTError` 之前或之后均可，建议紧跟 `Verify` 之后）：

```go
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
```

**关于 `keyTypeErr` 变量的说明：** `gojwt.ParseWithClaims` 的 `keyfunc` 返回的 `error` 会被 golang-jwt 内部包装成它自己的 `*ValidationError`，`Verify` 原有的 `mapJWTError` 只会把它归类为通用 `CodeTokenInvalid`。为了让密钥类型不匹配返回精确的 `CodeJWTKeyTypeMismatch`（而不是被 `mapJWTError` 吞成 `CodeTokenInvalid`），用外层闭包变量 `keyTypeErr` 捕获 `validateKeyType` 的原始错误，在 `ParseWithClaims` 返回错误后优先检查并返回它。

#### Step 4: 运行测试确认通过

Run: `cd go-auth && go test ./jwt/... -v`
Expected: 全部 PASS，包括 Task 2 新增的 4 个测试与已有的全部用例（尤其 `TestVerifyAlgorithmConfusion` 断言的 `CodeTokenInvalid` 必须不变）

#### Step 5: gofmt + go vet + 提交

```bash
gofmt -l go-auth/jwt/token.go go-auth/jwt/token_test.go
cd go-auth && go vet ./jwt/...
git add go-auth/jwt/token.go go-auth/jwt/token_test.go
git commit -m "feat(go-auth): support asymmetric key types (RS256/ES256/EdDSA) in JWT Sign/Verify"
```

---

### Task 3: 更新 godoc 说明密钥类型要求

**Files:**
- Modify: `go-auth/jwt/options.go`
- Modify: `go-auth/jwt/token.go`（仅注释，Task 2 已完成签名变更）

**Interfaces:**
- Consumes: 无新接口，仅补充注释
- Produces: 无新接口

- [ ] **Step 1: 更新 options.go 包注释与 WithSigningMethod godoc**

编辑 `go-auth/jwt/options.go` 顶部包注释：

```go
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
```

编辑 `WithSigningMethod` 的 godoc：

```go
// WithSigningMethod 设置 JWT 签名算法。nil 值忽略。
// 默认值为 jwt.SigningMethodHS256。
// 切换到 RS256/ES256/EdDSA 等非对称算法时，Sign/Verify 的 secret 参数
// 必须传入对应的私钥/公钥类型（见包注释），否则返回 autherror.ErrJWTKeyTypeMismatch。
func WithSigningMethod(method gojwt.SigningMethod) Option {
```

- [ ] **Step 2: 更新 token.go 中 Sign/Verify/Refresh 的 secret 参数说明**

`Sign` 函数注释追加一行：

```go
// Sign 签发 JWT，支持任意 Claims 类型。
// claims 必须实现 jwt.Claims 接口（通常通过嵌入 jwt.RegisteredClaims）。
// secret 的具体类型由 WithSigningMethod 指定的算法决定（默认 HS256 需要 []byte，
// 见包注释）；类型不匹配返回 autherror.ErrJWTKeyTypeMismatch。
// 默认过期时间为 2 小时，可通过 WithExpiration 覆盖；
// 若 Claims 已自带 ExpiresAt，则优先保留 Claims 中的显式值。
// 当 opts 中设置了 WithIssuer 时，自动设置 Issuer。
func Sign[T any](claims T, secret any, opts ...Option) (string, error) {
```

`Verify` 函数注释追加一行：

```go
// Verify 验证 JWT，返回指定类型的 Claims 指针。
// 验证失败时返回认证错误（TokenInvalid 或 TokenExpired）。
// 支持通过 opts 指定期望的签名算法（默认 HS256），防止算法混淆攻击。
// 使用 WithSigningMethod 覆盖默认算法（如 RS256、ES256）；secret 的具体类型
// 由该算法决定（见包注释），类型不匹配返回 autherror.ErrJWTKeyTypeMismatch。
func Verify[T any](tokenStr string, secret any, opts ...Option) (*T, error) {
```

`Refresh` 函数注释追加一行：

```go
// Refresh 刷新 JWT（延长过期时间，保留原有 Claims 数据）。
// 先验证原 Token 有效性，再使用新选项重新签发。
// secret 的类型要求与 Sign/Verify 一致，由当前签名算法决定。
// 原 Claims 中的 ExpiresAt、Issuer 等会被 opts 中的值覆盖；
// 未显式指定 WithExpiration 时，使用默认 2 小时过期。
func Refresh[T any](tokenStr string, secret any, opts ...Option) (string, error) {
```

- [ ] **Step 3: 运行完整测试确认无回归**

Run: `cd go-auth && go test ./... -count=1 -v`
Expected: 全部 PASS

- [ ] **Step 4: gofmt + 提交**

```bash
gofmt -l go-auth/jwt/options.go go-auth/jwt/token.go
git add go-auth/jwt/options.go go-auth/jwt/token.go
git commit -m "docs(go-auth): document asymmetric key type requirements in JWT package"
```

---

### Task 4: 全量验证

**Files:** 无新文件改动，仅验证

**Interfaces:** 无

- [ ] **Step 1: 全量构建**

Run: `go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`
Expected: 无错误（确认 `go-framework/hertz/middleware/jwt_auth.go` 等消费方在 `secret any` 签名变更后仍编译通过）

- [ ] **Step 2: go vet**

Run: `go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`
Expected: 无错误

- [ ] **Step 3: golangci-lint（逐 module）**

Run:
```bash
for m in go-common go-auth go-middleware go-framework; do
  golangci-lint run --timeout=5m ./$m/...
done
```
Expected: 无错误

- [ ] **Step 4: 全量测试**

Run: `go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1`
Expected: 全部 PASS

- [ ] **Step 5: gofmt 全量检查**

Run: `gofmt -l $(find . -name '*.go' -not -path '*/vendor/*' -not -path './.git/*' -not -path './.worktree/*')`
Expected: 空输出（无未格式化文件）

## Self-Review Notes

- **Spec 覆盖**：设计文档 5 项要点（错误码、参数类型、validateKeyType、调用时机约束、文档更新、测试）均对应 Task 1-3 中的具体步骤。
- **无占位符**：所有代码块均为可直接使用的完整实现。
- **类型一致性**：`validateKeyType(method gojwt.SigningMethod, key any, forSigning bool) error` 签名在 Task 2 定义后于 Task 2 内被 `Sign`/`Verify` 一致调用；Task 3 仅涉及注释，不引入新签名。
