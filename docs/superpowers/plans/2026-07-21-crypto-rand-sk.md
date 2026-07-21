# go-common/auth crypto/rand SK & AK Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the zero-entropy `MD5(ak+timestamp)` SK derivation with `crypto/rand`, and harden `GetRandAk` to use unbiased `crypto/rand` selection with a normalized charset.

**Architecture:** Rewrite `go-common/auth/ak.go`: `RefreshSK()` becomes a no-arg function returning 32 `crypto/rand` bytes hex-encoded (64 chars), panicking on rand failure; `GetRandAk(length)` uses `crypto/rand.Int` rejection sampling over a normalized 62-char alphanumeric charset (fixing duplicate `O`, missing `0`, and the `Intn(61)` off-by-one that made `9` unreachable). Update the test file and the single example caller.

**Tech Stack:** Go 1.26.5, stdlib only (`crypto/rand`, `encoding/hex`, `math/big`), stdlib `testing`, golangci-lint v2.

## Global Constraints

- **go-common has zero framework dependency** — only stdlib imports (`crypto/rand`, `encoding/hex`, `math/big`). Do NOT import Hertz/Kitex or any other go-tools module.
- **All exported symbols need godoc** starting with the symbol name (revive `exported` rule).
- **Error handling:** `crypto/rand` read failure → `panic` (keygen idiom, like `ed25519.GenerateKey`). Do not silently swallow; errcheck must be satisfied.
- **gofmt-clean**; grouped imports stdlib-only for this file.
- **Scope:** only `go-common/auth/ak.go`, `go-common/auth/ak_test.go`, and `example/handler/common_aksk.go`. No other files.
- **Design source of truth:** `docs/superpowers/specs/2026-07-21-crypto-rand-sk-design.md` (decisions D1–D4).

---

## File Structure

| File | Responsibility | Action |
|------|----------------|--------|
| `go-common/auth/ak.go` | AK + SK credential generation | Modify (rewrite both funcs, swap imports, add `akCharset` + `skBytes` consts) |
| `go-common/auth/ak_test.go` | Unit tests for the above | Modify (new SK assertions, charset sanity + coverage, updated integration) |
| `example/handler/common_aksk.go` | Demo route using the auth package | Modify (`RefreshSK(ak)` → `RefreshSK()`) |

**Task ordering rationale:** Task 1 rewrites `GetRandAk` first (removes the `math/rand` import while `time` + `go-common/crypto` are still used by the unchanged `RefreshSK`), then Task 2 rewrites `RefreshSK` (drops the now-unused `time` + `crypto` imports, adds `encoding/hex`). This keeps the file compiling after every task.

---

### Task 1: GetRandAk — crypto/rand + normalized charset

**Files:**
- Modify: `go-common/auth/ak.go` (replace `GetRandAk` body, add `akCharset` const, swap `math/rand`→`crypto/rand`+`math/big`)
- Test: `go-common/auth/ak_test.go`

**Interfaces:**
- Consumes: nothing new (stdlib only)
- Produces: `func GetRandAk(length int) string` (unchanged signature), `const akCharset string` (62 chars: a-z, A-Z, 0-9)

- [ ] **Step 1: Write the failing tests (behavioral, compile against the unchanged signature)**

Replace the existing `TestGetRandAk_*` functions in `go-common/auth/ak_test.go` with:

```go
func TestGetRandAk_Length(t *testing.T) {
	sizes := []int{0, 1, 5, 10, 32, 64}
	for _, size := range sizes {
		if ak := GetRandAk(size); len(ak) != size {
			t.Errorf("GetRandAk(%d) length = %d, want %d", size, len(ak), size)
		}
	}
	if ak := GetRandAk(-5); ak != "" {
		t.Errorf("GetRandAk(-5) = %q, want empty string", ak)
	}
}

func TestGetRandAk_OnlyAlphanumeric(t *testing.T) {
	const validChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for range 20 {
		ak := GetRandAk(100)
		for _, ch := range ak {
			if !strings.ContainsRune(validChars, ch) {
				t.Errorf("GetRandAk contains invalid char: %c", ch)
			}
		}
	}
}

func TestGetRandAk_Uniqueness(t *testing.T) {
	set := make(map[string]bool)
	for range 100 {
		ak := GetRandAk(16)
		if set[ak] {
			t.Errorf("duplicate AK generated (very unlikely): %s", ak)
		}
		set[ak] = true
	}
}

func TestGetRandAk_Coverage(t *testing.T) {
	ak := GetRandAk(10000)
	if !strings.ContainsRune(ak, '0') {
		t.Error("GetRandAk never produced '0' over 10000 chars; charset must include 0")
	}
	if !strings.ContainsRune(ak, '9') {
		t.Error("GetRandAk never produced '9' over 10000 chars; Intn(61) off-by-one must be fixed")
	}
}
```

