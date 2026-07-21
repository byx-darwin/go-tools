# 设计文档：TEA → AES-GCM 迁移（go-common/crypto）

- **Issue**: #25（replace deprecated TEA cipher with AES-GCM），同时包含并关闭 #26（propagate cipher construction errors）
- **跟踪父 Issue**: #34（多角色审计修复路线图）— Phase 1 / 组 C（go-common 密钥与加密）
- **里程碑**: 安全加固
- **日期**: 2026-07-21
- **模式**: full（gitflow-workflow `wf-2026-07-21-001`）

## 1. 背景与问题

`go-common/crypto/tea.go` 使用已在 `golang.org/x/crypto` 中弃用的 TEA 密码：

- **弱密码**：64 位分组、相关密钥/等效密钥弱点；逐块独立加密（ECB 式，无 IV/链接/MAC）。相同明文块产生相同密文（等值泄漏），且无 MAC 使密文可延展。
- **静默吞错（#26）**：`DecodeTeaStr`（tea.go:24-26）与 `EncodeTeaStr`（tea.go:65-67）在 cipher 构造失败时返回零值 + `nil` error，调用方无法感知加密设置失败。违反 `.claude/rules/go.md` §5（不得静默吞错）。
- 包注释中的 `//nolint:staticcheck` 也承认正在等待迁移到 AES。

### 现状调用面

| 导出符号 | 签名 | 仓库内调用方 |
|---------|------|-------------|
| `EncodeTeaStr` | `(src []byte, teaKey string) ([]byte, int, error)` | 仅 `example/handler/common_crypto.go` |
| `DecodeTeaStr` | `(src []byte, pad int, teaKey string) ([]byte, error)` | 仅 `example/handler/common_crypto.go` |
| `GetTeaPadLen` | `(length int) int` | 仅 `tea.go` 内部 |
| `TeaHexDecode` | `(hexBody []byte, bLen int, teaKey string) ([]byte, error)` | 无 |

> `go-framework/config/hertz/config.go` 含一个 `TeaKey string` 配置字段（数据字段，非函数调用），不受本次删除影响，但其语义将变得误导，列为后续 follow-up。

## 2. 目标

1. 彻底移除 TEA，新增基于 **AES-GCM** 的认证加密 API（机密性 + 完整性）。
2. 修复 #26：所有 cipher 构造错误与认证失败错误均正确传播。
3. 保持 `go-common` 零框架依赖（仅用标准库 `crypto/aes`、`crypto/cipher`、`crypto/rand`，不新增第三方依赖）。
4. 新代码遵循 Functional Options 模式（用户选定 Options 密码器对象形态）。
5. 通过 golangci-lint v2（errcheck 无吞错、revive 导出符号 godoc）。

## 3. 关键决策

| # | 决策点 | 结论 | 理由 |
|---|--------|------|------|
| D1 | 算法 | **AES-GCM** | Issue 标题已指定；标准库 `cipher.NewGCM` 原生支持；认证加密一步到位 |
| D2 | 向后兼容 | **彻底替换（clean break）** | TEA 已破损，为破损密码保留解密通道是反模式；仓库内唯一调用方仅 example；本库尚未正式发布、独立版本化 |
| D3 | API 形态 | **Options 密码器对象** `NewAESGCM(key, opts...)` + `Seal`/`Open` | 用户选定；符合 Functional Options 模式；便于扩展 AAD |
| D4 | 错误处理 | stdlib `errors` + 哨兵错误 | 与现有 crypto 包风格一致，不引入错误码耦合（go-common 无分配码段） |

## 4. API 设计

新文件 `go-common/crypto/aes.go`：

```go
// AESGCM 是基于 AES-GCM 的认证加密器。
// 密文格式为 nonce(12 字节) ‖ 密封数据（含 16 字节 tag）。
type AESGCM struct {
    aead cipher.AEAD
    aad  []byte
}

// Option 配置 AESGCM。
type Option func(*aesGCMOptions)

// WithAssociatedData 设置附加认证数据 (AAD)。
// AAD 不参与加密，但参与完整性校验；Seal 与 Open 必须使用相同 AAD。
func WithAssociatedData(aad []byte) Option

// NewAESGCM 使用 key 创建 AES-GCM 加密器。
// key 长度必须为 16/24/32 字节（对应 AES-128/192/256），否则返回 ErrInvalidKeySize。
func NewAESGCM(key []byte, opts ...Option) (*AESGCM, error)

// Seal 加密 plaintext，返回 nonce ‖ 密文 ‖ tag。
// 每次调用使用 crypto/rand 生成新的随机 nonce。
func (a *AESGCM) Seal(plaintext []byte) ([]byte, error)

// Open 解密 Seal 产出的密文并校验完整性。
// 密文长度非法返回 ErrCiphertextTooShort；密文被篡改或 AAD 不匹配返回认证错误。
func (a *AESGCM) Open(ciphertext []byte) ([]byte, error)
```

