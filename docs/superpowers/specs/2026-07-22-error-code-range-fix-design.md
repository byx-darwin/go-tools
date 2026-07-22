# 错误码段边界修正设计（Issue #28）

> **工作流**: wf-2026-07-22-001（fast mode）· **跟踪**: #34（多角色审计 roadmap · Phase 2）
> **日期**: 2026-07-22 · **前置**: #27（错误码归属迁移，PR #48 已合并）
> **范围**: 仅 #28（PR: `Closes #28, Refs #34`）

## 1. 背景

#27 完成了错误码归属迁移，但按设计「搬常量不改值」，`go-common/error` 的 `ProjectCodeMin` 仍为
**40000**，与文档约定（`CLAUDE.md`/`specs`：go-auth `40000-40099`、project `40100-59999`）不一致，
且缺少显式的 auth 段边界常量。#28 负责把常量对齐到文档，并补上 `AuthCodeMin/Max`。

**#27 已提前化解的部分**：原 #28 AC 中的「去重 token 码」已随 #27 删除 go-common 的
`CodeTokenInvalid=40112` / `CodeTokenExpired=40111`（及全部业务码 40010–40314）而完成；
token 错误域现专属 `go-auth/error`（`40001`/`40002`）。本设计不再涉及。

## 2. 事实发现（决策依据）

| # | 事实 | 证据 |
|---|------|------|
| 1 | `ProjectCodeMin = 40000`，文档约定 project 段为 `40100-59999` | `go-common/error/error.go:44` vs `CLAUDE.md` |
| 2 | go-common 无 `AuthCodeMin/Max`；auth 段 `40000-40099` 无显式边界常量 | `go-common/error/error.go` 常量块 |
| 3 | `httpStatusForCode` 兜底用 `case code >= ProjectCodeMin: return 200` | `error.go:113` |
| 4 | `IsBusinessErrorCode` 用 `code >= ProjectCodeMin \|\| (code < FrameworkCodeMin && code > 0)` | `error.go:135` |
| 5 | **现有测试已锁定 auth 段（40000–40099）按业务码处理**：`TestHTTPStatus_Fallback`（`Code(40001)`→200，走兜底）、`TestIsBusinessErrorCode`（`IsBusinessErrorCode(40001)==true`、`(40010)==true`）、`TestIsClientError`/`TestIsServerError`（40001 非 4xx/5xx） | `go-common/error/error_test.go` |
| 6 | go-auth 的错误码均经 `init()` 注册到 HTTP 注册表（40001–40009 → 200），但**兜底路径**仍依赖 `ProjectCodeMin` 边界 | `go-auth/error/error.go` |

## 3. 决策

**将业务码段下限从 `ProjectCodeMin`(40000) 解耦为 `AuthCodeMin`(40000)，`ProjectCodeMin` 修正为 40100。**

### 3.1 常量变更（`go-common/error/error.go`）

```go
const (
	FrameworkCodeMin  = 10000
	FrameworkCodeMax  = 10499
	MiddlewareCodeMin = 20000
	MiddlewareCodeMax = 20699
	AuthCodeMin       = 40000 // go-auth 最小错误码（业务码段下限）
	AuthCodeMax       = 40099 // go-auth 最大错误码
	ProjectCodeMin    = 40100 // 项目自定义最小错误码（原 40000 → 40100）
	ProjectCodeMax    = 59999
)
```

### 3.2 边界解耦（关键，避免回归）

`httpStatusForCode` 兜底与 `IsBusinessErrorCode` 的「业务码 → 200」边界，从 `ProjectCodeMin`
切换到 **`AuthCodeMin`**（业务码段真实下限 40000）：

```go
// httpStatusForCode 兜底
case code >= AuthCodeMin:   // 原 ProjectCodeMin
	return 200

// IsBusinessErrorCode
return code >= AuthCodeMin || (code < FrameworkCodeMin && code > 0)
```

**理由**：auth 段（40000–40099）与 project 段（40100–59999）共同构成「业务码 → HTTP 200」区间。
业务码段下限是 `AuthCodeMin`(40000)，不是 `ProjectCodeMin`(40100)。若仅改 `ProjectCodeMin` 而不解耦，
40000–40099 会跌出业务码判定 → 事实 5 的测试全部失败（行为回归）。解耦后行为**逐值不变**。

### 3.3 godoc 同步

