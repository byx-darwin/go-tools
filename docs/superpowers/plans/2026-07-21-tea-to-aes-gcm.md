# TEA → AES-GCM Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the deprecated, insecure TEA cipher in `go-common/crypto` with an AES-GCM authenticated-encryption API, and stop silently swallowing cipher errors.

**Architecture:** Clean break — delete all TEA code and add a new Options-based `AESGCM` cipher object (`NewAESGCM(key, opts...)` + `Seal`/`Open`) built solely on the Go standard library (`crypto/aes`, `crypto/cipher`, `crypto/rand`). Ciphertext layout is `nonce(12) ‖ sealed(plaintext+tag)`. Construction and authentication failures are propagated as errors (sentinels `ErrInvalidKeySize` / `ErrCiphertextTooShort`), fixing the silent-swallow bug.

**Tech Stack:** Go 1.26 (workspace mode), Go standard library only (no new dependencies), stdlib `testing`, golangci-lint v2.

**Source design:** `docs/superpowers/specs/2026-07-21-tea-to-aes-gcm-design.md` · Issue #25 (closes #26).

## Global Constraints

- **Zero new dependencies**: use only the Go standard library (`crypto/aes`, `crypto/cipher`, `crypto/rand`, `errors`, `fmt`). After TEA removal, `golang.org/x/crypto` must no longer be required by `go-common`.
- **Lint (golangci-lint v2, ≥ v2.12.2)**: all exported symbols need a godoc comment starting with the symbol name (revive); every returned error must be handled (errcheck); no `//nolint` without an explanation; run lint per-module (`golangci-lint run --timeout=5m ./go-common/...`), never at the workspace root.
- **Public API**: the removed TEA functions (`EncodeTeaStr`, `DecodeTeaStr`, `GetTeaPadLen`, `TeaHexDecode`) are a deliberate breaking change (clean break, D2). Document the migration in godoc.
- **Validation order**: single test → package tests → `go build` → `go vet` → `golangci-lint` → `go test ./go-common/... -count=1`.
- **Commits**: one focused commit per task.

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `go-common/crypto/aes.go` | Create | AES-GCM API: `AESGCM`, `Option`, `WithAssociatedData`, `NewAESGCM`, `Seal`, `Open`, sentinel errors |
| `go-common/crypto/aes_test.go` | Create | stdlib table-driven tests: roundtrip, key sizes, invalid key, tamper, too-short, AAD, random nonce |
| `example/handler/common_crypto.go` | Modify | Replace the TEA demo block with an AES-GCM `Seal`/`Open` demo (keep the package compiling after TEA removal) |
| `go-common/crypto/tea.go` | Delete | All TEA functions + `golang.org/x/crypto/tea` import + `//nolint:staticcheck` |
| `go-common/crypto/encrypt.go` | Modify | Package doc: drop "TEA", state "AES-GCM 认证加密" |
| `go-common/crypto/encrypt_test.go` | Modify | Remove all TEA tests + the now-unused `encoding/hex` import |
| `go-common/go.mod` / `go.sum` | Modify | `go mod tidy` drops `golang.org/x/crypto` (tea.go is its only user in go-common) |

**Task ordering keeps every commit compilable:** add AES-GCM (Task 1) → migrate example off TEA (Task 2) → delete TEA + tidy (Task 3) → full validation gate (Task 4).

---

### Task 1: Add the AES-GCM API (TDD)

**Files:**
- Create: `go-common/crypto/aes.go`
- Test: `go-common/crypto/aes_test.go`

**Interfaces:**
- Produces: `NewAESGCM(key []byte, opts ...Option) (*AESGCM, error)`, `(*AESGCM) Seal(plaintext []byte) ([]byte, error)`, `(*AESGCM) Open(ciphertext []byte) ([]byte, error)`, `WithAssociatedData(aad []byte) Option`, `ErrInvalidKeySize`, `ErrCiphertextTooShort`.

- [ ] **Step 1: Write the failing test file**

Create `go-common/crypto/aes_test.go`:

