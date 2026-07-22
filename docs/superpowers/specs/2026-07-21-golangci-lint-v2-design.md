# Design: 对齐 golangci-lint 到 v2（Issue #32）

- **日期**: 2026-07-21
- **Issue**: #32 — chore(ci): align golangci-lint version with v2 config
- **跟踪**: #34（多角色审计路线图，Phase 0「最先」项）
- **里程碑**: 工程卫生（#3）
- **模式**: fast（CI/工具链 chore，低风险、验收标准明确）

## 1. 问题

仓库 lint 配置与工具链版本错位，导致项目自身 lint 规则**实际未生效**：

| 位置 | 现状 |
|------|------|
| `.golangci.yml` | `version: "2"`（已是 v2 格式） |
| 本地二进制 `~/go/bin/golangci-lint` | `v1.64.8`（v1）→ 运行报错 *"configuration file for golangci-lint v2 with golangci-lint v1"* |
| `.github/workflows/ci.yml` | golangci-lint 步骤**被注释禁用**，注释称「等待 Go 1.25 兼容版本」，并固定了已过时的 `v2.2.0` |
| `.pre-commit-config.yaml` | `go-lint` hook 调用系统 `golangci-lint`（即本地 v1.64.8，同样报错） |

后果：`revive`（导出符号 godoc 检查）、`gocritic` 等项目规则在本地与 CI 都不跑，风格回归无人察觉。

## 2. 决策：升级到 v2（而非降级配置到 v1）

Issue 给出「二选一」：① 升级 golangci-lint 到 v2；② 把 `.golangci.yml` 降回 v1。

**结论：选 ①，升级到 golangci-lint v2（当前最新稳定版 `v2.12.2`，2026-05-06 发布）。**

理由：

1. **配置已是 v2 格式**——降级到 v1 会丢弃已完成的迁移工作、方向倒退。
2. **Go 1.25 兼容性已解决**——CI 注释中「等待 Go 1.25 兼容版本」的顾虑已过时：`v2.12.2` 以 `go1.25.8` 构建并可正常加载本项目 v2 配置（已实测 `config verify` 通过）。
3. v2 是上游当前主线，v1 已停止功能演进。

## 3. 验证发现（实测，v2.12.2）

将 v2.12.2 安装到**临时 GOBIN**（不覆盖用户本地二进制）后逐 module 运行：

| Module | 结果 |
|--------|------|
| go-common | 0 issues |
| go-auth | **1 issue** — `go-auth/jwt/token.go:128` govet(inline): `reflect.Ptr should be inlined` |
| go-middleware | 0 issues |
| go-framework | 0 issues |

唯一违规：`reflect.Ptr` 是 Go 1.18 起废弃的别名，应改用 `reflect.Pointer`（二者为同一常量值，行为完全一致）。启用 v2 lint 后必须修复此点才能满足「lint 全通过」验收。

## 4. 范围

### In scope（#32）
1. **CI**：重新启用 `ci.yml` 中被注释的 golangci-lint 步骤，版本固定 `v2.12.2`；从仓库根目录按 matrix module 运行（与本地/pre-commit 一致，确保能发现根目录 `.golangci.yml`）。
2. **修复唯一违规**：`go-auth/jwt/token.go` `reflect.Ptr` → `reflect.Pointer`。
3. **文档**：在 `README.md`（Requirements）与 `.claude/rules/go.md`（§8 静态分析）注明所需 golangci-lint 版本（v2，≥ v2.12.2）及本地升级方式。
4. **验证**：逐 module 运行 v2 lint 全绿（含 go-auth，共 4 个 module，对应验收命令）。

### Out of scope（标注为后续，不在本次改动）
- **本地二进制升级**：不自动覆盖用户 `~/go/bin/golangci-lint`（遵守「未经许可不安装/变更外部状态」规则）；仅在文档给出升级命令，由开发者自行执行。
- **pre-commit / CLAUDE.md 的 module 循环缺 go-auth**：现有本地循环只列 go-common/go-middleware/go-framework（漏 go-auth）。CI matrix 已覆盖 go-auth，故不影响 CI 把关；该本地覆盖缺口属独立问题，建议另开 issue。

## 5. 方案概要（实现层面）

- `ci.yml`：取消注释并改写 golangci-lint 步骤——使用 `golangci/golangci-lint-action`，`version: v2.12.2`，去掉 `working-directory`（在仓库根运行），`args: --timeout=5m ./${{ matrix.module }}/...`。
- `token.go:128`：`reflect.Ptr` → `reflect.Pointer`。
- `README.md` Requirements 段：补充 `golangci-lint v2 (>= v2.12.2)` 及安装/升级命令。
- `.claude/rules/go.md` §8：注明所需版本与升级命令。

## 6. 风险

- **低**。唯一代码改动是废弃别名替换（值不变、行为不变）；CI 改为启用既有意图的步骤；配置不变（已是 v2）。
- 若 CI runner 上 `golangci-lint-action` 主版本有更新，执行阶段确认其当前稳定主版本并使用。
