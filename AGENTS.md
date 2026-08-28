# AGENTS.md — go-tools

## Project Overview

**go-tools** is a Go workspace (`go.work`) containing four independently versioned libraries for Hertz (HTTP) and Kitex (RPC) microservice development. It serves as the foundation for [ncgo](https://github.com/byx-darwin/ncgo) scaffold-generated projects.

### Structure

```text
                    go-common    ← 最底层，零框架依赖
                        ↑          (crypto, cache, httpclient, log, timeutil, netutil, captcha, error)
                     go-auth       ← 认证工具 (JWT, Session/Device 接口, 错误码)
                    ↑       ↑
          ┌─────────┘       └─────────┐
    go-middleware                  go-framework
    中间件客户端                    框架适配 (hertz, kitex, config)
    (redis, kafka, db,
     es, clickhouse, tls, auth)
```

> 真实拓扑是 **DAG，非线性链**：`go-framework` 与 `go-middleware` 为**兄弟关系**（sibling），二者均依赖 `go-auth` + `go-common`，彼此**无依赖**。

| Module | Import Path | Purpose |
|--------|------------|---------|
| `go-common` | `github.com/byx-darwin/go-tools/go-common` | Pure utilities: crypto, cache, log, error, timeutil, netutil, httpclient, captcha |
| `go-auth` | `github.com/byx-darwin/go-tools/go-auth` | Auth utilities: JWT Sign/Verify/Refresh, Session/Device interfaces |
| `go-middleware` | `github.com/byx-darwin/go-tools/go-middleware` | Middleware clients: redis, kafka, db, es, clickhouse, tls, auth |
| `go-framework` | `github.com/byx-darwin/go-tools/go-framework` | Framework adapters: hertz, kitex, config |

### Error Code Ranges

```
go-framework: 10000-10499  (system, param, auth, config, RPC)
go-middleware: 20000-20699 (clickhouse 20401-20403, tls 20501-20504)
go-auth:       40000-40099 (token, session, device auth errors)
Project custom: 40100-59999 (business modules, no library predefinitions)
```

## Development Commands

```bash
# Build all modules
go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...

# Test all modules
go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1

# Test a specific module
go test ./go-common/... -count=1

# Lint (golangci-lint v2, 必须逐 module 运行)
for m in go-common go-auth go-middleware go-framework; do
  golangci-lint run --timeout=5m ./$m/...
done

# Full validation
go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... && \
  go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... && \
  for m in go-common go-auth go-middleware go-framework; do golangci-lint run --timeout=5m ./$m/... || exit 1; done && \
  go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1
```

**Prerequisites:** Go 1.25+ (workspace mode via `go.work`), golangci-lint v2 (>= v2.12.2).

## Architecture Rules

### Module Boundaries

- Each sub-module has its own `go.mod`. Cross-module imports use `github.com/byx-darwin/go-tools/<module>` paths.
- Do not create circular dependencies between modules.
- **go-common**: zero framework dependency, pure utility — does not import any other library
- **go-auth**: auth utilities — may import go-common only
- **go-middleware**: middleware clients — may import go-common and go-auth, must NOT import go-framework
- **go-framework**: Hertz/Kitex adapters — may import go-common and go-auth, must NOT import go-middleware (siblings)

### Code Placement

- Pure utilities (crypto, cache, time, net) → go-common
- Middleware clients (redis, kafka, db, es) → go-middleware
- Framework adapters (hertz, kitex, config, observability) → go-framework

### Key Decisions

| Decision | Conclusion |
|----------|-----------|
| Kafka library | **kafka-go** (not sarama) |
| Config time units | **time.Duration** (YAML: `30s` format) |
| Error library | **oops** as primary |
| Release strategy | Independent versioning per module |
| Cache | **github.com/samber/hot** wrapper |

## Go Coding Rules

### General Style

- Follow standard Go style, keep files `gofmt`-clean.
- Prefer small, focused functions. Prefer explicit, readable code over clever abstractions.
- Reuse existing helpers before adding new utility layers.
- Preserve stable public API contracts unless the task explicitly changes them.

### Functional Options Pattern

MUST use Options pattern when:
- Constructor has **3+ parameters**
- Config struct has **5+ fields**
- There's `ApplyDefaults` / default value logic
- Parameters may expand in the future

```go
// Option 定义配置选项函数。
type Option func(*Target)

// WithExpiration 设置过期时间。
func WithExpiration(expiration time.Duration) Option {
    return func(c *Target) {
        if expiration > 0 {
            c.expiration = expiration
        }
    }
}

// NewTarget 创建实例，支持 Options 配置。
func NewTarget(opts ...Option) *Target {
    t := &Target{
        expiration: defaultExpiration,
        preKey:     defaultPreKey,
    }
    for _, opt := range opts {
        opt(t)
    }
    t.internal = buildSomething(t.capacity)
    return t
}
```

Rules:
- Function name starts with `With`, sets one field each
- Defend against invalid input (`> 0`, `!= ""`)
- Constructor: fill defaults first, then apply opts, then dependent init
- Godoc comments required on all exported symbols

Forbidden:
- Constructors with 3+ positional params (excluding `opts ...Option`)
- Exporting struct fields for direct assignment
- Bool switch params instead of Options
- Multiple `NewXxxWithYyy` variant functions

### Static Analysis (golangci-lint v2)

Enabled linters and requirements:

| Linter | Requirement |
|--------|------------|
| `gofmt` | All files must be formatted |
| `goimports` | Three import groups: stdlib / third-party / project |
| `revive` | All exported symbols need `// Name ...` godoc comments |
| `errcheck` | All returned errors must be handled |
| `gocritic` | Code style rules (see below) |
| `misspell` | American spelling |
| `unconvert` | No unnecessary type conversions |
| `unparam` | Replace unused params with `_` |

gocritic specifics:
- Octal literals: `0o644` not `0644`
- Combine same-type params: `func(a, b int)`
- Don't shadow builtins (`max`, `min`, `copy`)
- No `defer` before `return`
- `//nolint:xxx` must have explanation: `//nolint:xxx // reason`

Type rules:
- Use `any` instead of `interface{}` (Go 1.18+)

Error handling:
```go
defer func() { _ = f.Close() }()          // explicit ignore in defer
require.NoError(t, err)                    // in tests
//nolint:gosec // explanation required      // in production
```

### Error Handling

- Return clear errors; do not swallow silently.
- Wrap errors with context when crossing package/module boundaries.
- Use `oops` as the primary error library.
- Prefer early returns over deep nesting.

### Tests

- `*_test.go` alongside source code.
- Add/update unit tests when logic branches change.
- Run per-module tests when working on a specific module.
- Cross-module changes MUST verify full workspace builds and tests pass.

## Agent Execution Rules

### Core Principles

- Make small, correct, explainable changes.
- MUST read relevant implementation and nearby tests before editing.
- MUST keep diffs minimal, avoid unrelated cleanup.
- MUST validate changes with smallest useful checks first.
- MUST update tests when behavior changes.
- MUST NOT install dependencies, deploy, or run destructive operations without explicit permission.

### Validation Order

Run from smallest/fastest to broadest/slowest:
1. Single test function or focused unit tests
2. Relevant test file
3. Relevant package tests
4. Related integration tests
5. Full workspace checks (only for PR-quality validation)

### Failure Handling

- Smallest plausible fix, then rerun the most relevant check.
- Do not stack multiple speculative fixes before rerunning.
- If repeated attempts don't improve, stop and ask.
- MUST NOT bypass failing validation silently.

### Stop Conditions

- Stop when the task is complete and validation passed.
- Do not continue into adjacent improvements without asking.
- Suggest next steps rather than doing them automatically.

## Claude Code Skills Reference

This project also has Claude Code skills in `.claude/skills/` (gitflow series). When asked to execute a gitflow skill, read the corresponding `.claude/skills/<name>/SKILL.md` file and follow its workflow. Key skills:

| Skill | Purpose |
|-------|---------|
| `gitflow-commit` | View/diff/patch/comment on commits |
| `gitflow-pr-create` | Create Pull Requests |
| `gitflow-pr-review` | Code review on PRs |
| `gitflow-pr-inline-review` | Inline PR comments |
| `gitflow-pr-apply-feedback` | Apply reviewer feedback |
| `gitflow-pr` | PR lifecycle (merge/close/reopen) |
| `gitflow-issue` | Issue management |
| `gitflow-precommit` | Pre-commit quality gate |
| `gitflow-quality` | 6-gate quality check |
| `gitflow-release` | Release workflow |
| `gitflow-security-check` | Security scanning |
| `gitflow-pipeline-analyzer` | CI/CD pipeline analysis |
| `gitflow-weekly-report` | Weekly report generation |
