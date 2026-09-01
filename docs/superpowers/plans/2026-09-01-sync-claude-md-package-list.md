# CLAUDE.md 子包清单同步 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 消除 `CLAUDE.md` / `specs/00_overview.md` / `specs/01_split_plan_summary.md` 中记录的 go-common 子包清单、TEA 加密能力与实际代码之间的脱节。

**Architecture:** 纯文档修改，不涉及代码变更。逐文件核对实际目录结构（`go-common/*`）与实际导出函数（`crypto` 包），修正三处文档，并核查 `specs/02-08` 是否存在类似问题（已核查：无）。

**Tech Stack:** Markdown 文档编辑。

**Spec:** Issue #60 — https://github.com/byx-darwin/go-tools/issues/60

## Global Constraints

- 仅修改文档，不改动 `go-common/*` 任何 `.go` 文件。
- TEA 算法当前未实现（`go-common/crypto/` 仅有 `encrypt.go`: MD5/SHA1/SHA256/SHA512/Hmac/HMACSHA256/EncodePwd；`aes.go`: AES-GCM）——按 Issue AC，移除"已支持 TEA"的表述，不在本 Issue 内实现 TEA。
- 已核查 `specs/02_config_schema_alignment.md` ~ `specs/08_hertz_response_redesign.md`：均不含 `astutil`/`executil`/`templateutil`/`TEA` 相关表述，无需修改。

---

### Task 1: 修正 CLAUDE.md 子包清单与 crypto 能力描述

**Files:**
- Modify: `CLAUDE.md:15`（Structure 代码块，go-common 子包括号列表）
- Modify: `CLAUDE.md:29`（Module 表格，`go-common` 行 Purpose 列）
- Modify: `CLAUDE.md:112`（Workspace Layout 代码块，`crypto/` 行注释）
- Modify: `CLAUDE.md:109-118`（Workspace Layout 代码块，补充 `astutil/`、`executil/`、`templateutil/` 三行）

**Interfaces:**
- 无代码接口，纯文本替换。

- [ ] **Step 1: 核对当前 go-common 实际子包**

已核对（本计划编写时执行）：`go-common/` 下实际子包为 `astutil, auth, cache, captcha, crypto, error, executil, httpclient, log, netutil, templateutil, timeutil`（12 个，均有 `go.mod` 同级目录，非独立 module，是同一 module 内的子包）。

- [ ] **Step 2: 修改 CLAUDE.md:15**

当前内容：
```
                        ↑          (crypto, cache, httpclient, log, timeutil, netutil, captcha, error)
```

替换为：
```
                        ↑          (crypto, cache, httpclient, log, timeutil, netutil, captcha, error, auth, astutil, executil, templateutil)
```

- [ ] **Step 3: 修改 CLAUDE.md:29**

当前内容：
```
| `go-common` | `github.com/byx-darwin/go-tools/go-common` | Pure utilities: crypto, cache, log, error, timeutil, netutil, httpclient, captcha |
```

替换为：
```
| `go-common` | `github.com/byx-darwin/go-tools/go-common` | Pure utilities: crypto, cache, log, error, timeutil, netutil, httpclient, captcha, auth, astutil, executil, templateutil |
```

- [ ] **Step 4: 修改 CLAUDE.md:112（crypto 能力描述）**

当前内容：
```
  crypto/                  → Encryption (MD5/SHA/HMAC/TEA)
```

替换为：
```
  crypto/                  → Encryption (MD5/SHA/HMAC/AES-GCM)
```

- [ ] **Step 5: 在 CLAUDE.md Workspace Layout 代码块中补充缺失子包**

在 `CLAUDE.md:117`（`auth/  → Auth helpers (AK/SK)`）之后、`error/` 行之前或之后，按字母顺序插入三行（保持代码块内其余行不变）：

