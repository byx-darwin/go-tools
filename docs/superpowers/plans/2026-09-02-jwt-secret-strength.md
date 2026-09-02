# JWT 密钥强度校验 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `go-auth/jwt` 的 HMAC 签名算法增加密钥长度下限校验，拒绝弱密钥被用于签发/验证 JWT，并提供 `GenerateSecret` helper 方便调用方生成合规密钥。

**Architecture:** 复用 `token.go` 中已有的 `validateKeyType` 校验入口（Sign 与 Verify 均已调用它），在 HMAC 分支类型断言通过后追加长度校验；新增 `GenerateSecret` 函数与新增错误码 `ErrJWTWeakSecret`，不改变现有函数签名。

**Tech Stack:** Go 1.25、`golang-jwt/jwt/v5`、`samber/oops`、`crypto/rand`、`testify`。

**Spec:** `docs/superpowers/specs/2026-09-02-jwt-secret-strength-design.md`

## Global Constraints

- 错误码范围：go-auth 40000-40099（本次新增 `CodeJWTWeakSecret = 40011`）。
- HMAC 密钥最小长度 = 对应哈希算法输出长度：HS256 ≥ 32 字节，HS384 ≥ 48 字节，HS512 ≥ 64 字节（用 `crypto.Hash.Size()` 动态取值，不硬编码算法名到长度的映射表）。
- 非对称算法（RSA/ECDSA/EdDSA）密钥强度校验不在本次范围内。
- 不改变 `Sign`/`Verify`/`Refresh` 现有函数签名；`validateKeyType` 内部实现可扩展但不改变其签名 `func(method gojwt.SigningMethod, key any, forSigning bool) error`。
- 所有新增导出符号（`ErrJWTWeakSecret`、`CodeJWTWeakSecret`、`GenerateSecret`）必须有 `// Name ...` 格式 godoc 注释（revive 规则，见 `.claude/rules/go.md` §8.3）。

---

## ⚠️ 关键前置发现：现有测试 fixture 长度不足

`go-auth/jwt/token_test.go` 中：
- `testSecret = []byte("test-secret-key-for-jwt")`（23 字节）—— 低于 HS256 的 32 字节下限，且测试中还会以 `WithSigningMethod(gojwt.SigningMethodHS512)` 复用该密钥（需要 ≥64 字节）。一旦加入长度校验，**所有使用 testSecret 的现有测试都会因 `ErrJWTWeakSecret` 而失败**，而非测试原本要验证的行为。
- `wrongSecret := []byte("wrong-secret-key")`（17 字节，仅出现在 `TestVerifySignatureMismatch`）—— 同样低于 32 字节下限，会导致该测试提前被 `ErrJWTWeakSecret` 拦截，而不是原本要验证的 `CodeTokenInvalid`（签名不匹配）。

因此 Task 2 必须先修复这两个 fixture 的长度，再添加长度校验逻辑，否则会导致大范围现有测试回归失败。

---

### Task 1: 新增 `CodeJWTWeakSecret` 错误码

**Files:**
- Modify: `go-auth/error/error.go`
- Test: `go-auth/error/error_test.go`

**Interfaces:**
- Produces: `autherror.CodeJWTWeakSecret` (`int` = `40011`)，`autherror.ErrJWTWeakSecret` (`autherror.Builder`，`Public()` = `"jwt_weak_secret"`，注册 HTTP 状态码 500)

- [ ] **Step 1: 在 `error_test.go` 的三个测试表中加入新错误码的用例（此时会编译失败，因为符号尚不存在）**

在 `go-auth/error/error_test.go` 的 `TestCodeConstants` 测试表末尾追加一行：

```go
		{"CodeJWTKeyTypeMismatch", CodeJWTKeyTypeMismatch},
		{"CodeJWTWeakSecret", CodeJWTWeakSecret},
```

在 `TestPredefinedErrors` 测试表末尾追加一行：

```go
		{"ErrJWTKeyTypeMismatch", ErrJWTKeyTypeMismatch, CodeJWTKeyTypeMismatch, "jwt_key_type_mismatch"},
		{"ErrJWTWeakSecret", ErrJWTWeakSecret, CodeJWTWeakSecret, "jwt_weak_secret"},
```

在 `TestHTTPStatusRegistration` 测试表末尾追加一行：

```go
		{"jwt key type mismatch", ErrJWTKeyTypeMismatch.Wrap(errors.New("x")), 500},
		{"jwt weak secret", ErrJWTWeakSecret.Wrap(errors.New("x")), 500},
```

- [ ] **Step 2: 运行测试确认编译失败**

Run: `go test ./go-auth/error/... -run TestCodeConstants -v`
Expected: 编译错误 `undefined: CodeJWTWeakSecret`

- [ ] **Step 3: 在 `error.go` 中新增错误码常量、构造器与 HTTP 状态注册**

将 `error.go` 中的常量块：

