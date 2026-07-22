# CLAUDE.md — go-tools

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**go-tools** is a Go workspace (`go.work`) containing four independently versioned libraries for Hertz (HTTP) and Kitex (RPC) microservice development. It serves as the foundation for [ncgo](https://github.com/byx-darwin/ncgo) scaffold-generated projects.

The 5-module → 3-library split (2026-06-23) is **complete**. ncgo generated projects now import from these libraries instead of embedding duplicated code.

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

## Key Decisions (Confirmed 2026-06-23)

| # | Decision | Conclusion | Status |
|---|---------|-----------|--------|
| D1 | Kafka library | **kafka-go** (matches ncgo choice) | ✅ done |
| D2 | Config time units | **time.Duration** (YAML: `30s` format) | ✅ done |
| D3 | Error library | **oops** as primary | ✅ done |
| D4 | Release strategy | **Independent versioning** | ✅ active |
| D5 | Old modules | **Fully removed**, all code migrated to 3 libraries | ✅ done |
| D6 | Error code ownership | **Owning modules** define their codes; `go-common/error` = mechanism + band boundaries + HTTP registry | ✅ active |

## Error Code Ranges

```
go-framework: 10000-10499  (system, param, auth, config, RPC; defined in go-framework/error + obs 20601-20605)
go-middleware: 20000-20699 (clickhouse 20401-20403, tls 20501-20504 defined in-package; redis/kafka/db/es bands reserved)
go-auth:       40000-40099 (token, session, device auth errors; defined in go-auth/error)
Project custom: 40100-59999 (business modules, external dependencies; no library predefinitions)
```

See `specs/00_overview.md` for full error code table.

## Spec Documents

| File | Content |
|------|---------|
| `specs/00_overview.md` | Overall alignment plan, decisions, error code overview |
| `specs/01_split_plan_summary.md` | 3-library split plan with module mapping |
| `specs/02_config_schema_alignment.md` | Config struct & YAML schema alignment |
| `specs/03_implementation_phases.md` | 5-phase implementation plan (✅ completed) |
| `specs/04_cache_algorithm_guide.md` | Cache algorithm selection guide (FIFO/LRU/LFU/CLOCK/ARC + samber/hot) |
| `specs/05_migration_guide.md` | Migration guide: go-tools v1 → v2 |
| `specs/06_kitex_observability_migration.md` | obs-opentelemetry (Kitex) migration ✅ completed |
| `specs/07_hertz_observability_migration.md` | obs-opentelemetry (Hertz) migration ✅ completed |
| `specs/08_hertz_response_redesign.md` | Hertz Response 重构设计（Responder 对象 + 中间件模式） |

## Development Commands

```bash
# Build all modules in workspace
go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...

# Test all modules
go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1

# Test a specific module
go test ./go-common/... -count=1
go test ./go-middleware/... -count=1
go test ./go-framework/... -count=1

# Lint
gofmt -l $(find . -name '*.go' -not -path '*/vendor/*' -not -path './.git/*')
go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...

# Lint (golangci-lint, workspace 必须逐 module 运行)
for m in go-common go-auth go-middleware go-framework; do golangci-lint run --timeout=5m ./$m/...; done

# Full validation (CI-equivalent)
go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... && \
  go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... && \
  for m in go-common go-auth go-middleware go-framework; do golangci-lint run --timeout=5m ./$m/... || exit 1; done && \
  go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1

# Pre-commit setup
pre-commit install --install-hooks --hook-type pre-commit --hook-type pre-push
```

**Prerequisites:** Go 1.25+ (workspace mode via `go.work`).

## Architecture

### Workspace Layout

```text
go.work                    → Workspace root (go 1.25.8)
go-common/                 → Zero-dependency utilities
  cache/                   → Generic cache (samber/hot wrapper): LRU/LFU/FIFO/TwoQueue/ARC
  captcha/                 → CAPTCHA generation with cache
  crypto/                  → Encryption (MD5/SHA/HMAC/TEA)
  httpclient/              → HTTP client with retry, m3u8 support
  log/                     → Structured logging (slog + lumberjack + OTel)
  netutil/                 → Network utilities
  timeutil/                → Time formatting helpers
  auth/                    → Auth helpers (AK/SK)
  error/                   → Error mechanism (oops Builder/Extract + band boundaries + HTTP status registry)
go-middleware/             → Middleware clients (no Hertz/Kitex dependency)
  redis/                   → Redis client (go-redis v9, UniversalClient)
  kafka/                   → Kafka client (kafka-go)
  db/                      → Database helpers
  es/                      → Elasticsearch client
  clickhouse/              → ClickHouse client
  tls/                     → TLS connection setup (火山引擎)
go-framework/              → Framework adapters (depends on go-common + go-auth; sibling of go-middleware)
  error/                   → Framework error codes (10000-10013 + obs 20601-20605, package frameworkerror)
  hertz/                   → Hertz HTTP server, response helpers, middleware
  hertz/observability/     → OTel Tracing + Metrics (W3C + B3 propagator, runtime metrics)
  kitex/                   → Kitex RPC options, discovery, registry, rpcerror
  kitex/observability/     → OTel Tracing + Metrics (stats.Tracer, Suite, peer service topology)
  config/                  → Configuration loading (Polaris, DB, Hertz, Kitex, Kafka, Redis)
specs/                     → Strategic planning documents
  ...
  06_kitex_observability_migration.md → Kitex OTel migration ✅
  07_hertz_observability_migration.md → Hertz OTel migration ✅
```

## Key Contracts

### Module boundaries
Each sub-module has its own `go.mod`. Cross-module dependencies use `github.com/byx-darwin/go-tools/<module>` import paths and are declared in `go.work`. Do not create circular dependencies between modules.

### Dependency hierarchy
The four libraries form a **DAG, not a linear chain**. `go-framework` and `go-middleware` are **siblings** — both consume `go-auth` + `go-common`, and neither depends on the other.
- **go-common**: zero framework dependency, pure utility — does not import any other library
- **go-auth**: auth utilities — may import go-common only
- **go-middleware**: middleware clients — may import go-common and go-auth, must NOT import go-framework
- **go-framework**: Hertz/Kitex adapters — may import go-common and go-auth, must NOT import go-middleware (siblings)

### ncgo alignment
- **Config structs**: go-tools structs are the "source of truth", ncgo templates import them
- **Error handling**: Unified on `oops` style. Error codes: go-framework 10000-10499, go-middleware 20000-20699
- **Logging**: Structured key-value style (`slog`-based, `log.Info("msg", "key", val)`)
- **Kafka**: Using `kafka-go` (not sarama). D1.
- **Time config**: Using `time.Duration` with YAML `30s` format. D2.
- **Cache**: Using `github.com/samber/hot` wrapper (`cache.New[string,int](cache.LRU, 100).Build()`). See `specs/04_cache_algorithm_guide.md`.

### Functional Options Pattern
New code with 3+ constructor params or 5+ config fields MUST use the Options pattern. See `.claude/rules/options-pattern.md` for the standard template.

### Public API stability
Exported functions, types, and interfaces are contract-sensitive. Changes require updating tests and docs together.

## Testing

- **Unit tests**: `*_test.go` alongside code for helpers, pure logic, utilities.
- **Integration tests**: Cross-module wiring tests where applicable.
- Run tests per-module when working on a specific module; run `go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1` for full validation.

## Rules

Hand-authored rules in `.claude/rules/`:
- `go.md` — Go coding style, workspace structure, module boundaries, **static analysis (golangci-lint) rules**.
- `agent-engineering.md` — Execution workflow, validation order, failure handling, risk control.
- `options-pattern.md` — Functional Options pattern standard.

<!-- OPENWIKI:START -->

## OpenWiki

This repository uses OpenWiki for recurring code documentation. Start with `openwiki/quickstart.md`, then follow its links to architecture, workflows, domain concepts, operations, integrations, testing guidance, and source maps.

The scheduled OpenWiki GitHub Actions workflow refreshes the repository wiki. Do not hand-edit generated OpenWiki pages unless explicitly asked; prefer updating source code/docs and letting OpenWiki regenerate.

<!-- OPENWIKI:END -->