(Leave the existing `TestRefreshSK_*` and `TestIntegration_AKAndSK` untouched for now — they are rewritten in Task 2. Keep the `strings` import.)

- [ ] **Step 2: Run the test to verify it fails (RED)**

Run: `go test ./go-common/auth/ -run 'TestGetRandAk' -count=1`
Expected: FAIL — `TestGetRandAk_Coverage` fails with both `'0'` and `'9'` missing (current charset omits `0` and `Intn(61)` excludes `9`).

- [ ] **Step 3: Write the minimal implementation**

In `go-common/auth/ak.go`, replace the `GetRandAk` function and the import block. Add the `akCharset` const. The file now reads:

```go
package auth

import (
	"crypto/rand"
	"math/big"
	"time"

	"github.com/byx-darwin/go-tools/go-common/crypto"
)

// akCharset 是 AK 使用的 62 个字母数字字符（a-z、A-Z、0-9）。
const akCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GetRandAk 生成并返回指定长度的随机 AK（Access Key）。
//
// 使用 crypto/rand 从 62 字符字母数字集合（a-z、A-Z、0-9）中无偏选取
// （rand.Int 拒绝采样），修复了历史上 math/rand、字符表重复 'O'、缺失 '0'
// 以及 Intn(61) 导致 '9' 永不出现的问题。length <= 0 时返回空字符串。
// 若读取 crypto/rand 失败（在支持的平台上几乎不可能），将 panic。
func GetRandAk(length int) string {
	if length <= 0 {
		return ""
	}
	ak := make([]byte, length)
	max := big.NewInt(int64(len(akCharset)))
	for i := range ak {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic("auth: read crypto/rand: " + err.Error())
		}
		ak[i] = akCharset[n.Int64()]
	}
	return string(ak)
}

// RefreshSK 刷新SK
func RefreshSK(ak string) string {
	signer := ak + "/" + time.Now().String()
	return crypto.MD5([]byte(signer))
}
```

Note: `math/rand` import is removed; `time` and `go-common/crypto` remain because the unchanged `RefreshSK` still uses them.

- [ ] **Step 4: Run the tests to verify they pass (GREEN)**

Run: `go test ./go-common/auth/ -run 'TestGetRandAk' -count=1`
Expected: PASS (all four `TestGetRandAk_*`).

- [ ] **Step 5: Add a white-box charset sanity test (now that `akCharset` exists)**

Append to `go-common/auth/ak_test.go`:

```go
func TestAkCharset_Sanity(t *testing.T) {
	seen := make(map[rune]bool)
	for _, ch := range akCharset {
		if seen[ch] {
			t.Errorf("akCharset contains duplicate rune: %c", ch)
		}
		seen[ch] = true
	}
	if len(seen) != 62 {
		t.Errorf("akCharset unique runes = %d, want 62", len(seen))
	}
	for _, must := range []rune{'0', '9', 'O', 'a', 'Z'} {
		if !seen[must] {
			t.Errorf("akCharset missing required rune: %c", must)
		}
	}
}
```

Run: `go test ./go-common/auth/ -run 'TestAkCharset_Sanity' -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go-common/auth/ak.go go-common/auth/ak_test.go
git commit -m "fix(go-common): generate AK with crypto/rand and normalize charset

Replace math/rand with crypto/rand (unbiased rejection sampling) and
normalize the charset to 62 alphanumerics (a-z, A-Z, 0-9), fixing the
duplicate 'O', missing '0', and Intn(61) off-by-one that made '9'
unreachable.

Refs #24"
```