```go
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
```

替换为：

```go
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
	CodeJWTWeakSecret      = 40011 // JWT HMAC 密钥强度不足（长度低于哈希算法输出长度）
)
```

将变量块：

```go
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
```

替换为：

```go
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
	ErrJWTWeakSecret      = goerror.Code(CodeJWTWeakSecret).Public("jwt_weak_secret")            // JWT HMAC 密钥强度不足
)
```

将 `init()` 中的 map：

```go
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

替换为：

```go
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
		CodeJWTWeakSecret:      500,
	})
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-auth/error/... -v`
Expected: PASS（全部用例，包括新增的三条）

- [ ] **Step 5: Commit**

```bash
git add go-auth/error/error.go go-auth/error/error_test.go
git commit -m "feat(go-auth): add CodeJWTWeakSecret error code"
```

---

### Task 2: HMAC 密钥长度校验

**Files:**
- Modify: `go-auth/jwt/token.go`
- Test: `go-auth/jwt/token_test.go`

**Interfaces:**
- Consumes: `autherror.ErrJWTWeakSecret`（Task 1 产出）
- Produces: `validateHMACSecretLength(method *gojwt.SigningMethodHMAC, secret []byte) error`（包内私有，供 `validateKeyType` 调用）

- [ ] **Step 1: 修复现有测试 fixture 长度（防止后续步骤引入回归）**

在 `go-auth/jwt/token_test.go` 中，将：

```go
var testSecret = []byte("test-secret-key-for-jwt")
```

替换为（64 字节，同时满足 HS256/384/512 全部下限）：

```go
var testSecret = []byte("test-secret-key-for-jwt-tests-must-be-at-least-64-bytes-long-okk")
```

在 `TestVerifySignatureMismatch` 中，将：

```go
	wrongSecret := []byte("wrong-secret-key")
```

替换为（52 字节，满足 HS256 下限，且与 testSecret 不同以保持"签名不匹配"语义）：

```go
	wrongSecret := []byte("wrong-secret-key-must-also-be-at-least-32-bytes-long")