```
  astutil/                 → Go AST manipulation (dave/dst-based, codegen helper)
  executil/                → Enhanced command execution wrapper
  templateutil/            → Pluggable template helper functions
```

最终 `go-common/` 代码块段（`CLAUDE.md:109-118`）应为：
```
go-common/                 → Zero-dependency utilities
  cache/                   → Generic cache (samber/hot wrapper): LRU/LFU/FIFO/TwoQueue/ARC
  captcha/                 → CAPTCHA generation with cache
  crypto/                  → Encryption (MD5/SHA/HMAC/AES-GCM)
  httpclient/              → HTTP client with retry, m3u8 support
  log/                     → Structured logging (slog + lumberjack + OTel)
  netutil/                 → Network utilities
  timeutil/                → Time formatting helpers
  auth/                    → Auth helpers (AK/SK)
  astutil/                 → Go AST manipulation (dave/dst-based, codegen helper)
  executil/                → Enhanced command execution wrapper
  templateutil/            → Pluggable template helper functions
  error/                   → Error mechanism (oops Builder/Extract + band boundaries + HTTP status registry)
```

- [ ] **Step 6: 校验 Markdown 渲染无误**

Run: `grep -n "astutil\|executil\|templateutil" CLAUDE.md`
Expected: 命中 Step 2/3/5 新增的行，共 3+ 处。

Run: `grep -n "TEA" CLAUDE.md`
Expected: 无命中（TEA 描述已移除）。

- [ ] **Step 7: Commit**

```bash
git add CLAUDE.md
git commit -m "docs(go-tools): sync CLAUDE.md go-common package list with actual code (#60)"
```

---

### Task 2: 修正 specs/00_overview.md 与 specs/01_split_plan_summary.md 的 TEA 描述

**Files:**
- Modify: `specs/00_overview.md:38`
- Modify: `specs/01_split_plan_summary.md:42`
- Modify: `specs/01_split_plan_summary.md:146`

**Interfaces:**
- 无代码接口，纯文本替换。

- [ ] **Step 1: 修改 specs/00_overview.md:38**

当前内容：
```
| `crypto` | 加密（MD5/SHA/HMAC/TEA） |
```

替换为：
```
| `crypto` | 加密（MD5/SHA/HMAC/AES-GCM） |
```

- [ ] **Step 2: 修改 specs/01_split_plan_summary.md:42**

当前内容：
```
| `tools/crypto/*` | `go-common/crypto/` | MD5/SHA/HMAC/TEA |
```

替换为：
```
| `tools/crypto/*` | `go-common/crypto/` | MD5/SHA/HMAC/AES-GCM |
```

- [ ] **Step 3: 修改 specs/01_split_plan_summary.md:146**

先读取该行上下文（前后各 10 行），确认这是一段 API 签名示例代码块。当前内容：
```
func TEAEncrypt(data, key []byte) ([]byte, error)
```

删除该行（TEA 从未实现，属于过时的历史规划签名，不应保留在"已确认接口"示例中）。若该代码块删除此行后括号/代码围栏不完整，需同步调整围栏结构，保持代码块语法合法。

- [ ] **Step 4: 校验**

Run: `grep -rn "TEA" specs/00_overview.md specs/01_split_plan_summary.md`
Expected: 无命中。

- [ ] **Step 5: Commit**

```bash
git add specs/00_overview.md specs/01_split_plan_summary.md
git commit -m "docs(specs): remove unimplemented TEA references from spec 00/01 (#60)"
```

---

## Self-Review Notes (completed during plan authoring)

- **Spec coverage:** AC1（子包清单同步）→ Task 1；AC2（移除/修正 TEA 描述）→ Task 2；AC3（核查 00-08 其他脱节）→ 已在 Global Constraints 中记录核查结果（02-08 无问题，无需新增 Task）。
- **Placeholder scan:** 无 TBD/TODO，所有替换文本均为完整最终内容。
- **Type consistency:** 不适用（纯文档任务，无代码接口）。