### 哨兵错误

```go
// ErrInvalidKeySize 表示 key 长度不是 16/24/32 字节。
var ErrInvalidKeySize = errors.New("crypto: invalid AES key size; must be 16, 24, or 32 bytes")

// ErrCiphertextTooShort 表示密文长度不足以包含 nonce 与 tag。
var ErrCiphertextTooShort = errors.New("crypto: ciphertext too short")
```

### 数据格式

- **nonce**：12 字节（GCM 标准推荐长度），`Seal` 时由 `crypto/rand.Read` 生成，前置于输出。
- **输出布局**：`nonce(12) ‖ gcm.Seal(plaintext)`，其中 `gcm.Seal` 输出 = 密文 ‖ tag(16)。
- **Open**：先校验 `len(ciphertext) >= nonceSize + tagSize(overhead)`，否则返回 `ErrCiphertextTooShort`；拆分 `nonce` 与 `sealed` 后调用 `gcm.Open`。
- **AAD**：由 `WithAssociatedData` 注入，`Seal`/`Open` 使用同一实例配置。

### 迁移说明（写入 godoc）

本次为**破坏性变更**：移除全部 TEA 函数（`EncodeTeaStr`/`DecodeTeaStr`/`GetTeaPadLen`/`TeaHexDecode`），不提供 TEA 兼容解密。需解密历史 TEA 数据的调用方应在升级前自行迁移数据。

## 5. 错误处理（修复 #26）

| 场景 | 行为 |
|------|------|
| key 长度非法 | `NewAESGCM` 返回 `ErrInvalidKeySize` |
| `aes.NewCipher` / `cipher.NewGCM` 构造失败 | `NewAESGCM` 包装并返回该 error |
| `crypto/rand` 读取 nonce 失败 | `Seal` 返回该 error |
| 密文过短 | `Open` 返回 `ErrCiphertextTooShort` |
| 认证失败（篡改 / AAD 不匹配） | `Open` 原样传播 `gcm.Open` 的 error |

> 不再有任何路径返回 `(零值, nil)` 来掩盖失败。

## 6. 受影响文件

| 文件 | 变更 |
|------|------|
| `go-common/crypto/tea.go` | **删除**（含 `golang.org/x/crypto/tea` 导入与 nolint 豁免） |
| `go-common/crypto/aes.go` | **新增** AES-GCM API |
| `go-common/crypto/encrypt.go` | 更新包注释：当前声称"支持 AES、TEA"但实际无 AES → 改为反映 AES-GCM，移除 TEA |
| `go-common/crypto/encrypt_test.go` | 移除全部 TEA 测试 |
| `go-common/crypto/aes_test.go` | **新增** AES-GCM 测试 |
| `example/handler/common_crypto.go` | TEA 演示段替换为 `NewAESGCM` + `Seal`/`Open` |

> `go.mod`：`golang.org/x/crypto` 若仅被 tea.go 使用则可移除（实现阶段确认；如 crypto 包其他文件或 go-common 他处仍用则保留）。

## 7. 测试计划（stdlib `testing`，表驱动）

`aes_test.go`：

1. **往返**：空 / 单块 / 多块明文，`Open(Seal(x)) == x`。
2. **key 长度**：16/24/32 字节均可构造并往返。
3. **非法 key**：长度 0/15/17/33 → `NewAESGCM` 返回 `ErrInvalidKeySize`。
4. **篡改密文**：翻转密文字节 → `Open` 返回错误。
5. **密文过短**：`Open([]byte{...} < nonce+tag)` → `ErrCiphertextTooShort`。
6. **AAD 不匹配**：Seal 用 AAD=A，Open 用 AAD=B → 失败。
7. **随机 nonce**：同一明文两次 `Seal` 密文不同。

验证命令：`go test ./go-common/... -count=1`。

## 8. 范围外（follow-up，建议另开 issue）

- `go-framework/config/hertz` 的 `TeaKey` 字段重命名/移除：涉及 ncgo 模板，contract-sensitive，不纳入本次最小改动。

## 9. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 破坏性 API 变更影响 ncgo 生成项目 | 本库尚未正式发布（独立版本化、#29 待办）；审计属安全加固里程碑；godoc 明确迁移说明 |
| example 未同步导致 workspace 编译失败 | 同 PR 内更新 example handler |
| 历史 TEA 存量数据无法解密 | 决策 D2 已确认采用 clean break；如需兼容应另开 issue |