```go
package crypto

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"
)

func TestAESGCMRoundtrip(t *testing.T) {
	key := []byte("0123456789abcdef") // 16 bytes
	tests := []struct {
		name      string
		plaintext []byte
	}{
		{"empty", []byte("")},
		{"single-byte", []byte("x")},
		{"short", []byte("hello")},
		{"one-block", []byte("data1234data1234")},
		{"multi-block", []byte("this is longer data spanning multiple blocks")},
	}

	g, err := NewAESGCM(key)
	if err != nil {
		t.Fatalf("NewAESGCM failed: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sealed, err := g.Seal(tt.plaintext)
			if err != nil {
				t.Fatalf("Seal failed: %v", err)
			}
			opened, err := g.Open(sealed)
			if err != nil {
				t.Fatalf("Open failed: %v", err)
			}
			if !bytes.Equal(opened, tt.plaintext) {
				t.Errorf("roundtrip mismatch: got %q, want %q", opened, tt.plaintext)
			}
		})
	}
}

func TestAESGCMKeySizes(t *testing.T) {
	plaintext := []byte("key size test")
	for _, size := range []int{16, 24, 32} {
		key := make([]byte, size)
		if _, err := rand.Read(key); err != nil {
			t.Fatalf("rand.Read failed: %v", err)
		}
		g, err := NewAESGCM(key)
		if err != nil {
			t.Fatalf("NewAESGCM(%d bytes) failed: %v", size, err)
		}
		sealed, err := g.Seal(plaintext)
		if err != nil {
			t.Fatalf("Seal failed: %v", err)
		}
		opened, err := g.Open(sealed)
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		if !bytes.Equal(opened, plaintext) {
			t.Errorf("roundtrip mismatch for %d-byte key", size)
		}
	}
}

func TestAESGCMInvalidKeySize(t *testing.T) {
	for _, size := range []int{0, 15, 17, 31, 33} {
		key := make([]byte, size)
		_, err := NewAESGCM(key)
		if !errors.Is(err, ErrInvalidKeySize) {
			t.Errorf("NewAESGCM(%d bytes) error = %v, want ErrInvalidKeySize", size, err)
		}
	}
}

func TestAESGCMTamperedCiphertext(t *testing.T) {
	key := []byte("0123456789abcdef")
	g, err := NewAESGCM(key)
	if err != nil {
		t.Fatalf("NewAESGCM failed: %v", err)
	}
	sealed, err := g.Seal([]byte("tamper me"))
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}
	sealed[len(sealed)-1] ^= 0xff // flip a bit in the tag
	if _, err := g.Open(sealed); err == nil {
		t.Error("Open should fail on tampered ciphertext")
	}
}

func TestAESGCMCiphertextTooShort(t *testing.T) {
	key := []byte("0123456789abcdef")
	g, err := NewAESGCM(key)
	if err != nil {
		t.Fatalf("NewAESGCM failed: %v", err)
	}
	_, err = g.Open([]byte("too short"))
	if !errors.Is(err, ErrCiphertextTooShort) {
		t.Errorf("Open(short) error = %v, want ErrCiphertextTooShort", err)
	}
}

func TestAESGCMAssociatedData(t *testing.T) {
	key := []byte("0123456789abcdef")
	plaintext := []byte("aad protected")

	sealer, err := NewAESGCM(key, WithAssociatedData([]byte("aad-A")))
	if err != nil {
		t.Fatalf("NewAESGCM(sealer) failed: %v", err)
	}
	sealed, err := sealer.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	openerSame, err := NewAESGCM(key, WithAssociatedData([]byte("aad-A")))
	if err != nil {
		t.Fatalf("NewAESGCM(openerSame) failed: %v", err)
	}
	opened, err := openerSame.Open(sealed)
	if err != nil {
		t.Fatalf("Open with matching AAD failed: %v", err)
	}
	if !bytes.Equal(opened, plaintext) {
		t.Errorf("AAD roundtrip mismatch: got %q, want %q", opened, plaintext)
	}

	openerDiff, err := NewAESGCM(key, WithAssociatedData([]byte("aad-B")))
	if err != nil {
		t.Fatalf("NewAESGCM(openerDiff) failed: %v", err)
	}
	if _, err := openerDiff.Open(sealed); err == nil {
		t.Error("Open should fail when AAD does not match")
	}
}

func TestAESGCMRandomNonce(t *testing.T) {
	key := []byte("0123456789abcdef")
	g, err := NewAESGCM(key)
	if err != nil {
		t.Fatalf("NewAESGCM failed: %v", err)
	}
	plaintext := []byte("same plaintext")
	c1, err := g.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal #1 failed: %v", err)
	}
	c2, err := g.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal #2 failed: %v", err)
	}
	if bytes.Equal(c1, c2) {
		t.Error("two Seal calls of the same plaintext should produce different ciphertexts (random nonce)")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails (compile error)**

Run: `go test ./go-common/crypto/ -run TestAESGCM -count=1`
Expected: FAIL — `undefined: NewAESGCM` (and `AESGCM`, `WithAssociatedData`, `ErrInvalidKeySize`, `ErrCiphertextTooShort`).

- [ ] **Step 3: Write the minimal implementation**

Create `go-common/crypto/aes.go`:

```go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

