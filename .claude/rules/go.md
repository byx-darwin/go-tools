# Go Coding Rules

This file defines Go-specific coding rules for the `go-tools` workspace.
Execution flow, testing order, and risk control live in `.claude/rules/agent-engineering.md`.

## 1. Goals

- Keep Go changes small, readable, and consistent with the current codebase.
- Preserve stable public API contracts unless the task explicitly changes them.
- Prefer existing repository patterns over introducing new abstractions.
- Respect module boundaries — each sub-module has its own `go.mod`.
- Write code with the 3-library split in mind (go-common / go-middleware / go-framework).

## 2. General Style

- Follow standard Go style and keep files `gofmt`-clean.
- Prefer small, focused functions over large rewrites.
- Prefer explicit, readable code over clever abstractions.
- Reuse existing helpers before adding new utility layers.
- Keep naming aligned with nearby packages and existing exported APIs.

## 3. Workspace Structure

Current modules (being deprecated per D5):

- `config/`: Configuration loading (Polaris, DB, Hertz, Kitex, Kafka, Redis). → go-framework
- `hertz/`: Hertz HTTP server setup, response helpers, middleware, registry. → go-framework
- `kitex/`: Kitex RPC options, discovery, registry, error handling. → go-framework
- `middleware/`: Shared middleware (Redis, Kafka/Sarama producer). → go-middleware
- `tools/`: Utility libraries (crypto, HTTP client, cache, CAPTCHA, Ent helpers, time, netutil). → go-common

Strategic planning documents live in `specs/`. See `CLAUDE.md` for spec index.

## 4. Module Boundaries

- Each sub-module has its own `go.mod` and is part of the `go.work` workspace.
- Cross-module imports use `gitee.com/byx_darwin/go-tools/<module>` paths.
- Do not create circular dependencies between modules.
- When adding a cross-module dependency, update both `go.mod` files and `go.work.sum`.
- Keep modules independently publishable — avoid tight coupling.

### Evolution to 3-Library Split

Current modules are being reorganized into three libraries (see `specs/02_split_plan_summary.md`):

| Current module | Target library | Dependency rule |
|---------------|---------------|-----------------|
| `tools/` | `go-common` | Zero framework dependency |
| `middleware/` | `go-middleware` | No Hertz/Kitex dependency |
| `config/`, `hertz/`, `kitex/` | `go-framework` | Depends on go-common + go-middleware |

When adding new code, place it according to these rules:
- Pure utilities (crypto, cache, time, net) → go-common tier
- Middleware clients (redis, kafka, db, es) → go-middleware tier
- Framework adapters (hertz, kitex, config, observability) → go-framework tier

## 5. Errors and Control Flow

- Return clear errors; do not swallow errors silently.
- Wrap errors with useful context when crossing package or module boundaries.
- Preserve existing error wording when tests or public contracts rely on it.
- Prefer early returns to deeply nested control flow.
- **Error library**: Use `oops` as the primary error library (D3). go-framework will provide a pkg/errors → oops bridge.
- **Error code ranges**: go-framework 10000-10499, go-middleware 20000-20699, project custom 40000-59999.

## 6. Tests and Test Placement

- Put tests close to the code they validate (`*_test.go` alongside source).
- Add helper-level tests for pure logic, output formatting, and utility functions.
- When changing exported APIs, update or add corresponding tests.
- Run per-module tests when working on a specific module: `go test ./tools/... -count=1`

## 7. Documentation and Code Coupling

- When Go changes affect public API behavior, update relevant docs or comments.
- If code introduces a new exported type, function, or interface, document it with godoc comments.
- Keep README and examples aligned with the actual API.

## 8. Avoid

- Large opportunistic refactors during behavior work
- Mixing docs-only, refactor-only, and behavior changes without need
- Renaming or reshaping stable exported APIs without updating dependent tests
- Editing unrelated files just to make style more uniform
- Creating cross-module circular dependencies
- Writing custom cache eviction implementations — use `github.com/samber/hot` (`hot.NewHotCache[K,V](hot.LRU, max).Build()`) instead of `tools/cache/core/`. See `specs/06_cache_algorithm_guide.md` for algorithm selection (LRU default, FIFO for queues, LFU for hotspots, CLOCK for memory-constrained).
