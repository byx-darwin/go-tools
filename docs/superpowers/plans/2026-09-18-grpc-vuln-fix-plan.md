# grpc 漏洞修复（GO-2026-6348）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `go-framework` 模块直接依赖的 `google.golang.org/grpc` 从 `v1.82.1` 升级到 `>= v1.83.1`，消除 govulncheck 报告的 GO-2026-6348 堆内存耗尽漏洞，且不影响 go-middleware（已确认不受影响，不在本计划范围内）。

**Architecture:** 单一依赖版本升级，无代码架构变更。`go-framework` 直接 `require` 了 grpc，仅通过 `google.golang.org/grpc/credentials` 包（`hertz/observability/provider.go` 与 `kitex/observability/provider.go` 中用于 OTel gRPC exporter 的 TLS 配置）间接触达漏洞路径，未直接调用 grpc 核心 API，因此升级预期是纯版本 bump，不涉及调用方代码改动（若 govulncheck / build / test 验证发现破坏性 API 变更，见 Task 2 的分支处理）。

**Tech Stack:** Go 1.26.5 workspace（`go.work`），`go-framework` 独立 `go.mod`，govulncheck 静态分析。

**Spec:** `docs/superpowers/specs/2026-09-18-grpc-vuln-fix-design.md`

## Global Constraints

- 仅修改 `go-framework/go.mod` 和 `go-framework/go.sum`；不修改 `go-middleware`（已在设计阶段用 govulncheck 验证不受影响）
- 目标版本：`google.golang.org/grpc >= v1.83.1`
- 验证命令固定为：`go build ./go-framework/...`、`go test ./go-framework/... -count=1`、`GOWORK=off govulncheck ./...`（在 `go-framework/` 目录下运行，因为 govulncheck 不支持 workspace 根目录 `./...`）
- 不做无关重构，不改动 go-middleware/go-auth/go-common

---

### Task 1: 升级 go-framework 的 grpc 依赖

**Files:**
- Modify: `go-framework/go.mod`
- Modify: `go-framework/go.sum`

**Interfaces:**
- Consumes: 无（纯依赖版本变更）
- Produces: `google.golang.org/grpc` 在 `go-framework/go.mod` 中的 require 版本 `>= v1.83.1`，供 Task 2 验证使用

- [ ] **Step 1: 记录升级前基线（用于对照）**

Run:
```bash
cd go-framework && GOWORK=off govulncheck ./... 2>&1 | tee /tmp/govulncheck-before.txt
```
Expected: 输出包含 `Vulnerability #1: GO-2026-6348`，`Found in: google.golang.org/grpc@v1.82.1`

- [ ] **Step 2: 执行依赖升级**

Run:
```bash
cd go-framework && go get google.golang.org/grpc@v1.83.1
```
Expected: 命令成功退出（exit 0），`go.mod` 中 `google.golang.org/grpc` 行版本变为 `v1.83.1`（或 go get 解析到的更高兼容 patch/minor 版本，只要满足 `>= v1.83.1`）

- [ ] **Step 3: 整理依赖图**

Run:
```bash
cd go-framework && go mod tidy
```
Expected: 命令成功退出，`go.mod`/`go.sum` 更新完成，无 `missing go.sum entry` 类错误

- [ ] **Step 4: 校验 go.mod 中版本号**

Run:
```bash
grep "google.golang.org/grpc " go-framework/go.mod
```
Expected: 输出形如 `google.golang.org/grpc v1.83.1`（或更高版本），无 `// indirect` 后缀（保持直接依赖声明）

- [ ] **Step 5: Commit**

```bash
git add go-framework/go.mod go-framework/go.sum
git commit -m "fix(go-framework): bump google.golang.org/grpc to v1.83.1 to fix GO-2026-6348"
```

---

### Task 2: 验证构建、测试与漏洞扫描

**Files:**
- Test: `go-framework/...`（全模块，无新增文件）

**Interfaces:**
- Consumes: Task 1 产出的 `go-framework/go.mod` 中 `google.golang.org/grpc >= v1.83.1`
- Produces: 验证通过的构建产物与 govulncheck 报告（0 vulnerabilities affecting code），供 Phase 3 Gate 3→4 的 `tests_passed` 证据使用

- [ ] **Step 1: 构建验证**

Run:
```bash
go build ./go-framework/...
```
Expected: 命令成功退出（exit 0），无编译错误

- [ ] **Step 2: 单元测试验证**

Run:
```bash
go test ./go-framework/... -count=1
```
Expected: 命令成功退出（exit 0），所有测试 PASS，特别关注 `go-framework/hertz/observability/provider_test.go`、`go-framework/kitex/observability/provider_test.go`、`go-framework/kitex/observability/peer_test.go`（这三个文件涉及使用 `google.golang.org/grpc/credentials` 的 provider 代码路径）

- [ ] **Step 3: govulncheck 复验**

Run:
```bash
cd go-framework && GOWORK=off govulncheck ./... 2>&1 | tee /tmp/govulncheck-after.txt
```
Expected: 输出中**不再包含** `GO-2026-6348`；理想情况下输出 `No vulnerabilities found.` 或该 CVE 从 "Your code is affected by" 列表中消失

- [ ] **Step 4: 对照升级前后结果**

Run:
```bash
diff /tmp/govulncheck-before.txt /tmp/govulncheck-after.txt || true
```
Expected: 人工确认 diff 显示 GO-2026-6348 从"受影响"变为"不受影响"或"未出现"；若 govulncheck 仍报告该 CVE，说明升级版本未达到修复线或存在其他调用路径，需回到 Task 1 Step 2 提升版本或排查（不在本计划自动化范围内，出现此情况需暂停并向用户报告）

- [ ] **Step 5: Lint 验证（不修改代码，仅确认升级未引入格式/静态分析问题）**

Run:
```bash
gofmt -l go-framework/go.mod 2>&1 || true
go vet ./go-framework/...
```
Expected: `go vet` 无输出、退出码 0（`gofmt -l` 对 `.mod` 文件无意义，此步骤主要确认 `go vet` 通过）

- [ ] **Step 6: 清理临时文件**

Run:
```bash
rm -f /tmp/govulncheck-before.txt /tmp/govulncheck-after.txt
```

- [ ] **Step 7: Commit（若 Task 1 未单独提交，或本任务产生任何文件变更）**

本任务不产生源码变更（仅验证），若所有步骤通过且无文件差异，无需额外 commit；Task 1 的 commit 已覆盖依赖变更。

---

## Post-Plan Note（打 tag 决策，非本计划任务）

设计文档中的"视情况打新 tag 并发布"取决于 Task 2 验证结果：
- 若升级为纯 patch 版本 bump，无 API 破坏性变更 → 建议按 D4（独立版本发布）为 `go-framework` 打新 patch tag（如 `go-framework/v0.x.(y+1)`）
- 若发现破坏性变更（Task 2 Step 2/3 失败且需要代码改动）→ 需要额外任务修复调用方代码后再评估是否需要 minor 版本号

此决策点在 Phase 3 TDD 循环完成后、Phase 4 交付前由用户确认，不作为独立 Task 编号（避免计划中出现无法预先确定步骤的占位任务）。
