# JWT Verify Issuer Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `jwt.Verify` actually enforce issuer validation via a new `WithExpectedIssuer` option, without changing `WithIssuer`'s existing Sign-only behavior or breaking `Refresh`.

**Architecture:** Add a new `Option` (`WithExpectedIssuer`) that sets a new `config.expectedIssuer` field; `Verify` passes `gojwt.WithIssuer(cfg.expectedIssuer)` as a `ParserOption` to `gojwt.ParseWithClaims` when that field is non-empty. `WithIssuer` is untouched functionally — only its godoc gains a clarifying note. The two options are orthogonal, so `Refresh` (which reuses `opts` for both its internal `Verify` and its final `Sign`) needs no code changes.

**Tech Stack:** Go 1.25 workspace, `golang-jwt/jwt/v5` (`gojwt.WithIssuer(iss string) ParserOption`), `samber/oops`, `testify`.

**Spec:** `docs/superpowers/specs/2026-09-02-jwt-verify-issuer-design.md`

## Global Constraints

- `go-auth` may only import `go-common` (unaffected here — no new imports needed beyond what's already imported).
- All exported symbols need godoc comments starting with the symbol name (revive linter).
- All error returns must be handled (errcheck) — no new error-wrap sites are introduced by this change; the existing `mapJWTError` default branch already handles the new failure mode.
- gofmt-clean, 3-group import ordering (stdlib / third-party / local) — no import changes needed in `token.go` (`gojwt` is already imported) or `options.go` (`gojwt` is already imported).
- Use `any` not `interface{}` — n/a, no new interface-typed params here.

---

## Task 1: Add `WithExpectedIssuer` option and wire it into `Verify`

**Files:**
- Modify: `go-auth/jwt/options.go` (config struct, new `WithExpectedIssuer` function, `WithIssuer` godoc)
- Modify: `go-auth/jwt/token.go:61-102` (`Verify` function)
- Test: `go-auth/jwt/token_test.go`

**Interfaces:**
- Consumes: `gojwt.WithIssuer(iss string) gojwt.ParserOption` (golang-jwt v5, already a transitive import via `gojwt "github.com/golang-jwt/jwt/v5"` in `token.go`), `gojwt.ParseWithClaims(tokenString string, claims Claims, keyFunc Keyfunc, options ...ParserOption) (*Token, error)` (already called in `Verify`, currently with zero `ParserOption`s).
- Produces: `func WithExpectedIssuer(issuer string) Option` — new public symbol other code/tests can call directly, e.g. `jwt.Verify[T](token, secret, jwt.WithExpectedIssuer("myapp"))`.

- [ ] **Step 1: Write the failing tests**

Add the following four test functions to `go-auth/jwt/token_test.go`. Insert them right after `TestWithIssuerEmptyIgnored` (currently ends at line 381, right before `func TestWithSigningMethodNilIgnored` at line 383):

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./go-auth/jwt/... -run 'TestVerifyWithExpectedIssuer|TestVerifyWithoutExpectedIssuer' -v`
Expected: compile failure — `undefined: WithExpectedIssuer` (the option doesn't exist yet).

- [ ] **Step 3: Add `expectedIssuer` field and `WithExpectedIssuer` option**

In `go-auth/jwt/options.go`, update the `config` struct (currently lines 49-54):

```go
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
```

Replace the existing `WithIssuer` function (currently lines 70-77) with:

```go
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
// 给定值。空字符串忽略。
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
```

- [ ] **Step 4: Wire `expectedIssuer` into `Verify`**

In `go-auth/jwt/token.go`, the `Verify` function currently builds `token, err := gojwt.ParseWithClaims(tokenStr, claims, func(tok *gojwt.Token) (any, error) { ... })` with no trailing `ParserOption`s (lines 74-86). Change it to compute parser options first and pass them through:

```go
	var parserOpts []gojwt.ParserOption
	if cfg.expectedIssuer != "" {
		parserOpts = append(parserOpts, gojwt.WithIssuer(cfg.expectedIssuer))
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
	}, parserOpts...)
```

This replaces only the `token, err := gojwt.ParseWithClaims(...)` call and the lines immediately before it (the `var keyTypeErr error` declaration moves above the new `parserOpts` block, everything else in `Verify` — the keyfunc body, the error handling after the call, the final type-assertion return — stays exactly as-is).

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./go-auth/jwt/... -run 'TestVerifyWithExpectedIssuer|TestVerifyWithoutExpectedIssuer' -v`
Expected: PASS (4/4).

- [ ] **Step 6: Run the existing Refresh/Sign/Verify suite to confirm no regression**

Run: `go test ./go-auth/jwt/... -count=1 -v`
Expected: ALL PASS, including `TestRefreshWithIssuer` unchanged and still green, `TestSignWithIssuer`/`TestWithIssuerEmptyIgnored` unchanged and still green.

- [ ] **Step 7: Add a package-doc usage example for discoverability**

In `go-auth/jwt/options.go`, the package comment's usage block (lines 13-26) currently ends with the RS256 example. Append one more example line after the existing `claims, err := jwt.Verify[UserClaims](token, secret)` line (line 21) — insert it directly below that line, before the blank line that precedes the `Refresh` example:

```go
//	token, err := jwt.Sign(UserClaims{UserUUID: "123"}, secret, jwt.WithExpiration(time.Hour))
//	claims, err := jwt.Verify[UserClaims](token, secret)
//	claims, err := jwt.Verify[UserClaims](token, secret, jwt.WithExpectedIssuer("myapp")) // 校验 issuer
//	newToken, err := jwt.Refresh[UserClaims](ctx, token, secret, revocationStore, jwt.WithExpiration(24*time.Hour))
```

- [ ] **Step 8: Full module validation**

```bash
gofmt -l go-auth/
go vet ./go-auth/...
golangci-lint run ./go-auth/...
go test ./go-auth/... -count=1
```

Expected: `gofmt -l` no output; `go vet`/`golangci-lint`/`go test` all clean.

- [ ] **Step 9: Commit**

```bash
git add go-auth/jwt/options.go go-auth/jwt/token.go go-auth/jwt/token_test.go
git commit -m "fix(go-auth): enforce issuer validation in jwt.Verify via WithExpectedIssuer"
```

---

## Self-Review Notes (checked while writing this plan)

- **Spec coverage:** The design's three AC items (Verify path enforces issuer check / unit tests for mismatch / godoc clarifying Sign vs Verify behavior) are all covered by Task 1 Steps 3-4 (implementation), Step 1 (tests: match, mismatch, missing-claim, backward-compat regression), and Steps 3+7 (godoc on both `WithIssuer` and `WithExpectedIssuer`, plus a package-doc usage example).
- **Refresh non-regression:** Step 6 explicitly runs the full `go-auth/jwt` suite including the pre-existing `TestRefreshWithIssuer`, which the design identified as the test most likely to break under a naive (single-option) implementation. Since this plan's `WithExpectedIssuer` is a distinct option that `Refresh` never sets internally, that test requires zero changes.
- **Type consistency:** `WithExpectedIssuer(issuer string) Option` matches the existing `Option func(*config)` type and the calling convention used by every other `WithXxx` function in `options.go` (single string param, empty-string-ignored guard, matching the project's Functional Options Pattern rule in `.claude/rules/options-pattern.md` §2.2).
- **No placeholders:** every step above contains literal code to write or literal commands to run.
