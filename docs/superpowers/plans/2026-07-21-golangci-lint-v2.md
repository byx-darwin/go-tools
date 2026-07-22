# Plan: 对齐 golangci-lint 到 v2（Issue #32）

- **日期**: 2026-07-21 · **模式**: fast · **设计文档**: `docs/superpowers/specs/2026-07-21-golangci-lint-v2-design.md`
- **分支**: `feat/32-golangci-lint-v2`（worktree）

## 目标
让项目 v2 格式的 `.golangci.yml` 在本地与 CI 真正生效：统一使用 golangci-lint **v2（≥ v2.12.2）**，重新启用 CI lint 步骤，修复启用后暴露的唯一违规，并在文档注明所需版本。

## 基线证据（已实测，v2.12.2）
- `config verify`：通过
- go-common / go-middleware / go-framework：0 issues
- **go-auth：1 issue** — `go-auth/jwt/token.go:128` govet(inline) `reflect.Ptr should be inlined`（RED）

## 执行步骤（TDD：RED → GREEN → REFACTOR）

1. **Worktree** — 创建并进入 `feat/32-golangci-lint-v2`。
2. **RED 复现** — 用 v2.12.2（临时 GOBIN，不动用户本地二进制）跑 `./go-auth/...`，确认 1 issue。
3. **GREEN 修复** — `go-auth/jwt/token.go:128`：`reflect.Ptr` → `reflect.Pointer`（同一常量值，行为不变）。
4. **GREEN 验证** — v2.12.2 逐 module（go-common/go-auth/go-middleware/go-framework）运行 → 全部 0 issues。
5. **CI** — 改写 `.github/workflows/ci.yml`：取消注释并更新 golangci-lint 步骤
   - `uses: golangci/golangci-lint-action@v9`
   - `with: { version: v2.12.2, args: --timeout=5m ./${{ matrix.module }}/... }`
   - 去掉 `working-directory`（在仓库根运行，确保发现根目录 `.golangci.yml`）；删除「暂不可用」过时注释。
6. **文档** —
   - `README.md` → `## Requirements`：补充 `golangci-lint v2 (>= v2.12.2)` 及安装/升级命令。
   - `.claude/rules/go.md` §8：注明所需版本与本地升级命令（`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`）。
7. **全量校验** — 逐 module：`go build` / `go vet` / `go test -count=1` + v2 lint 全绿。
8. **提交 & PR** — Conventional Commit；`gitflow-pr-create`，PR body 含 `Closes #32`。

## 明确不做（Out of scope，见设计文档 §4）
- 不自动升级用户本地 `~/go/bin/golangci-lint`（仅在文档给命令，由开发者自行执行）。
- 不改动 pre-commit / CLAUDE.md 中「本地循环漏 go-auth」的问题（CI matrix 已覆盖 go-auth；该缺口建议另开 issue）。

## 风险
低。唯一代码改动为废弃别名替换（值不变）；CI 为启用既有意图步骤；`.golangci.yml` 不变。

## 验收对照（#32 AC）
- [x→] 二选一：选「升级 v2」
- [ ] CI/pre-commit 使用正确版本（CI 启用 v2.12.2；pre-commit 沿用系统二进制，文档注明需 v2）
- [ ] `for m in go-common go-auth go-middleware go-framework; do golangci-lint run --timeout=5m ./$m/...; done` 全通过
- [ ] README/rules 注明所需版本
