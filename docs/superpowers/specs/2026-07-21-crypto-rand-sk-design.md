# 设计文档：SK 改用 crypto/rand 生成（go-common/auth）

- **Issue**: #24（generate SK with crypto/rand instead of MD5(ak+timestamp)）
- **跟踪父 Issue**: #34（多角色审计修复路线图）— Phase 1 / 组 C（go-common 密钥与加密）
- **里程碑**: 安全加固
- **日期**: 2026-07-21
- **模式**: full（gitflow-workflow `wf-2026-07-21-005`）

## 1. 背景与问题

`go-common/auth/ak.go` 的 `RefreshSK` 以 `SK = MD5(ak + "/" + time.Now().String())` 派生密钥：

- **零秘密熵**：SK 是保护 AK/SK 认证方案的 HMAC 密钥，却完全由一个**公开标识符**（ak）和一个**墙上时钟时刻**决定。在创建时刻可知或可收窄（注册时间、计划轮换）时，无盐 MD5 可在 GPU 速度下暴力破解。SK 必须具备独立的、密码学强度的随机熵。
- **同文件的相邻问题**：`GetRandAk` 使用 `math/rand`（非密码学安全），且字符表与取模存在多个历史 bug（见 §2）。

### 现状调用面

| 导出符号 | 签名 | 仓库内调用方 |
|---------|------|-------------|
| `RefreshSK` | `(ak string) string` | 仅 `example/handler/common_aksk.go` |
| `GetRandAk` | `(length int) string` | 仅 `example/handler/common_aksk.go` |

> **外部调用确认**：经确认，外部 / ncgo 生成的项目**未调用** `RefreshSK`，因此签名破坏性变更可接受。

## 2. 目标

1. `RefreshSK` 改用 `crypto/rand` 生成 SK，不再从标识符 + 时钟派生，确保密钥具备完整秘密熵。
2. 一并修复 `GetRandAk`：改用 `crypto/rand` 无偏选取字符，并修复字符表历史 bug。
3. 更新 godoc 注释说明新的生成方式。
4. 补充/更新单元测试验证长度、字符集与随机性。
5. 保持 `go-common` 零框架依赖（仅用标准库 `crypto/rand`、`encoding/hex`、`math/big`）。
6. 通过 golangci-lint v2（revive godoc、errcheck、unparam）。

### GetRandAk 现存 bug（本次一并修复）

原字符表 `"...ABCDEFGHIJKLOM" + "NOPQRSTUVWXYZ123456789"`（拼接长 62）存在三个问题：

| 问题 | 说明 |
|------|------|
| **`O` 重复** | 两个分块各含一个 `O`，抽到 `O` 的概率翻倍（有偏选取） |
| **缺失 `0`** | digits 仅 `1-9`，字符表中完全没有数字零 |
| **`9` 不可达** | `rand.Intn(61)` 仅取索引 0–60，排除最后一位 `9`（off-by-one） |

> 因此「修 off-by-one」不是把 61 改成 62（那会保留 O 重复有偏 + 缺 0），而是**规范化字符表为标准 62 字母数字**（`a-z` + `A-Z` + `0-9`，含 `0`，`O` 仅一次），一次修掉三个问题。

## 3. 关键决策

| # | 决策点 | 结论 | 理由 |
|---|--------|------|------|
| D1 | API 形态 | **`RefreshSK() string`**（去掉 `ak` 参数） | SK 不再依赖 ak，保留 `ak` 参数会误导调用方以为存在派生关系；已确认无外部调用方，破坏性变更可接受 |
| D2 | rand 失败处理 | **panic** | `crypto/rand.Read` 在支持的平台上实际不会失败；panic 是密钥生成惯用法（同 `ed25519.GenerateKey`）；保持 `string` 单返回值，避免破坏性 error 返回 |
| D3 | 编码/长度 | **hex，64 字符**（32 字节随机数） | 与 `crypto` 包现有约定一致（MD5/SHA/HMAC 均返回 hex）；无 padding、URL 安全；256 位熵充足 |
| D4 | 范围 | **一并修复 `GetRandAk`** | 同文件、同漏洞类别（凭据生成弱随机源）；`Intn(61)` 为真实正确性 bug；顺手规范化字符表 |

## 4. API 设计

修改文件 `go-common/auth/ak.go`：

