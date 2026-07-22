# 错误码段边界修正（#28）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:subagent-driven-development. Steps use checkbox (`- [ ]`) syntax.

**Goal:** 将 `go-common/error` 的 `ProjectCodeMin` 从 40000 修正为 40100，新增显式 `AuthCodeMin/Max=40000/40099`，并把「业务码 → HTTP 200」的边界从 `ProjectCodeMin` 解耦到 `AuthCodeMin`，使常量与文档一致且 auth 段（40000–40099）行为逐值不变。

**Architecture:** 纯常量 + 边界解耦，不新增任何错误码、不改任何已分配码值（wire 稳定）。设计依据见 `docs/superpowers/specs/2026-07-22-error-code-range-fix-design.md`。

**Tech Stack:** Go 1.25+（workspace go.work）、testify、golangci-lint v2。

## Global Constraints

- **码值不可变更**：仅改 `ProjectCodeMin` 常量值（40000→40100）与新增边界常量；任何已分配错误码值不变。
- **行为逐值不变**：auth 段 40000–40099 的 `HTTPStatus`/`IsBusinessErrorCode`/`IsClientError`/`IsServerError` 结果与迁移前完全一致（靠边界切换到 `AuthCodeMin` 保证）。
- **lint 逐模块运行**：`for m in go-common go-auth go-middleware go-framework; do golangci-lint run --timeout=5m ./$m/... || exit 1; done`（v2 ≥ v2.12.2）。
- **revive**：新导出常量必须有 `// Name ...` godoc。
- **提交前缀**：`fix(go-common)` / `docs`；每条 commit 尾部带 `(#28)`。
- **TDD**：先写失败测试，再实现；每任务独立 commit。
- **PR**：`Closes #28, Refs #34`；设计文档随分支提交。

## File Structure

| 文件 | 职责 | 动作 |
|------|------|------|
| `go-common/error/error.go` | 常量块 + `httpStatusForCode` + `IsBusinessErrorCode` + 包 godoc | 改（Task 2）|
| `go-common/error/error_test.go` | 边界关系 + auth 边界值测试 | 改（Task 2）|
| `CLAUDE.md` | Error Code Ranges 补注 `AuthCodeMin/Max` | 改（Task 3）|
| `specs/00_overview.md` §四 | 补注边界常量说明 | 改（Task 3）|

---

### Task 1: 准备分支 + 提交设计文档与计划

**Files:**
- Commit（docs）: `docs/superpowers/specs/2026-07-22-error-code-range-fix-design.md`、`docs/superpowers/plans/2026-07-22-error-code-range-fix.md`

- [ ] **Step 1: 创建 worktree 分支 `feat/28-error-code-range-fix`**

由编排器创建。将主仓库工作区未跟踪的两个文档复制进 worktree 并提交：

```bash
mkdir -p docs/superpowers/specs docs/superpowers/plans
cp <MAIN>/docs/superpowers/specs/2026-07-22-error-code-range-fix-design.md docs/superpowers/specs/
cp <MAIN>/docs/superpowers/plans/2026-07-22-error-code-range-fix.md docs/superpowers/plans/
git add docs/superpowers/specs/2026-07-22-error-code-range-fix-design.md docs/superpowers/plans/2026-07-22-error-code-range-fix.md
git commit -m "docs: add error-code range fix design & implementation plan (#28)"
```

---

### Task 2: go-common/error 常量 + 边界解耦（TDD）

**Files:**
- Modify: `go-common/error/error.go`
- Modify: `go-common/error/error_test.go`

- [ ] **Step 1: 写失败测试（`error_test.go`）**

更新 `TestCodeConstants` 并新增 auth 边界测试：

```go
func TestCodeConstants(t *testing.T) {
	assert.Less(t, FrameworkCodeMax, MiddlewareCodeMin)
	assert.Less(t, MiddlewareCodeMax, AuthCodeMin)
	assert.Less(t, AuthCodeMax, ProjectCodeMin)
}

// auth 段（40000-40099）是业务码段下限，行为须与迁移前逐值一致。
func TestAuthBandBoundary(t *testing.T) {
	assert.Equal(t, 40000, AuthCodeMin)
	assert.Equal(t, 40099, AuthCodeMax)
	assert.Equal(t, 40100, ProjectCodeMin)

	// 业务码判定：auth 段 + project 段均为业务码
	assert.True(t, IsBusinessErrorCode(AuthCodeMin))   // 40000
	assert.True(t, IsBusinessErrorCode(AuthCodeMax))   // 40099
	assert.True(t, IsBusinessErrorCode(ProjectCodeMin)) // 40100

	// HTTP 兜底：auth 段未注册码 → 200
	assert.Equal(t, 200, HTTPStatus(Code(40050).Public("auth_band").Wrap(errors.New("x"))))
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-common/error/ -run 'TestCodeConstants|TestAuthBandBoundary' -count=1`
Expected: FAIL — `undefined: AuthCodeMin`

