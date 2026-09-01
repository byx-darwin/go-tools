# PR #82 流水线健康分析报告

- **仓库**: byx-darwin/go-tools
- **PR**: #82 `feat(go-auth): JWT 密钥参数支持非对称算法类型 (RS256/ES256/EdDSA)`（Closes #73）
- **分支**: `feat/73-jwt-key-type-any` → `main`
- **PR 状态**: 已合并（created `2026-09-01T10:09:47Z` → merged `2026-09-01T10:10:06Z`，间隔约 19 秒）
- **分析工具**: `gf pipeline report/status/jobs`（只读，未触发/重跑/取消任何流水线）
- **分析时间窗**: 近 30 天（PR 分支）+ `main` 分支近 30 天作为基线对比
- **报告生成时间**: 2026-09-01

## 数据概览

| 维度 | 数据源 | 结果 |
|------|--------|------|
| PR 分支 (`feat/73-jwt-key-type-any`) | `gf pipeline report --branch feat/73-jwt-key-type-any --days 30` | totalRuns=1, successRate=0%, avgDuration=11s（*注：该数值是运行中快照，非最终结果，见下文说明*） |
| `main` 基线 | `gf pipeline report --branch main --days 30` | totalRuns=33, successRate=66.7%, avgDuration=142.6s |

**数据局限性说明**：PR 分支只有 1 次流水线运行（`33495981183`），样本量不足以做趋势分析；已结合 `main` 分支近 30 天 33 次运行、以及 PR 合并前后的两次相邻运行作为对比基线，弥补单分支数据不足。

## 一、成功率趋势

- `main` 分支近 30 天成功率 **66.7%**（33 次运行，22 成功 / 11 失败量级），低于健康阈值（建议 ≥80%）。🟡
- 时间序列观察：2026-09-01 当天 03:16–04:57 期间连续 **10 次成功**运行，随后 09:01:51 和 10:10:09（PR #82 合并后 3 秒触发）两次运行**均失败**，且失败作业完全相同（`go-middleware`、`go-framework`）。
- 结论：成功率下降不是随机波动，而是在某个时间点（约 08:xx–09:xx 之间，晚于最后一次成功运行 04:57、早于 09:01 失败）引入了一个**持续性（persistent）故障**，此后每次运行必现。

## 二、失败模式归类

### 根因定位

对 PR 分支运行 `33495981183` 及 `main` 上紧邻的两次失败运行 `33490082361`（09:01）、`33496010087`（10:10，PR #82 合并触发）分别抓取失败作业日志，三次失败的报错**完全一致**：

```
go-middleware  go mod tidy check  go: finding module for package github.com/byx-darwin/go-tools/go-auth/revocation
go-middleware  go mod tidy check  go: downloading github.com/byx-darwin/go-tools v0.2.2
go-middleware  go mod tidy check  go: github.com/byx-darwin/go-tools/go-middleware/auth imports
go-middleware  go mod tidy check  	github.com/byx-darwin/go-tools/go-auth/revocation: module github.com/byx-darwin/go-tools/go-auth@latest found (v0.1.0), but does not contain package github.com/byx-darwin/go-tools/go-auth/revocation
##[error]Process completed with exit code 1.
```

`go-framework` 作业的 `go mod tidy check` 步骤报错同源（`go-framework/hertz/middleware` 同样 import 了 `go-auth/revocation`）。

**归类：单一持续性故障，非 flaky**，影响 `go-middleware`、`go-framework` 两个作业的 "go mod tidy check" 步骤。

- 根因：`go-auth/revocation` 包是最近提交（`1760a07 feat(auth): Token 撤销机制 + 错误码传导到 HTTP 响应 #72`）新增的，但 `go-auth` 模块尚未发布包含该包的新 tag（模块代理上 `go-auth@latest` 仍解析为 `v0.1.0`）。CI 中的 `go mod tidy -diff` 检查未使用 `go.work` 的本地 replace 语义，而是按 Go module proxy 正常解析路径去请求已发布版本，因此必然失败，与 PR #82 本身改动（JWT 密钥类型）无关——是**已存在的仓库级基础设施问题**，PR #82 只是恰好在该问题窗口期内提交。