// ErrInvalidKeySize 表示 key 长度不是 16/24/32 字节（AES-128/192/256）。
var ErrInvalidKeySize = errors.New("crypto: invalid AES key size; must be 16, 24, or 32 bytes")

// ErrCiphertextTooShort 表示密文长度不足以包含 nonce 与认证 tag。
var ErrCiphertextTooShort = errors.New("crypto: ciphertext too short")

// AESGCM 是基于 AES-GCM 的认证加密器。
// Seal 产出的密文格式为 nonce(12 字节) ‖ 密封数据（含 16 字节 tag）。
type AESGCM struct {
	aead cipher.AEAD
	aad  []byte
}

// aesGCMOptions 持有 AESGCM 的可选配置。
type aesGCMOptions struct {
	aad []byte
}

// Option 配置 AESGCM。
type Option func(*aesGCMOptions)

// WithAssociatedData 设置附加认证数据 (AAD)。
// AAD 不参与加密，但参与完整性校验；Seal 与 Open 必须使用相同的 AAD。
func WithAssociatedData(aad []byte) Option {
	return func(o *aesGCMOptions) {
		o.aad = aad
	}
}

// NewAESGCM 使用 key 创建 AES-GCM 加密器。
// key 长度必须为 16/24/32 字节（对应 AES-128/192/256），否则返回 ErrInvalidKeySize。
func NewAESGCM(key []byte, opts ...Option) (*AESGCM, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: new AES cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new GCM: %w", err)
	}

	var o aesGCMOptions
	for _, opt := range opts {
		opt(&o)
	}

	return &AESGCM{aead: aead, aad: o.aad}, nil
}

// Seal 加密 plaintext，返回 nonce ‖ 密文 ‖ tag。
// 每次调用使用 crypto/rand 生成新的随机 nonce。
func (a *AESGCM) Seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, a.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("crypto: generate nonce: %w", err)
	}

	return a.aead.Seal(nonce, nonce, plaintext, a.aad), nil
}

