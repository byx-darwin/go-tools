# CLAUDE.md — go-tools

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**go-tools** is a Go workspace (`go.work`) containing shared libraries and tooling for Hertz (HTTP) and Kitex (RPC) microservice development. It is being actively restructured to serve as the foundation for [ncgo](https://github.com/byx-darwin/ncgo) scaffold-generated projects.

**Core problem being solved**: ncgo currently embeds full implementation code (config, interceptors, response, rpcerror, redis, kafka) in generated projects, leading to duplicated code across projects. The goal is to make generated projects thin adapters that `import` from go-tools libraries.

### Current Structure (5 modules, being deprecated — see D5)

| Module | Purpose | Status |
|--------|---------|--------|
| `config` | Configuration loading (Polaris, DB, Hertz, Kitex, Kafka, Redis) | → go-framework |
| `hertz` | Hertz HTTP server setup, response helpers, middleware, registry | → go-framework |
| `kitex` | Kitex RPC options, discovery, registry, error handling | → go-framework |
| `middleware` | Shared middleware (Redis, Kafka/Sarama producer) | → go-middleware |
| `tools` | Utility libraries (crypto, HTTP client, cache, CAPTCHA, Ent helpers, time) | → go-common |

### Target Structure (3 independent libraries, see `specs/` for details)

```text
go-common          ← 最底层，零框架依赖 (crypto, cache, httpclient, log, timeutil, netutil, captcha, auth, entutil)
    ↑
go-middleware       ← 中间件客户端 (redis, kafka, db, es, clickhouse, tls)
    ↑
go-framework        ← 框架适配 (hertz, kitex, config, observability, accesslog, rpcerror)
```

Each library will be independently versioned (D4). See `specs/02_split_plan_summary.md` for full mapping.

## Key Decisions (Confirmed 2026-06-23)

| # | Decision | Conclusion |
|---|---------|-----------|
| D1 | Kafka library | **kafka-go** (matches ncgo choice), existing sarama marked deprecated |
| D2 | Config time units | **time.Duration** (YAML: `30s` format) |
| D3 | Error library | **oops** as primary, go-framework provides pkg/errors → oops bridge |
| D4 | Release strategy | **Independent versioning** (go-common changes least, go-framework most) |
| D5 | Old modules (config/hertz/kitex/middleware/tools) | **Mark deprecated**, keep in go.work for 1-2 release cycles |

## Error Code Ranges

```
go-framework: 10000-10499  (system, param, auth, config, RPC middleware)
go-middleware: 20000-20699 (redis, kafka, db, es, clickhouse, observability)
Project custom: 40000-59999 (business modules, external dependencies)
```

See `specs/00_overview.md` for full error code table.

## Spec Documents

| File | Content |
|------|---------|
| `specs/00_overview.md` | Overall alignment plan, decisions, error code overview |
| `specs/01_inventory_mapping.md` | ncgo template content vs go-tools capabilities mapping |
| `specs/02_split_plan_summary.md` | 3-library split plan with module归属 |
| `specs/03_template_alignment.md` | ncgo template改造 details (Kitex + Hertz) |
| `specs/04_config_schema_alignment.md` | Config struct & YAML schema alignment |
| `specs/05_implementation_phases.md` | 5-phase implementation plan (~10 working days) |
| `specs/06_cache_algorithm_guide.md` | Cache algorithm selection guide (FIFO/LRU/LFU/CLOCK/MRU + samber/hot) |

## Development Commands

```bash
# Build all modules in workspace
go build ./...

# Test all modules
go test ./... -count=1

# Test a specific module
go test ./tools/... -count=1
go test ./config/... -count=1

# Lint
gofmt -l $(find . -name '*.go' -not -path '*/vendor/*' -not -path './.git/*')
go vet ./...

# Full validation (CI-equivalent)
go build ./... && go vet ./... && go test ./... -count=1

# Pre-commit setup
pre-commit install --install-hooks --hook-type pre-commit --hook-type pre-push
```

**Prerequisites:** Go 1.24.1+ (workspace mode via `go.work`).

## Architecture

### Current Workspace Layout

```text
go.work                  → Workspace root (go 1.24.1)
config/                  → Configuration loading (deprecated → go-framework)
  config.go              → Core config loader (Jaeger/Registry)
  polaris.go             → Polaris service discovery config
  db/                    → Database configuration
  hertz/                 → Hertz-specific config
  kitex/                 → Kitex-specific config
  kafka/                 → Kafka config
  redis/                 → Redis config
hertz/                   → Hertz HTTP server (deprecated → go-framework)
  server.go              → Server setup and lifecycle
  response.go            → Response helpers (OK/Err/BindError)
  middleware/            → Hertz middleware (CORS/Auth/Casbin/Location)
  registry/              → Service registry (Polaris)
kitex/                   → Kitex RPC (deprecated → go-framework)
  option/                → RPC server/client options
  discover/              → Service discovery (Polaris)
  registry/              → Service registry (Polaris)
  rpc_error/             → ErrorType enum-based errors
middleware/              → Shared middleware (deprecated → go-middleware)
  redis/                 → Redis client factory
  kafka/                 → Kafka/Sarama producer (D1: → kafka-go)
tools/                   → Utility libraries (deprecated → go-common)
  ak.go                  → AK/SK generation
  crypto/                → Encryption (MD5/SHA/HMAC/TEA)
  http_client/           → HTTP client with retry, m3u8 support
  cache/                 → Cache interface (will use samber/hot instead of custom FIFO/LRU/LFU/CLOCK/MRU)
  captcha/               → CAPTCHA generation with cache
  entutils/              → Ent ORM helpers and drivers
  time/                  → Time formatting
  netutil/               → Network utilities
specs/                   → Strategic planning documents (split plan, alignment)
```

### Implementation Phases

```text
Phase 1: go-common 拆分          (2 days) — crypto, cache, httpclient, log, timeutil, netutil, captcha, auth, entutil
Phase 2: go-middleware 拆分      (2.5 days) — redis, kafka(kafka-go), db, es, clickhouse, tls
Phase 3: go-framework 拆分       (3 days) — hertz, kitex, config, observability, accesslog, rpcerror(oops)
Phase 4: ncgo 模板改造           (2 days) — templates import from 3 libraries instead of embedding
Phase 5: 验证 + 文档             (1.5 days) — E2E validation, docs, migration guide
```

Phases 1-3 are sequential (have dependencies). See `specs/05_implementation_phases.md` for full task breakdown.

## Key Contracts

### Module boundaries
Each sub-module has its own `go.mod`. Cross-module dependencies must use the `gitee.com/byx_darwin/go-tools/*` import path and be declared in `go.work`. Do not create circular dependencies between modules.

### Evolution direction
New code should be written with the 3-library split in mind:
- **go-common**: zero framework dependency, pure utility
- **go-middleware**: middleware clients, no Hertz/Kitex dependency
- **go-framework**: Hertz/Kitex adapters, depends on go-common + go-middleware

When adding features, consider which library they belong to. See `specs/02_split_plan_summary.md`.

### ncgo alignment
Code in go-tools is being aligned with ncgo's template implementations:
- **Config structs**: go-tools structs become the "source of truth", ncgo templates import them
- **Error handling**: Unifying on `oops` style (ncgo approach), with bridge from pkg/errors
- **Logging**: Structured key-value style (`klog.CtxKVLog`), not printf-style
- **Kafka**: Switching from sarama to kafka-go (D1)
- **Time config**: Using `time.Duration` with YAML `30s` format (D2)
- **Cache**: Using `github.com/samber/hot` (generic HotCache[K,V] with LRU/FIFO/LFU/CLOCK/MRU + TTL) instead of custom implementations. See `specs/06_cache_algorithm_guide.md` for algorithm selection guide.

### Public API stability
Exported functions, types, and interfaces in any module are contract-sensitive. Changes require updating tests and docs together.

## Testing

- **Unit tests**: `*_test.go` alongside code for helpers, pure logic, utilities.
- **Integration tests**: Cross-module wiring tests where applicable.
- Run tests per-module when working on a specific module; run `go test ./... -count=1` for full validation.

## Rules

Hand-authored rules in `.claude/rules/`:
- `go.md` — Go coding style, workspace structure, module boundaries.
- `agent-engineering.md` — Execution workflow, validation order, failure handling, risk control.

## Ruflo — Claude Code Configuration

## Rules

- Do what has been asked; nothing more, nothing less
- NEVER create files unless absolutely necessary — prefer editing existing files
- NEVER create documentation files unless explicitly requested
- NEVER save working files or tests to root — use `/src`, `/tests`, `/docs`, `/config`, `/scripts`
- ALWAYS read a file before editing it
- NEVER commit secrets, credentials, or .env files
- NEVER add a `Co-Authored-By` trailer to user commits unless this project's `.claude/settings.json` has `attribution.commit` set (#2078). The Claude Code Bash tool may suggest one in its default commit-message template — ignore it. `Co-Authored-By` is semantic authorship attribution under git/GitHub convention; the tool is the facilitator, not a co-author.
- Keep files under 500 lines
- Validate input at system boundaries

## Agent Comms (SendMessage-First Coordination)

Named agents coordinate via `SendMessage`, not polling or shared state.

```
Lead (you) ←→ architect ←→ developer ←→ tester ←→ reviewer
              (named agents message each other directly)
```

### Spawning a Coordinated Team

```javascript
// ALL agents in ONE message, each knows WHO to message next
Agent({ prompt: "Research the codebase. SendMessage findings to 'architect'.",
  subagent_type: "researcher", name: "researcher", run_in_background: true })
Agent({ prompt: "Wait for 'researcher'. Design solution. SendMessage to 'coder'.",
  subagent_type: "system-architect", name: "architect", run_in_background: true })
Agent({ prompt: "Wait for 'architect'. Implement it. SendMessage to 'tester'.",
  subagent_type: "coder", name: "coder", run_in_background: true })
Agent({ prompt: "Wait for 'coder'. Write tests. SendMessage results to 'reviewer'.",
  subagent_type: "tester", name: "tester", run_in_background: true })
Agent({ prompt: "Wait for 'tester'. Review code quality and security.",
  subagent_type: "reviewer", name: "reviewer", run_in_background: true })

// Kick off the pipeline
SendMessage({ to: "researcher", summary: "Start", message: "[task context]" })
```

### Patterns

| Pattern | Flow | Use When |
|---------|------|----------|
| **Pipeline** | A → B → C → D | Sequential dependencies (feature dev) |
| **Fan-out** | Lead → A, B, C → Lead | Independent parallel work (research) |
| **Supervisor** | Lead ↔ workers | Ongoing coordination (complex refactor) |

### Rules

- ALWAYS name agents — `name: "role"` makes them addressable
- ALWAYS include comms instructions in prompts — who to message, what to send
- Spawn ALL agents in ONE message with `run_in_background: true`
- After spawning: STOP, tell user what's running, wait for results
- NEVER poll status — agents message back or complete automatically

## Swarm & Routing

### Config
- **Topology**: hierarchical-mesh (anti-drift)
- **Max Agents**: 15
- **Memory**: hybrid
- **HNSW**: Enabled
- **Neural**: Enabled

```bash
npx @claude-flow/cli@latest swarm init --topology hierarchical --max-agents 8 --strategy specialized
```

### Agent Routing

| Task | Agents | Topology |
|------|--------|----------|
| Bug Fix | researcher, coder, tester | hierarchical |
| Feature | architect, coder, tester, reviewer | hierarchical |
| Refactor | architect, coder, reviewer | hierarchical |
| Performance | perf-engineer, coder | hierarchical |
| Security | security-architect, auditor | hierarchical |

### When to Swarm
- **YES**: 3+ files, new features, cross-module refactoring, API changes, security, performance
- **NO**: single file edits, 1-2 line fixes, docs updates, config changes, questions

### 3-Tier Model Routing

| Tier | Handler | Use Cases |
|------|---------|-----------|
| 1 | Agent Booster (WASM) | Simple transforms — skip LLM, use Edit directly |
| 2 | Haiku | Simple tasks, low complexity |
| 3 | Sonnet/Opus | Architecture, security, complex reasoning |

## Memory & Learning

### Before Any Task
```bash
npx @claude-flow/cli@latest memory search --query "[task keywords]" --namespace patterns
npx @claude-flow/cli@latest hooks route --task "[task description]"
```

### After Success
```bash
npx @claude-flow/cli@latest memory store --namespace patterns --key "[name]" --value "[what worked]"
npx @claude-flow/cli@latest hooks post-task --task-id "[id]" --success true --store-results true
```

### MCP Tools (use `ToolSearch("keyword")` to discover)

| Category | Key Tools |
|----------|-----------|
| **Memory** | `memory_store`, `memory_search`, `memory_search_unified` |
| **Bridge** | `memory_import_claude`, `memory_bridge_status` |
| **Swarm** | `swarm_init`, `swarm_status`, `swarm_health` |
| **Agents** | `agent_spawn`, `agent_list`, `agent_status` |
| **Hooks** | `hooks_route`, `hooks_post-task`, `hooks_worker-dispatch` |
| **Security** | `aidefence_scan`, `aidefence_is_safe`, `aidefence_has_pii` |
| **Hive-Mind** | `hive-mind_init`, `hive-mind_consensus`, `hive-mind_spawn` |

### Background Workers

| Worker | When |
|--------|------|
| `audit` | After security changes |
| `optimize` | After performance work |
| `testgaps` | After adding features |
| `map` | Every 5+ file changes |
| `document` | After API changes |

```bash
npx @claude-flow/cli@latest hooks worker dispatch --trigger audit
```

## Agents

**Core**: `coder`, `reviewer`, `tester`, `planner`, `researcher`
**Architecture**: `system-architect`, `backend-dev`, `mobile-dev`
**Security**: `security-architect`, `security-auditor`
**Performance**: `performance-engineer`, `perf-analyzer`
**Coordination**: `hierarchical-coordinator`, `mesh-coordinator`, `adaptive-coordinator`
**GitHub**: `pr-manager`, `code-review-swarm`, `issue-tracker`, `release-manager`

Any string works as a custom agent type.

## Build & Test

- ALWAYS run tests after code changes
- ALWAYS verify build succeeds before committing

```bash
npm run build && npm test
```

## CLI Quick Reference

```bash
npx @claude-flow/cli@latest init --wizard           # Setup
npx @claude-flow/cli@latest swarm init --v3-mode     # Start swarm
npx @claude-flow/cli@latest memory search --query "" # Vector search
npx @claude-flow/cli@latest hooks route --task ""    # Route to agent
npx @claude-flow/cli@latest doctor --fix             # Diagnostics
npx @claude-flow/cli@latest security scan            # Security scan
npx @claude-flow/cli@latest performance benchmark    # Benchmarks
```

26 commands, 140+ subcommands. Use `--help` on any command for details.

## Setup

```bash
claude mcp add claude-flow -- npx -y @claude-flow/cli@latest
npx @claude-flow/cli@latest daemon start
npx @claude-flow/cli@latest doctor --fix
```

**Agent tool** handles execution (agents, files, code, git). **MCP tools** handle coordination (swarm, memory, hooks). **CLI** is the same via Bash.