```

- [ ] **Step 2: 运行完整 jwt 包测试，确认修复 fixture 后仍全部通过（校验逻辑尚未加入，此步骤是回归基线）**

Run: `go test ./go-auth/jwt/... -v`
Expected: PASS（全部现有用例，因为尚未加入长度校验，此步仅验证 fixture 替换未破坏原有行为）

- [ ] **Step 3: 在 `token_test.go` 末尾新增弱密钥拒绝测试（此时会编译失败，因为 `ErrJWTWeakSecret` 校验逻辑尚未接入）**

```go
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
```

- [ ] **Step 4: 运行测试确认失败（弱密钥当前仍被接受）**

Run: `go test ./go-auth/jwt/... -run TestSignRejectsWeakHMACSecret -v`
Expected: FAIL（`require.Error(t, err)` 得到 nil error，因为长度校验尚未实现）

- [ ] **Step 5: 在 `token.go` 中实现长度校验，接入 `validateKeyType`**

将 `validateKeyType` 中的 HMAC 分支：

```go
	switch method.(type) {
	case *gojwt.SigningMethodHMAC:
		_, ok = key.([]byte)
		expected = "[]byte"
```

替换为：

```go
	switch m := method.(type) {
	case *gojwt.SigningMethodHMAC:
		var secret []byte
		secret, ok = key.([]byte)
		expected = "[]byte"
		if ok {
			if err := validateHMACSecretLength(m, secret); err != nil {
				return err
			}
		}
```

并将 `switch method.(type) {` 后续 case 分支中的 `method.(type)` 保持不变（`m` 变量仅在 HMAC 分支使用，其余分支继续用原有的 `method` 变量，Go 允许 type switch 保留原变量名 `method` 同时在需要的分支引入 `m`——因为 `switch m := method.(type)` 会在每个 case 分支内都生成对应类型的 `m`，其余分支不使用 `m` 不会导致编译错误，只需确认其余分支代码不受影响）。

在 `validateKeyType` 函数下方新增：

```go
// validateHMACSecretLength 校验 HMAC 密钥长度不低于对应哈希算法的输出长度
// （RFC 7518 建议：密钥长度 >= 哈希输出长度，如 HS256 要求 >=32 字节）。
// 不满足时返回 autherror.ErrJWTWeakSecret，防止弱密钥被离线暴力破解伪造 Token。
func validateHMACSecretLength(method *gojwt.SigningMethodHMAC, secret []byte) error {
	minLen := method.Hash.Size()
	if len(secret) < minLen {
		return autherror.ErrJWTWeakSecret.Errorf(
			"HMAC signing method %s requires key length >= %d bytes, got %d", method.Alg(), minLen, len(secret))
	}
	return nil
}
```

- [ ] **Step 6: 运行完整 jwt 包测试确认全部通过**

Run: `go test ./go-auth/jwt/... -v`
Expected: PASS（含新增的 5 个用例与全部既有用例）

- [ ] **Step 7: Commit**

```bash
git add go-auth/jwt/token.go go-auth/jwt/token_test.go
git commit -m "feat(go-auth): enforce HMAC secret minimum length in JWT Sign/Verify"
```

---

### Task 3: `GenerateSecret` helper 与 godoc 更新

**Files:**
- Modify: `go-auth/jwt/token.go`
- Modify: `go-auth/jwt/options.go`
- Test: `go-auth/jwt/token_test.go`

**Interfaces:**
- Consumes: `autherror.CodeJWTSignFailed`（已存在，Task 1/2 之前既有）
- Produces: `GenerateSecret(method *gojwt.SigningMethodHMAC) ([]byte, error)`（导出函数）

- [ ] **Step 1: 在 `token_test.go` 末尾新增 `GenerateSecret` 测试（此时会编译失败，因为函数尚不存在）**

```go
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
```

- [ ] **Step 2: 运行测试确认编译失败**

Run: `go test ./go-auth/jwt/... -run TestGenerateSecret -v`
Expected: 编译错误 `undefined: GenerateSecret`

- [ ] **Step 3: 在 `token.go` 中新增 `GenerateSecret` 函数**

在 `import` 块中加入 `"crypto/rand"`：

```go
import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
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
```

在 `validateHMACSecretLength` 函数下方新增：

```go
// GenerateSecret 使用 crypto/rand 生成满足 method 密钥强度要求的 HMAC 共享密钥。
// 生成长度等于 method 对应哈希算法的输出长度（如 HS256 返回 32 字节），可直接
// 用作 Sign/Verify 的 secret 参数。仅支持 HMAC 族签名算法；非对称算法
// （RS256/ES256/EdDSA）请使用各自的密钥生成方式（如 rsa.GenerateKey）。
func GenerateSecret(method *gojwt.SigningMethodHMAC) ([]byte, error) {
	if method == nil {
		return nil, oops.With("jwt.GenerateSecret").
			Code(autherror.CodeJWTSignFailed).
			Errorf("method must not be nil")
	}

	secret := make([]byte, method.Hash.Size())
	if _, err := rand.Read(secret); err != nil {
		return nil, oops.With("jwt.GenerateSecret").
			Code(autherror.CodeJWTSignFailed).
			Wrap(err)
	}

	return secret, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-auth/jwt/... -v`
Expected: PASS（含新增的 3 个 `GenerateSecret` 用例与全部既有用例）

- [ ] **Step 5: 更新 `options.go` 包注释，说明密钥强度要求与 `GenerateSecret` 用法**

将 `options.go` 顶部包注释：

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
```

替换为：

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
// HMAC 密钥强度要求（RFC 7518）：密钥长度必须不低于对应哈希算法的输出长度，
// 即 HS256 >= 32 字节、HS384 >= 48 字节、HS512 >= 64 字节，否则 Sign/Verify
// 返回 autherror.ErrJWTWeakSecret。使用 GenerateSecret 生成合规密钥：
//
//	secret, err := jwt.GenerateSecret(gojwt.SigningMethodHS256) // 32 字节
//
// 用法：
```

- [ ] **Step 6: 运行完整 workspace 校验**

Run: `go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... && go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... && go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1`
Expected: 全部通过，无编译错误

- [ ] **Step 7: Lint 校验**

Run: `golangci-lint run --timeout=5m ./go-auth/...`
Expected: 无新增 lint 问题（重点检查新增导出符号的 godoc 注释格式）

- [ ] **Step 8: Commit**

```bash
git add go-auth/jwt/token.go go-auth/jwt/options.go go-auth/jwt/token_test.go
git commit -m "feat(go-auth): add GenerateSecret helper and update JWT package docs"
```

---

## Self-Review Notes（供实施前复核）

- **Spec 覆盖**：设计文档四项方案（错误码、长度校验、GenerateSecret、godoc）分别对应 Task 1/2/2/3；测试要求（弱密钥拒绝 + GenerateSecret 长度）对应 Task 2 Step 3 与 Task 3 Step 1。
- **Fixture 回归风险**：已在 Task 2 Step 1-2 显式处理 `testSecret`/`wrongSecret` 长度不足的问题，避免引入校验后大范围现有测试失败。
- **类型一致性**：`validateHMACSecretLength(method *gojwt.SigningMethodHMAC, secret []byte) error` 与 `GenerateSecret(method *gojwt.SigningMethodHMAC) ([]byte, error)` 均使用 `*gojwt.SigningMethodHMAC` 具体类型（而非 `gojwt.SigningMethod` 接口），因为二者都需要访问 `.Hash` 字段，且调用方传入的 `gojwt.SigningMethodHS256` 等本身就是 `*gojwt.SigningMethodHMAC` 类型，无需额外类型断言负担调用方。