// Open 解密 Seal 产出的密文并校验完整性。
// 密文长度非法返回 ErrCiphertextTooShort；密文被篡改或 AAD 不匹配时返回认证错误。
func (a *AESGCM) Open(ciphertext []byte) ([]byte, error) {
	nonceSize := a.aead.NonceSize()
	if len(ciphertext) < nonceSize+a.aead.Overhead() {
		return nil, ErrCiphertextTooShort
	}

	nonce, sealed := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := a.aead.Open(nil, nonce, sealed, a.aad)
	if err != nil {
		return nil, fmt.Errorf("crypto: decrypt: %w", err)
	}

	return plaintext, nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./go-common/crypto/ -run TestAESGCM -count=1 -v`
Expected: PASS — all `TestAESGCM*` tests ok.

- [ ] **Step 5: Commit**

```bash
git add go-common/crypto/aes.go go-common/crypto/aes_test.go
git commit -m "feat(go-common): add AES-GCM authenticated cipher (#25)"
```

---

### Task 2: Migrate the example handler off TEA

**Files:**
- Modify: `example/handler/common_crypto.go` (the TEA demo block, currently lines ~31-43)

**Interfaces:**
- Consumes: `crypto.NewAESGCM`, `(*crypto.AESGCM) Seal`, `(*crypto.AESGCM) Open` (from Task 1).

- [ ] **Step 1: Replace the TEA demo block with AES-GCM**

In `example/handler/common_crypto.go`, replace this block:

```go
	// TEA 加密/解密示例。
	teaKey := "1234567890123456" // TEA 需要 16 字节密钥
	encoded, pad, encErr := crypto.EncodeTeaStr([]byte("sensitive-data"), teaKey)
	if encErr == nil {
		decoded, decErr := crypto.DecodeTeaStr(encoded, pad, teaKey)
		if decErr == nil {
			results["tea_encoded_hex"] = fmt.Sprintf("%x", encoded)
			results["tea_decoded"] = string(decoded)
			results["tea_pad_len"] = pad
		}
	}
```

with:

```go
	// AES-GCM 认证加密/解密示例。
	aesKey := []byte("0123456789abcdef") // AES-128 需要 16 字节密钥
	aesCipher, newErr := crypto.NewAESGCM(aesKey)
	if newErr == nil {
		encoded, sealErr := aesCipher.Seal([]byte("sensitive-data"))
		if sealErr == nil {
			decoded, openErr := aesCipher.Open(encoded)
			if openErr == nil {
				results["aesgcm_encoded_hex"] = fmt.Sprintf("%x", encoded)
				results["aesgcm_decoded"] = string(decoded)
			}
		}
	}
```

- [ ] **Step 2: Verify the example package builds**

Run: `go build ./example/...`
Expected: build succeeds (TEA still exists, so nothing else breaks yet).

- [ ] **Step 3: Commit**

```bash
git add example/handler/common_crypto.go
git commit -m "chore(example): use AES-GCM instead of TEA in crypto demo (#25)"
```

---

### Task 3: Remove TEA, update package doc, drop the x/crypto dependency

**Files:**
- Delete: `go-common/crypto/tea.go`
- Modify: `go-common/crypto/encrypt.go` (package doc)
- Modify: `go-common/crypto/encrypt_test.go` (remove TEA tests + unused `encoding/hex` import)
- Modify: `go-common/go.mod`, `go-common/go.sum` (via `go mod tidy`)

**Interfaces:**
- Removes: `EncodeTeaStr`, `DecodeTeaStr`, `GetTeaPadLen`, `TeaHexDecode` (breaking change, D2 clean break).

- [ ] **Step 1: Delete tea.go**

```bash
git rm go-common/crypto/tea.go
```

- [ ] **Step 2: Update the package doc in encrypt.go**

In `go-common/crypto/encrypt.go`, replace:

```go
// Package crypto 提供加密/解密、哈希和 HMAC 工具函数。
//
// 支持 AES、TEA、MD5、SHA 系列算法，以及 HMAC 签名验证。
package crypto
```

with:

```go
// Package crypto 提供加密/解密、哈希和 HMAC 工具函数。
//
// 支持 AES-GCM 认证加密、MD5、SHA 系列哈希，以及 HMAC 签名验证。
package crypto
```

- [ ] **Step 3: Remove the TEA tests and the unused hex import from encrypt_test.go**

In `go-common/crypto/encrypt_test.go`:

(a) Change the import block from:

```go
import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)
```

to:

```go
import (
	"crypto/sha256"
	"testing"
)
```

(b) Delete these six functions entirely: `TestTEAEncodeDecode`, `TestTEAEncodeDecodeBlockAligned`, `TestTEAEncodeDecodeMultiBlock`, `TestGetTeaPadLen`, `TestTeaHexDecode`, `TestDecodeTeaStrWrongLength`. Keep `TestMD5`, `TestSHA1`, `TestSHA512`, `TestHmac`, `TestEncodePwd`.

- [ ] **Step 4: Drop the now-unused x/crypto dependency**

```bash
cd go-common && go mod tidy && cd ..
```

Expected: `golang.org/x/crypto` is removed from `go-common/go.mod` (tea.go was its only user in go-common). Verify: `grep -c "golang.org/x/crypto" go-common/go.mod` prints `0`.

- [ ] **Step 5: Verify go-common builds, vets, and tests clean without TEA**

Run:
```bash
go build ./go-common/...
go vet ./go-common/...
go test ./go-common/... -count=1
```
Expected: all succeed; `aes_test.go` tests pass; no remaining reference to TEA.

- [ ] **Step 6: Commit**

```bash
git add -A go-common/crypto go-common/go.mod go-common/go.sum
git commit -m "refactor(go-common)!: remove deprecated TEA cipher, closes #26 (#25)"
```

---

### Task 4: Full validation gate (lint + workspace build)

**Files:** none (verification only; commit any go.work.sum churn if produced)

- [ ] **Step 1: Confirm no lingering TEA references in Go sources**

Run: `grep -rn "Tea\|TEA\|x/crypto/tea" --include="*.go" go-common/ example/`
Expected: no matches in `go-common/crypto` or the example handler (the `go-framework/config/hertz` `TeaKey` field is a documented out-of-scope follow-up and is not searched here).

- [ ] **Step 2: Build the whole workspace**

Run: `go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... ./example/...`
Expected: build succeeds.

- [ ] **Step 3: Run golangci-lint v2 per-module on go-common**

Run: `golangci-lint run --timeout=5m ./go-common/...`
Expected: no issues (errcheck: all errors handled; revive: godoc on `AESGCM`, `Option`, `WithAssociatedData`, `NewAESGCM`, `Seal`, `Open`, `ErrInvalidKeySize`, `ErrCiphertextTooShort`; no unexplained `//nolint`).

- [ ] **Step 4: Run gofmt/goimports cleanliness check**

Run: `gofmt -l go-common/crypto/ example/handler/`
Expected: empty output (all files formatted).

- [ ] **Step 5: Full test sweep**

Run: `go test ./go-common/... -count=1`
Expected: PASS.

- [ ] **Step 6: Commit go.work.sum if it changed**

```bash
git add go.work.sum 2>/dev/null && git diff --cached --quiet || git commit -m "chore: sync go.work.sum after dropping x/crypto (#25)"
```

(Skip if `git status` shows no `go.work.sum` change.)