包注释 `（业务码 ≥ ProjectCodeMin → 200…）` 改为 `（业务码 ≥ AuthCodeMin → 200…）`。

## 4. 行为兼容矩阵（迁移前后逐值一致）

| 错误码 | 迁移前 | 迁移后 | 说明 |
|--------|--------|--------|------|
| 40001（auth，注册）| 200 | 200 | 走注册表，不变 |
| 40001（auth，兜底）| 200 | 200 | `>= AuthCodeMin(40000)`，不变 |
| 40050（auth 段未分配，兜底）| 200 | 200 | `>= AuthCodeMin`，不变 |
| 40100（project，兜底）| 200 | 200 | `>= AuthCodeMin`，不变 |
| 20999（middleware 未注册）| 500 | 500 | `>0` 且 `<AuthCodeMin`，不变 |
| 10000（framework）| 500/注册值 | 同 | 不变 |
| `IsBusinessErrorCode(40001)` | true | true | `>= AuthCodeMin` |
| `IsBusinessErrorCode(40010)` | true | true | `>= AuthCodeMin` |

## 5. 迁移清单

| 文件 | 改动 |
|------|------|
| `go-common/error/error.go` | `ProjectCodeMin` 40000→40100；新增 `AuthCodeMin=40000`/`AuthCodeMax=40099`；`httpStatusForCode` 兜底 + `IsBusinessErrorCode` 边界改用 `AuthCodeMin`；包 godoc 同步 |
| `go-common/error/error_test.go` | `TestCodeConstants` 增加 `MiddlewareCodeMax < AuthCodeMin`、`AuthCodeMax < ProjectCodeMin` 断言；新增 auth 边界值测试（`IsBusinessErrorCode(40000/40099/40100)`、`HTTPStatus` 兜底 40050→200）；现有行为测试保持绿 |
| `CLAUDE.md` | Error Code Ranges 已为目标值（go-auth 40000-40099 / project 40100-59999），补注 `AuthCodeMin/Max` 显式边界 |
| `specs/00_overview.md` §四 | 补注 `AuthCodeMin/Max=40000/40099`、`ProjectCodeMin=40100` 常量与业务码段下限说明 |

## 6. 测试策略

- **TDD**：先改 `TestCodeConstants` + 新增 auth 边界测试（RED：`AuthCodeMin` 未定义编译失败），再实现（GREEN）。
- **回归锁定**：现有 `TestHTTPStatus_Fallback` / `TestIsBusinessErrorCode` / `TestIsClientError` /
  `TestIsServerError` 全绿（auth 段行为不变）。
- **最终验证**：四模块 `go build` + `go vet` + `golangci-lint` v2 逐模块 + `go test -count=1` 全量。

## 7. 范围边界与非目标

**本 PR 做**：`ProjectCodeMin` 40000→40100；新增 `AuthCodeMin/Max`；边界解耦（`httpStatusForCode`/`IsBusinessErrorCode`）；测试 + 文档。

**明确不做：**

| 事项 | 归属 |
|------|------|
| 去重 token 码（40111/40112）| 已由 #27 完成 |
| 改任何已分配码值 | 禁止（wire 稳定）|
| 新增 auth/project 具体错误码 | 禁止（本 PR 只动边界常量）|
| go-auth 段码值调整 | 不变（40001–40009 维持）|

## 8. 风险与缓解

| 风险 | 缓解 |
|------|------|
| 边界解耦遗漏导致 auth 段回归 | 现有测试已锁定 40001/40010 行为；TDD 先红后绿；行为兼容矩阵 §4 逐值核对 |
| ncgo/下游依赖 `ProjectCodeMin==40000` | 仓库无 tag/release，无外部消费者；`ProjectCodeMin` 改为 40100 与文档一致，反而消除歧义 |
| `IsBusinessErrorCode` 语义变化 | 解耦后对 ≥40000 的码逐值不变；仅「常量名」变化，行为不变 |

## 9. 验收对照（Issue #28 AC）

- [x] 去重 token 码（由 #27 完成）
- [ ] `ProjectCodeMin` → 40100
- [ ] 新增 `AuthCodeMin/Max=40000/40099`
- [ ] （新增）`httpStatusForCode`/`IsBusinessErrorCode` 边界切换到 `AuthCodeMin`，auth 段行为不变
- [ ] 更新 specs/CLAUDE.md
- [ ] 受影响模块测试通过