---

### Task 2: RefreshSK — crypto/rand, no-arg, hex 64, panic

**Files:**
- Modify: `go-common/auth/ak.go` (replace `RefreshSK`, add `skBytes` const, drop `time`+`crypto` imports, add `encoding/hex`)
- Test: `go-common/auth/ak_test.go`

**Interfaces:**
- Consumes: `crypto/rand`, `encoding/hex` (stdlib)
- Produces: `func RefreshSK() string` (**breaking**: drops the `ak` param), `const skBytes int` (=32)

- [ ] **Step 1: Write the failing tests (new no-arg signature)**

In `go-common/auth/ak_test.go`, delete the three old `RefreshSK` tests (`TestRefreshSK_ProducesValue`, `TestRefreshSK_SameInputProducesConsistentFormat`, `TestRefreshSK_DifferentAKProducesDifferentSK`) and replace with:

```go
func TestRefreshSK_Length(t *testing.T) {
	if sk := RefreshSK(); len(sk) != 64 {
		t.Errorf("RefreshSK length = %d, want 64 (32-byte hex)", len(sk))
	}
}

func TestRefreshSK_HexCharset(t *testing.T) {
	sk := RefreshSK()
	for _, ch := range sk {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			t.Errorf("RefreshSK contains non-hex char: %c", ch)
		}
	}
}

func TestRefreshSK_Uniqueness(t *testing.T) {
	set := make(map[string]bool)
	for range 100 {
		sk := RefreshSK()
		if set[sk] {
			t.Errorf("duplicate SK generated (astronomically unlikely): %s", sk)
		}
		set[sk] = true
	}
}
```

And update the integration test to the no-arg call and 64-char SK:

```go
func TestIntegration_AKAndSK(t *testing.T) {
	ak := GetRandAk(32)
	if len(ak) != 32 {
		t.Fatalf("AK length = %d, want 32", len(ak))
	}
	sk := RefreshSK()
	if len(sk) != 64 {
		t.Fatalf("SK length = %d, want 64 (32-byte hex)", len(sk))
	}
}
```

- [ ] **Step 2: Run the test to verify it fails (RED)**

Run: `go test ./go-common/auth/ -run 'TestRefreshSK' -count=1`
Expected: FAIL — build error: `too few arguments in call to RefreshSK` (old signature still takes `ak`). This compile failure is the RED state for a signature change.

- [ ] **Step 3: Write the minimal implementation**

In `go-common/auth/ak.go`, replace the `RefreshSK` function and the import block. The file now reads (final form):

```go
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
)

// akCharset 是 AK 使用的 62 个字母数字字符（a-z、A-Z、0-9）。
const akCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// skBytes 是 SK 的随机字节数（256 位熵）。
const skBytes = 32

// GetRandAk 生成并返回指定长度的随机 AK（Access Key）。
//
// 使用 crypto/rand 从 62 字符字母数字集合（a-z、A-Z、0-9）中无偏选取
// （rand.Int 拒绝采样），修复了历史上 math/rand、字符表重复 'O'、缺失 '0'
// 以及 Intn(61) 导致 '9' 永不出现的问题。length <= 0 时返回空字符串。
// 若读取 crypto/rand 失败（在支持的平台上几乎不可能），将 panic。
func GetRandAk(length int) string {
	if length <= 0 {
		return ""
	}
	ak := make([]byte, length)
	max := big.NewInt(int64(len(akCharset)))
	for i := range ak {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic("auth: read crypto/rand: " + err.Error())
		}
		ak[i] = akCharset[n.Int64()]
	}
	return string(ak)
}

// RefreshSK 生成并返回密码学安全的随机 SK（Secret Key）。
//
// 内部使用 crypto/rand 生成 32 字节（256 位）随机数据并 hex 编码（64 字符），
// 用作 AK/SK 认证方案的 HMAC 密钥。SK 不再由 ak 或时间戳派生，具备完整秘密熵。
// 若读取 crypto/rand 失败（在支持的平台上几乎不可能），将 panic。
func RefreshSK() string {
	b := make([]byte, skBytes)
	if _, err := rand.Read(b); err != nil {
		panic("auth: read crypto/rand: " + err.Error())
	}
	return hex.EncodeToString(b)
}
```

