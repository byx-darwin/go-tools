# PR #84 代码审查报告

- **仓库**: byx-darwin/go-tools
- **PR**: [#84](https://github.com/byx-darwin/go-tools/pull/84) — feat(auth): Refresh Token 缺少轮换与复用检测机制
- **分支**: `feat/74-jwt-refresh-rotation` → `main`
- **关联 Issue**: #74
- **审查时间**: 2026-09-01
- **审查人**: Claude（gf-workflow Phase 4 独立审查，`gf-pr-review` 6 维度分析）
- **PR 状态**: Open（未合并）
- **审查性质**: 本 PR 在提交前已经过内部完整审查流程（subagent-driven-development 3 个任务的逐任务审查 + 一轮修复，加一次面向安全的全分支终审，发现并修复 6 项 Important 问题）。本报告是 Phase 4 要求的**独立验证**，不依赖也不采信上述历史审查结论，重新核查全部改动。
- **⚠️ 流程说明（重要）**: 通过 `gf auth status` 确认当前 `gf` 认证用户为 `byx-darwin`，与本 PR 作者（`byx-darwin`）**为同一账号**，属于自审（self-review）。`gf-review` 技能规则明确"禁止自审 / Reviewing your own PR → 拒绝"，因此**本次审查结论仅以书面报告形式产出，未通过 `gf review` 提交至 PR**。如需正式 approve/request-changes 记录写入 PR，需由另一账号或人工执行 `gf review`。

## 变更概览

| 文件 | 改动 |
|------|------|
| `go-auth/jwt/token.go` | `Refresh` 签名破坏性变更：新增 `ctx context.Context`、必选 `store revocation.Store` 参数；成功路径撤销旧 JTI 并签发新 JTI（`uuid.NewString()`）；复用检测命中返回 `autherror.ErrTokenRevoked`；无 JTI 时跳过轮换（向后兼容）；store 错误 fail-closed |
| `go-auth/jwt/options.go` | 包注释示例同步新签名 |
| `go-auth/jwt/token_test.go` | 新增/更新轮换成功、复用检测、无 JTI 跳过、`IsRevoked`/`Revoke` 错误、nil-store、缺失 `exp` 等用例 |
| `go-auth/go.mod` / `go.sum` | 新增 `github.com/google/uuid` 叶子依赖 |
| `go-middleware/auth/revocation_memory.go` | 新增 `MemoryRevocationStore`（基于 `samber/hot` 的内存版 `revocation.Store` 实现） |
| `go-middleware/auth/revocation_memory_test.go` | 接口一致性、撤销/查询、TTL≤0 no-op、TTL 过期四组用例 |
| `example/handler/auth_jwt.go` | `jwtRefreshHandler` 接入新 `Refresh` 签名 |
| `example/main.go` | 接线 `MemoryRevocationStore` 至 example |
| `docs/superpowers/plans/…`、`specs/…` | 设计与实施计划文档（非代码） |

## 6 维度核查结论

### 1. 正确性（Correctness）—— ✅ 通过

- `go-auth/jwt/token.go` 的 `Refresh` 中，nil-store 防护先于任何 `store` 方法调用；`IsRevoked`/`Revoke` 返回错误时均 fail-closed（不签发新 token）；缺失 `exp` 声明时同样 fail-closed，避免对 `rc.ExpiresAt.Time` 的空解引用。
- 三条路径区分正确并均有对应测试覆盖：
  - 无 JTI（`ExtractJTI` 返回 `(_, false)`）→ 完全跳过轮换逻辑，保持旧签名调用方的行为不变（向后兼容）。
  - 命中复用（`IsRevoked == true`）→ 返回 `ErrTokenRevoked`，不签发新 token。
  - 成功路径 → 先撤销旧 JTI，再通过反射向同一 `*gojwt.RegisteredClaims` 指针写入 `uuid.NewString()` 后签发，别名关系正确（`rc` 与 `claims` 指向同一底层结构）。
- 全仓库 `go build`/`go vet` 通过，`gofmt -l` 无输出。新增测试（含 `-race`）全部 PASS。

### 2. 安全性（Security）—— ✅ 通过

- 一次性轮换 + 复用检测的语义与 PR 描述一致，实现无逃逸路径。
- 已知的 TOCTOU 竞态（`IsRevoked`→`Revoke` 两步非原子）在 `Refresh` 的 godoc 中有明确说明（`go-auth/jwt/token.go` 函数注释），与 PR 描述中"已记录但本次不修复"的条目一致，非新增问题。
- 非对称算法（RS256/ES256/EdDSA）暂不支持一节同样已在 godoc 中说明，为改动前既有限制，非本 PR 引入。
- `MemoryRevocationStore` 的 LRU 淘汰可能在 TTL 到期前因内存压力提前"遗忘"撤销记录，导致理论上的 fail-open——该风险已在类型 godoc 中明确标注仅适用于开发/测试场景，与 PR 描述中"已修复的文档化条目"一致。

### 3. 性能（Performance）—— ✅ 通过（一处可忽略的重复计算，见"次要发现"）

`Refresh` 非高频路径，性能层面无阻断性问题。

### 4. 可维护性（Maintainability）—— ⚠️ 轻微，非阻断（见"新发现"）

`go-auth/jwt/token.go` 中 `ExtractJTI(claims)` 与随后的 `extractRegisteredClaims(any(claims).(gojwt.Claims))` 对同一输入做了两次等价的反射式提取。当前安全性依赖"两次提取结果确定性一致、`rc` 不会为 nil"这一隐式假设，未在调用处显式校验或注释说明。目前不构成实际 bug，但属于脆弱点——若日后单独重构任一提取函数，可能悄然引入空指针风险。

### 5. 测试覆盖（Test Coverage）—— ✅ 通过

- `go-auth/jwt/token_test.go`：轮换成功、复用检测、无 JTI 跳过、`IsRevoked` 错误、`Revoke` 错误、nil-store + 有 JTI、缺失 `exp` + 有 JTI，均有独立用例并对错误码（`CodeTokenRevoked`/`CodeJWTRefreshFailed`）做了断言。
- `go-middleware/auth/revocation_memory_test.go`：接口一致性、撤销后查询、TTL≤0 no-op、TTL 真实过期，四组用例覆盖核心行为。
- `-race` 下运行全部新增测试无告警。

### 6. 文档（Documentation）—— ✅ 通过

- 新增导出符号（`MemoryRevocationStore`、`NewMemoryRevocationStore`、`Revoke`、`IsRevoked`、`SetRevocationStore`）均有以符号名开头的 godoc 注释，符合 `.claude/rules/go.md` §8.3。
- `Refresh` 的 godoc 完整覆盖了轮换语义、TOCTOU 警示、非对称算法限制、"调用方需持久化新 token"提示。
- `go-auth/jwt/options.go` 包级示例已同步新签名。

## 模块边界与破坏性变更专项核查

- **模块依赖边界** —— ✅：`go-auth/go.mod` 仅新增外部叶子依赖 `google/uuid`，未引入新的内部跨模块 import；`go-middleware/auth` 依赖 `go-auth/revocation` 属已允许方向（`go-middleware` 可依赖 `go-auth` + `go-common`），且该依赖关系在本 PR 之前已因 `revocation_redis.go` 存在，未违反 `.claude/rules/go.md` §4 的 DAG 约束。
- **破坏性 API 变更** —— ✅：`jwt.Refresh` 新增 `ctx`、`store` 两个必选参数，已在函数 godoc 与包示例中说明；全仓库检索 `jwt.Refresh[` 调用点，仅 `example/handler/auth_jwt.go` 一处，已同步更新，无遗留旧签名调用。

## 新发现（超出 PR 描述中已记录的"暂缓/待办"条目）

以下均为**低严重度、非阻断**问题，且均不同于 PR 描述中已列出的三项已知条目（TOCTOU 竞态、非对称算法限制、example 的 refresh_token 过期时间沿用 access token 有效期）：

1. **可维护性**（`go-auth/jwt/token.go`，`Refresh` 函数体内 `ExtractJTI` 调用处与紧随其后的 `extractRegisteredClaims` 调用处）：两次对同一 claims 做等价反射提取，第二次提取后未对结果做 nil 防御，正确性依赖未显式声明的"两次提取结果一致"隐式契约。建议后续将 `ExtractJTI` 改为同时返回 `*RegisteredClaims`，或提炼一个内部 helper 同时给出两者，从而消除重复调用与隐式假设。**不要求本次处理**。
2. **性能**：同一处重复反射提取带来的开销可忽略不计（`Refresh` 非热路径），仅作记录，不构成问题。

未发现新的正确性、安全性或并发问题；PR 描述中已披露的三项条目（TOCTOU 竞态、非对称算法限制、example 刷新令牌过期时间）均已如实记录，未发现被低估或遗漏的严重程度。

## 验证记录

在独立 worktree（基于 `origin/feat/74-jwt-refresh-rotation`）中复核：

```text
go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... ./example/...  → 通过
go vet   ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...                  → 通过
gofmt -l <all .go files>                                                                        → 无输出
golangci-lint run ./go-auth/...      → 0 issues
golangci-lint run ./go-middleware/... → 0 issues
go test ./go-auth/... ./go-middleware/... -run 'Refresh|RevocationStore' -race -v              → 全部 PASS
go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1          → 全部 PASS
```

与 PR 描述中声明的 Test plan 一致，结论可复现。

## 结论

6 个维度中 5 项直接通过，1 项（可维护性）存在一处轻微、非阻断的次要发现（重复反射提取、隐式 nil 安全假设），不影响功能正确性，可作为后续小改进处理。未发现任何超出 PR 描述已披露范围的新增正确性/安全性/并发问题。构建、vet、lint、全量测试（含 `-race`）均通过。

**独立审查结论：建议 Approve（无阻断性问题）。**

**流程限制**：因审查执行账号（`byx-darwin`）与 PR 作者账号相同，构成自审，本报告未通过 `gf review` 提交正式 verdict 至 PR，仅作为 Phase 4 独立审查记录留档。如需在 PR 上留下可见的正式审查结论，需更换审查账号后执行 `gf review approve 84 --body "..."`，或由人工在 GitHub 上完成。
