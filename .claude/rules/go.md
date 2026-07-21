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
- Cross-module imports use `github.com/byx-darwin/go-tools/<module>` paths.
- Do not create circular dependencies between modules.
- When adding a cross-module dependency, update both `go.mod` files and `go.work.sum`.
- Keep modules independently publishable — avoid tight coupling.

### Evolution to 3-Library Split

Current modules are being reorganized into three libraries (see `specs/02_split_plan_summary.md`):

| Current module | Target library | Dependency rule |
|---------------|---------------|-----------------|
| `tools/` | `go-common` | Zero framework dependency |
| `middleware/` | `go-middleware` | No Hertz/Kitex dependency |
| `config/`, `hertz/`, `kitex/` | `go-framework` | Depends on go-common + go-auth (sibling of go-middleware) |

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

## 8. Static Analysis (golangci-lint)

项目使用 `golangci-lint` 进行静态分析，配置在 `.golangci.yml`。所有新代码 MUST 通过 lint 检查。

### 8.1 启用的 Linter

| Linter | 规则 | 编码要求 |
|--------|------|---------|
| `gofmt` | 代码格式 | 所有文件必须 gofmt 格式化 |
| `goimports` | import 分组 | 标准库 / 第三方 / 本项目 三组 |
| `revive` | 导出符号注释 | 所有 `exported` 类型、函数、常量、变量必须有 `// Name ...` 格式的 godoc 注释 |
| `errcheck` | 错误返回值 | 函数返回的 error 必须处理：检查、`_ =` 显式忽略、或 `require.NoError` |
| `gocritic` | 代码风格 | 见下方具体规则 |
| `misspell` | 拼写 | 使用美式拼写 |
| `unconvert` | 冗余类型转换 | 避免不必要的类型转换 |
| `unparam` | 未使用参数 | 未使用的参数用 `_` 替代 |

### 8.2 gocritic 具体规则

- **octalLiteral**: 使用新八进制写法 `0o644` 而非 `0644`
- **paramTypeCombine**: 同类型参数合并 `func(a, b int)` 而非 `func(a int, b int)`
- **builtinShadow**: 不要覆盖内置标识符（如 `max`、`min`、`copy`）
- **unnecessaryDefer**: 不要在 `return` 之前的语句用 `defer`
- **whyNoLint**: `//nolint:xxx` 必须附带解释 `//nolint:xxx // 原因`

### 8.3 godoc 注释规范

```go
// ✅ 正确：注释以符号名开头
// LRU 最近最少使用淘汰算法。
const LRU = hot.LRU

// Send 发送 HTTP 请求。
func Send(url string) error { ... }

// ❌ 错误：缺少注释或注释不以符号名开头
const LRU = hot.LRU
// 发送请求
func Send(url string) error { ... }
```

分组常量/变量时，每个分组前加一行分组注释即可：
```go
// Redis 错误码 20001-20099。
const (
    CodeRedisConnect = 20001 // Redis 连接失败
    CodeRedisPing    = 20002 // Redis Ping 失败
)
```

### 8.4 错误处理规范

```go
// ✅ defer 清理资源时显式忽略
defer func() { _ = f.Close() }()

// ✅ 测试中使用 require 检查
require.NoError(t, store.Set("key", "value"))

// ✅ 生产代码中 nolint 必须带解释
InsecureSkipVerify: true, //nolint:gosec // 用户可通过配置显式关闭 TLS 校验

// ❌ 不允许
defer f.Close()          // errcheck 报错
//nolint:gosec           // 缺少解释
```

### 8.5 类型规范

- 使用 `any` 替代 `interface{}`（Go 1.18+）

### 8.6 Workspace 下运行 lint

**所需版本：golangci-lint v2（>= v2.12.2）**。`.golangci.yml` 为 v2 格式，用 v1 运行会报 *"configuration file for golangci-lint v2 with golangci-lint v1"* 而失败。升级：

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

golangci-lint 不支持 workspace 根目录 `./...`，必须逐 module 运行：

```bash
for m in go-common go-auth go-middleware go-framework; do
  golangci-lint run --timeout=5m ./$m/...
done
```

## 9. Avoid

- Large opportunistic refactors during behavior work
- Mixing docs-only, refactor-only, and behavior changes without need
- Renaming or reshaping stable exported APIs without updating dependent tests
- Editing unrelated files just to make style more uniform
- Creating cross-module circular dependencies
- Writing custom cache eviction implementations — use `github.com/samber/hot` (`hot.NewHotCache[K,V](hot.LRU, max).Build()`) instead of `tools/cache/core/`. See `specs/06_cache_algorithm_guide.md` for algorithm selection (LRU default, FIFO for queues, LFU for hotspots, CLOCK for memory-constrained).