- [ ] **Step 3: 实现常量块（`error.go`）**

将常量块替换为：

```go
// 错误码范围边界常量。
const (
	FrameworkCodeMin  = 10000 // go-framework 最小错误码
	FrameworkCodeMax  = 10499 // go-framework 最大错误码
	MiddlewareCodeMin = 20000 // go-middleware 最小错误码
	MiddlewareCodeMax = 20699 // go-middleware 最大错误码
	AuthCodeMin       = 40000 // go-auth 最小错误码（业务码段下限）
	AuthCodeMax       = 40099 // go-auth 最大错误码
	ProjectCodeMin    = 40100 // 项目自定义最小错误码
	ProjectCodeMax    = 59999 // 项目自定义最大错误码
)
```

- [ ] **Step 4: 边界解耦（`error.go`）**

`httpStatusForCode` 兜底：

```go
	switch {
	case code >= AuthCodeMin:   // 业务错误（auth 段 + project 段，RPC 成功，HTTP 200）
		return 200
	case code > 0:
		return 500 // 未注册的框架/基础设施错误
	default:
		return 200 // 非 oops 错误 / 无错误码
	}
```

`IsBusinessErrorCode`：

```go
func IsBusinessErrorCode(code int) bool {
	return code >= AuthCodeMin || (code < FrameworkCodeMin && code > 0)
}
```

- [ ] **Step 5: 同步包 godoc**

将包注释第 9 行：

```
//     （业务码 ≥ ProjectCodeMin → 200；其余 >0 → 500；非 oops → 200）
```

改为：

```
//     （业务码 ≥ AuthCodeMin → 200；其余 >0 → 500；非 oops → 200）
```

- [ ] **Step 6: 运行全部 go-common/error 测试**

Run: `go test ./go-common/error/ -count=1`
Expected: PASS（新测试 + 现有 `TestHTTPStatus_Fallback`/`TestIsBusinessErrorCode`/`TestIsClientError`/`TestIsServerError` 全绿——auth 段行为逐值不变）

- [ ] **Step 7: 提交**

```bash
gofmt -w go-common/error/error.go go-common/error/error_test.go
git add go-common/error/error.go go-common/error/error_test.go
git commit -m "fix(go-common): align ProjectCodeMin to 40100 and add AuthCode band bounds (#28)"
```

---

### Task 3: 文档更新

**Files:**
- Modify: `CLAUDE.md`
- Modify: `specs/00_overview.md`

- [ ] **Step 1: 改 `CLAUDE.md` Error Code Ranges**

`go-auth` 行补注显式边界：

```
go-auth:       40000-40099 (token, session, device auth errors; defined in go-auth/error; AuthCodeMin/Max)
Project custom: 40100-59999 (business modules, external dependencies; no library predefinitions; ProjectCodeMin)
```

- [ ] **Step 2: 改 `specs/00_overview.md` §四**

在 go-auth 段与项目业务段说明处补注边界常量：

```
go-auth/error (autherror)  40001-40009  ── token/session/device/JWT → HTTP 200（AuthCodeMin=40000 / AuthCodeMax=40099）
...
项目业务       40100-59999  ── HTTP 200（ProjectCodeMin=40100；业务码段下限为 AuthCodeMin=40000）
```

并在「HTTP 状态映射机制」段把「业务码 → 200」的范围说明为「≥ AuthCodeMin（40000）」。

- [ ] **Step 3: 提交**

```bash
git add CLAUDE.md specs/00_overview.md
git commit -m "docs: note AuthCodeMin/Max and ProjectCodeMin=40100 bounds (#28)"
```

---

### Task 4: 最终全量验证

- [ ] **Step 1: 全模块构建** — `go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`
- [ ] **Step 2: go vet** — `go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`
- [ ] **Step 3: gofmt 检查** — `gofmt -l $(find . -name '*.go' -not -path '*/vendor/*' -not -path './.git/*')`（应无输出）
- [ ] **Step 4: golangci-lint 逐模块** — `for m in go-common go-auth go-middleware go-framework; do golangci-lint run --timeout=5m ./$m/... || exit 1; done`
- [ ] **Step 5: 全量测试** — `go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1`
- [ ] **Step 6: 边界值终检** — `grep -rn "ProjectCodeMin\s*=\s*40000\|code >= ProjectCodeMin" --include='*.go' go-common/`（应无输出：`ProjectCodeMin` 不再等于 40000，且兜底/判定不再用 `ProjectCodeMin`）
- [ ] **Step 7: 若失败** — 最小修复，独立 commit `fix: <修复> (#28)`，重跑失败项。