Note: `time` and `github.com/byx-darwin/go-tools/go-common/crypto` imports are removed (no longer used); `encoding/hex` is added.

- [ ] **Step 4: Run the tests to verify they pass (GREEN)**

Run: `go test ./go-common/auth/ -count=1`
Expected: PASS (all AK + SK + charset + integration tests).

- [ ] **Step 5: Commit**

```bash
git add go-common/auth/ak.go go-common/auth/ak_test.go
git commit -m "fix(go-common): generate SK with crypto/rand instead of MD5(ak+timestamp)

RefreshSK now returns 32 crypto/rand bytes hex-encoded (64 chars) and no
longer derives the key from ak + wall-clock time (zero secret entropy).
Breaking: drops the ak parameter (no external callers).

Closes #24"
```

---

### Task 3: Update the example caller

**Files:**
- Modify: `example/handler/common_aksk.go` (`RefreshSK(ak)` → `RefreshSK()`)

**Interfaces:**
- Consumes: `auth.RefreshSK() string` (no-arg, from Task 2), `auth.GetRandAk(32)`
- Produces: a compiling `example` workspace module

- [ ] **Step 1: Update the call site**

In `example/handler/common_aksk.go`, change the SK generation line and its comment:

```go
	// 生成随机 AK。
	ak := auth.GetRandAk(32)

	// 生成密码学安全的随机 SK。
	sk := auth.RefreshSK()
```

(The rest of the handler — `crypto.HMACSHA256([]byte(message), []byte(sk))` and the response map — is unchanged.)

- [ ] **Step 2: Verify the example module compiles**

Run: `go build ./example/...`
Expected: success (no output).

- [ ] **Step 3: Commit**

```bash
git add example/handler/common_aksk.go
git commit -m "chore(example): use no-arg RefreshSK in aksk handler

Refs #24"
```

---

### Task 4: Full verification & static analysis

**Files:** none (verification only; commit only if a fix is required)

- [ ] **Step 1: gofmt check**

Run: `gofmt -l go-common/auth/ak.go go-common/auth/ak_test.go example/handler/common_aksk.go`
Expected: no output (all files formatted). If any file is listed, run `gofmt -w <file>`.

- [ ] **Step 2: Module tests**

Run: `go test ./go-common/... -count=1`
Expected: PASS (all go-common packages).

- [ ] **Step 3: go vet**

Run: `go vet ./go-common/...`
Expected: no output.

- [ ] **Step 4: golangci-lint (per-module, v2)**

Run: `golangci-lint run --timeout=5m ./go-common/...`
Expected: no issues. (Verifies revive godoc on `RefreshSK`/`GetRandAk`/consts, errcheck on the handled `rand` errors, unparam — no unused params since `RefreshSK` is now no-arg.)

- [ ] **Step 5: Standard workspace build**

Run: `go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`
Expected: success (no output).

- [ ] **Step 6: Commit fixes (only if Steps 1–5 required any change)**

```bash
git add -A
git commit -m "chore(go-common): gofmt/lint fixes for crypto/rand SK

Refs #24"
```

If no changes were needed, skip this step.

---

## Acceptance Traceability (issue #24)

| Acceptance Criterion | Covered by |
|----------------------|-----------|
| `RefreshSK` 改用 `crypto/rand`（32 字节，hex 编码） | Task 2 (impl + `TestRefreshSK_Length`/`_HexCharset`) |
| 不再从标识符 + 时钟派生密钥 | Task 2 (no-arg `RefreshSK`, removes `ak`/`time`) |
| 更新 godoc 注释说明 SK 生成方式 | Task 2 (`RefreshSK` godoc), Task 1 (`GetRandAk` godoc) |
| 补充/更新单元测试验证 SK 长度与随机性 | Task 2 (`TestRefreshSK_*`), Task 1 (`TestGetRandAk_Coverage`/`TestAkCharset_Sanity`) |
| `go test ./go-common/... -count=1` 通过 | Task 4 Step 2 |