```go
// akCharset 是 AK 使用的 62 个字母数字字符（a-z、A-Z、0-9）。
const akCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// skBytes 是 SK 的随机字节数（256 位熵）。
const skBytes = 32

// GetRandAk 生成并返回指定长度的随机 AK（Access Key）。
//
// 使用 crypto/rand 从 62 字符字母数字集合（a-z、A-Z、0-9）中无偏选取
// （rand.Int 拒绝采样），修复了历史上 math/rand、字符表重复 'O'、缺失 '0'
// 以及 Intn(61) 导致 '9' 永不出现的问题。
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

### import 变更

- **新增**：`crypto/rand`、`encoding/hex`、`math/big`
- **移除**：`math/rand`、`time`、`github.com/byx-darwin/go-tools/go-common/crypto`（不再使用 `crypto.MD5`）

## 5. 行为变更说明（破坏性）

| 变更 | 前 | 后 |
|------|----|----|
| `RefreshSK` 签名 | `RefreshSK(ak string) string` | `RefreshSK() string` |
| SK 来源 | `MD5(ak + "/" + time.Now())` | `crypto/rand` 32 字节 |
| SK 长度 | 32 字符（MD5 hex） | 64 字符（hex） |
| `GetRandAk` 随机源 | `math/rand`（有偏、`9` 不可达） | `crypto/rand`（无偏） |
| `GetRandAk` 字符表 | 62 字符但 `O` 重复、缺 `0` | 标准 62 字母数字（含 `0`/`9`，`O` 一次） |
| `GetRandAk(<=0)` | 返回 `""` | 返回 `""`（保持，显式 guard 避免 `make([]byte, -1)` panic） |

> 迁移说明将写入 godoc。`RefreshSK` 为破坏性签名变更，但已确认无外部调用方，仓库内唯一调用方（example）同 PR 更新。

## 6. 受影响文件

| 文件 | 变更 |
|------|------|
| `go-common/auth/ak.go` | 重写 `RefreshSK`/`GetRandAk`，新增常量，更新 import 与 godoc |
| `go-common/auth/ak_test.go` | 修订 SK 长度/字符集断言，改写依赖 ak 入参的用例，新增随机性/字符集健全性测试 |
| `example/handler/common_aksk.go` | `auth.RefreshSK(ak)` → `auth.RefreshSK()` |

## 7. 测试计划（stdlib `testing`）

`ak_test.go`：

1. **`TestRefreshSK_Length`**：`len(RefreshSK()) == 64`。
2. **`TestRefreshSK_HexCharset`**：输出仅含 `[0-9a-f]`。
3. **`TestRefreshSK_Uniqueness`**：多次调用（如 100 次）结果两两不同（256 位熵下碰撞概率可忽略）。替代原依赖 ak 入参的 `TestRefreshSK_DifferentAKProducesDifferentSK`。
4. **`TestGetRandAk_Length`**：`{0, 1, 5, 10, 32, 64}` 长度正确；负数返回 `""`。
5. **`TestGetRandAk_OnlyAlphanumeric`**：`validChars` 更新为规范字符表，输出字符均在其中。
6. **`TestGetRandAk_Uniqueness`**：多次调用结果两两不同。
7. **`TestAkCharset_Sanity`**（新增）：`akCharset` 恰为 62 个唯一字符（无重复、含 `0` 与 `9`）。
8. **`TestGetRandAk_Coverage`**（新增，统计性）：生成足够长 AK（如 10000 字符），断言 `0` 与 `9` 均出现（修复前 `9` 永不可达）。
9. **`TestIntegration_AKAndSK`**：端到端生成 AK(32) + SK，SK 长度断言由 32 改为 64。

验证命令：`go test ./go-common/... -count=1`。

## 8. 范围外（follow-up，建议另开 issue）

- 无。`GetRandAk` 的弱随机源与 off-by-one 已纳入本次（D4）。AK 为公开标识符，其安全性随 `crypto/rand` 已充分。

## 9. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 破坏性签名变更影响调用方 | 已确认无外部调用方；仓库内 example 同 PR 更新；godoc 明确说明 |
| 字符表规范化改变 AK 输出分布 | 属修复历史 bug（O 重复有偏、缺 0、9 不可达），非随机性退化；测试覆盖 |
| `crypto/rand` 读取失败导致 panic | 在支持的平台上实际不发生；panic 为密钥生成惯用法，消息可定位 |
| example 未同步导致 workspace 编译失败 | 同 PR 内更新 example handler |