### 治理问题：合并未等待 CI 完成

PR #82 从创建到合并仅 **19 秒**（`10:09:47` → `10:10:06`），而其自身触发的流水线 `33495981183` 在合并时刻（`10:10:06`）仍处于 `running` 状态（`go-framework` 作业当时仍 `in_progress`，`go-middleware` 作业已失败）。这意味着：

1. 该 PR 是在 CI 结果不完整/已知失败的情况下被合并的；
2. 合并后立即触发的 `main` 分支运行（`33496010087`）复现了同样的失败，把破损状态带入了主干。

这提示 `main` 分支的分支保护规则（required status checks）可能未强制启用或未覆盖全部作业，属于流程风险而非代码缺陷。

## 三、耗时瓶颈分析

基于近期 4 次成功运行（`main`，2026-09-01 04:41–04:57）逐作业耗时统计：

| 作业 | 典型耗时 | 相对占比 |
|------|---------|---------|
| `go.mod hygiene` | ~5–7s | 最快，可忽略 |
| `go-auth` | ~50–70s | 中等 |
| `go-middleware` | ~65–75s | 中等偏高 |
| `go-common` | ~85–100s | 偏高 |
| `go-framework` | ~90–105s | **最高，瓶颈作业** |

- 流水线整体平均耗时（main，30 天）142.6s，与 `go-framework`/`go-common` 两个最长作业耗时量级吻合（作业并行执行，总时长由最慢作业决定）。
- `go-framework`、`go-common` 是稳定的耗时瓶颈，而不是本次分析窗口内的偶发现象（4 次采样耗时区间稳定，波动 <15%）。

## 四、Flaky Test 识别

- 本次分析窗口内**未发现 flaky test**（间歇性失败模式）。
- 唯二的两次相关失败（`33490082361`、`33496010087`）报错完全一致、可稳定复现，判定为**持续性基础设施故障**，不满足 flaky 的"间歇性/不可稳定复现"定义。
- 由于 PR 分支样本量仅 1 次运行，无法在分支粒度判断是否存在 flaky test；建议待该分支或后续同类分支积累更多运行后重新评估。

## 五、改进建议（按优先级排序）

1. **[P0] 修复 `go mod tidy check` 根因**：为 `go-auth` 打一个包含 `go-auth/revocation` 包的新 tag/release，或调整 CI 中 `go mod tidy -diff` 的执行方式使其感知 `go.work` 本地 replace（而非直接向 module proxy 解析未发布路径）。当前该故障会导致 `go-middleware`、`go-framework` 两个作业在后续所有运行中持续失败，直至修复。
2. **[P0] 核查 `main` 分支保护规则**：确认 "require status checks to pass before merging" 是否对 `go-middleware`、`go-framework` 等全部作业生效；PR #82 在自身 CI 未完成（且已知失败）情况下 19 秒内被合并，建议排查是否存在管理员绕过合并保护或规则未覆盖新增作业的情况。
3. **[P1] 优化耗时瓶颈**：为 `go-framework`、`go-common` 作业引入/复核 Go 构建与模块缓存（`actions/setup-go` 内置 cache 或 `actions/cache`），二者耗时（~90–105s）远高于 `go.mod hygiene`（~5s），是流水线总时长的主要来源。
4. **[P2] 补充数据积累**：当前 PR 分支仅 1 次运行，建议在该分支或同类新分支积累 ≥5 次运行后重新运行本分析，以获得更可靠的成功率与 flaky 判定。

## 结论

本次分析发现 **3 项问题**：
1. 持续性（非 flaky）CI 故障 — `go-auth/revocation` 包未随 tag 发布，导致 `go-middleware`/`go-framework` 的 `go mod tidy check` 必现失败；
2. 流程治理风险 — PR #82 在自身 CI 未完成时被合并，怀疑分支保护未生效或被绕过；
3. 耗时瓶颈 — `go-framework`、`go-common` 作业耗时显著高于其他作业，建议引入构建缓存。

未发现 flaky test。所有分析均为只读操作，未触发/重跑/取消任何流水线，未修改 CI 配置。
