# grpc 漏洞修复设计（GO-2026-6348）

**Issue:** #114
**分类:** bounded（有界变更，无需完整架构设计）
**状态:** 已批准

## 背景

PR #112 交付后 Phase 4 pipeline 分析发现 `govulncheck (go-framework)` CI job 失败，报告见
`reports/pipeline/pr-112-2026-09-18.md`。确认与 PR #112 本身无关，是仓库级已有依赖暴露。

## 漏洞

`GO-2026-6348` — HTTP/2 DATA Frame Fragmentation 导致的堆内存耗尽（OOM），存在于
`google.golang.org/grpc@v1.82.1`，修复版本 `v1.83.1`。

## 探查结论

1. `go-framework/go.mod` 直接 `require google.golang.org/grpc v1.82.1`。已用 `govulncheck` 复现，
   命中 15 条调用链，均经由 `hertz/observability/provider.go:133`（OTel metric exporter）与
   `go-common` 的 `oops` error builder。
2. `go-middleware` 间接引入 `google.golang.org/grpc v1.40.0`（未直接 require）。已用
   `GOWORK=off govulncheck ./...` 验证：**不受影响**——代码路径未调用到漏洞函数
   （govulncheck 报告归类为 "vulnerability in modules you require, but your code doesn't
   appear to call these vulnerabilities"）。
3. 因此本次改动范围收窄为**仅 go-framework**。

## 方案

- 在 `go-framework/go.mod` 执行 `go get google.golang.org/grpc@v1.83.1`（或更高兼容版本），
  随后 `go mod tidy`
- 验证：`go build ./go-framework/...`、`go test ./go-framework/... -count=1`、
  `GOWORK=off govulncheck ./...`（预期 0 vulnerabilities affecting code）
- 若升级引入破坏性 API 变更，据实测结果调整调用方代码
- 视情况为 go-framework 打新 patch tag 并发布（D4 独立版本发布）

## 不做的事

- 不改动 `go-middleware`（已验证不受影响）
- 不做无关重构

## 验收标准

- [ ] `go-framework` 的 `google.golang.org/grpc` 升级到 `>= v1.83.1`，`govulncheck ./...` 通过
- [ ] 确认 `go-middleware` 不受影响（已在设计阶段验证，记录结论）
- [ ] `go build` + `go test` 通过
- [ ] 视情况打新 tag 并发布
