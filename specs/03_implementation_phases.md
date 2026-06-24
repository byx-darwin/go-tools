# go-tools 实施阶段

> **实施日期：2026-06-23 | 状态：已完成 ✅**

Phase 1-3, P0-P1, Phase 5 全部完成。

```text
Phase 1: go-common      ██████████  9 包 ✅
Phase 2: go-middleware  ██████████  7 包 ✅ (+tls)
Phase 3: go-framework   ██████████ 12 包 ✅
Phase 5: 验证 + 文档    ██████████  ✅
```

---


> 总工期预估：~10 个工作日

## 阶段总览

```text
Phase 1: go-common 拆分          ████████░░ 2 天
Phase 2: go-middleware 拆分      ██████████ 2.5 天
Phase 3: go-framework 拆分       ██████████ 3 天
Phase 5: 验证 + 文档             ██████░░░░ 1.5 天
```

## Phase 1：go-common 拆分（2 天）

### 目标
创建零框架依赖的通用工具库。

### 任务清单

| # | 任务 | 工作量 |
|---|------|--------|
| 1.1 | 创建 `go-common/` 目录、`go.mod`、`README.md` | 0.5h |
| 1.2 | 搬迁 `tools/crypto/` → `go-common/crypto/` | 0.5h |
| 1.3 | 搬迁 `tools/cache/` → `go-common/cache/`，底层替换为 `github.com/samber/hot`（删除 `core/` 下自定义 FIFO/LRU/LFU/CLOCK/MRU 实现，对齐的 `hot.NewHotCache[K,V](hot.LRU, max).Build()` 用法） | 1.5h |
| 1.4 | 搬迁 `tools/http_client/` → `go-common/httpclient/` | 1h |
| 1.5 | 搬迁 `tools/time/` → `go-common/timeutil/` | 0.5h |
| 1.6 | 搬迁 `tools/netutil/` → `go-common/netutil/` | 0.5h |
| 1.7 | 搬迁 `tools/captcha/` → `go-common/captcha/` | 0.5h |
| 1.8 | 搬迁 `tools/ak.go` → `go-common/auth/ak.go` | 0.5h |
| 1.9 | 搬迁 `tools/entutils/` → `go-common/entutil/` | 1h |
| 1.10 | **新建** `go-common/log/` — 基于 `log/slog` 的日志库（slog.Handler + lumberjack 轮转 + gzip 压缩 + OTel span 联动）+ `adapters/` 包（klog/hlog 适配器，对齐） | 3h |
| 1.11 | 调整 import 路径 + 跑通所有测试 | 2h |

**风险**：
- `tools/` 的测试可能依赖 `config/` 模块，搬迁后需解除依赖

**验收**：
- `cd go-common && go test ./...` 全部通过
- 没有 import `github.com/byx-darwin/go-tools/` 的旧路径

## Phase 2：go-middleware 拆分（2.5 天）

### 目标
创建不依赖框架的中间件客户端库。

### 任务清单

| # | 任务 | 工作量 |
|---|------|--------|
| 2.1 | 创建 `go-middleware/` 目录、`go.mod`、`README.md` | 0.5h |
| 2.2 | 搬迁 `middleware/redis/` + `config/redis/` → `go-middleware/redis/` | 2h |
| 2.3 | **新建** `go-middleware/kafka/` — 基于 kafka-go 重写（替代 sarama） | 4h |
| 2.4 | 搬迁 `config/db/` → `go-middleware/db/` | 0.5h |
| 2.5 | **新建** `go-middleware/es/` — Elasticsearch 客户端 + 配置回流 | 2h |
| 2.6 | **新建** `go-middleware/clickhouse/` — ClickHouse 客户端 + 配置回流 | 2h |
| 2.7 | Redis 配置结构体增强（支持 sentinel/RESP3/UniversalClient） | 2h |
| 2.8 | 各中间件包定义 `errors.go`（预定义 oops 错误码 20000-20699） | 1.5h |
| 2.9 | **新建** `go-middleware/tls/` — 火山引擎日志服务 TLS 客户端（Producer/Consumer） | 2h |
| 2.10 | 调整 import + 跑通测试 | 1h |

**风险**：
- Kafka 库切换（sarama → kafka-go）需要全新实现 Writer/Reader 工厂方法
- Redis 从 `*redis.Client` 升级到 `UniversalClient` 涉及 API 变化
- TLS 日志服务依赖火山引擎 SDK（`volc-sdk-golang/service/tls`），需评估 license 兼容性

**验收**：
- `cd go-middleware && go test ./...` 全部通过
- Redis/Kafka 客户端可以独立创建和连接

## Phase 3：go-framework 拆分（3 天）

### 目标
创建 Hertz/Kitex 框架适配层。这是最复杂的阶段。

### 任务清单

| # | 任务 | 工作量 |
|---|------|--------|
| 3.1 | 创建 `go-framework/` 目录、`go.mod`、`README.md` | 0.5h |
| 3.2 | 搬迁 `config/config.go` + `config/polaris.go` → `go-framework/config/` | 1h |
| 3.3 | 搬迁 `config/hertz/` → `go-framework/config/hertz/` | 0.5h |
| 3.4 | 搬迁 `config/kitex/` → `go-framework/config/kitex/`（字段统一为 Duration） | 1h |
| 3.5 | 搬迁 `hertz/` → `go-framework/hertz/`（server/response/middleware/registry） | 2h |
| 3.6 | **新增** `go-framework/kitex/middleware/` — 回流中间件 | 3h |
| 3.7 | **新增** `go-framework/kitex/rpcerror/` — 统一 oops 风格的错误处理 | 2h |
| 3.8 | 搬迁 `kitex/option/` + `kitex/registry/` + `kitex/discover/` | 2h |
| 3.9 | 搬迁 `kitex/rpc_error/` → 合并到 `go-framework/kitex/rpcerror/` | 1h |
| 3.10 | 统一 `go-framework/config/loader.go` — 标准 YAML 加载器 | 1h |
| 3.11 | **新增** `go-framework/config/observability.go` — 可观测统一配置结构体 | 0.5h |
| 3.12 | **新增** `go-framework/kitex/observability/` — Kitex 链路追踪 Provider（`kitex-contrib/obs-opentelemetry`） | 2h |
| 3.13 | **新增** `go-framework/hertz/observability/` — Hertz 链路追踪 Provider（`hertz-contrib/obs-opentelemetry`） | 2h |
| 3.14 | **新增** `go-framework/kitex/middleware/accesslog.go` — Kitex 结构化 Access Log 中间件 | 1h |
| 3.15 | **新增** `go-framework/hertz/middleware/accesslog.go` — Hertz 结构化 Access Log 中间件 | 1h |
| 3.16 | 调整 import + 跑通测试 | 2h |

**关键决策**：
- 中间件是从 **反哺**到 go-framework（模板经验 → 库代码）
- rpcerror 统一用 `oops` 风格，已完全替代 ErrorType 枚举 ✅

**验收**：
- `cd go-framework && go test ./...` 全部通过
- 可以用 `go-framework` 的 option 工厂启动一个最小 Kitex 服务


**风险**：
- 模板中残留的 import 路径需要全面替换

**验收**：
- `go test ./...` 全部通过
- `./scripts/smoke.sh` 通过
- 用新模板生成一个测试项目，确认可以编译

## Phase 5：验证 + 文档（1.5 天）

### 任务清单

| # | 任务 | 工作量 |
|---|------|--------|
| 5.0 | **自动化冒烟测试**：CI 新增 job，每次 PR 自动执行：` generate` → 生成 Hertz + Kitex 项目 → `go build ./...` → `go vet ./...` | 2h |
| 5.1 | 端到端验证：生成 Hertz + Kitex 两个完整项目，确认可编译、可启动 | 3h |
| 5.2 | 更新 go-tools 各库的 README（go-common / go-middleware / go-framework） | 2h |
| 5.3 | 更新文档（README.md / README.zh-CN.md / docs/examples.md） | 2h |
| 5.4 | 更新文档中嵌入的设计文档（`internal/assets/_data/docs/`） | 2h |
| 5.5 | 补充迁移指南：已有项目如何从旧模板迁移到新模板 | 2h |
| 5.6 | Markdown 诊断（所有 .md 文件） | 0.5h |
| 5.7 | CI 验证：`go build ./... && go build . && go vet ./... && go test ./... -count=1 && ./scripts/smoke.sh` | 1h |

**验收**：
- 两个端到端项目可以编译并启动（不要求业务逻辑完整）
- 所有文档中英文对齐
- CI 全部通过

## 六、依赖关系

```text
Phase 1 (go-common)
    ↓
Phase 2 (go-middleware) ──→ 需要 go-common
    ↓
Phase 3 (go-framework)  ──→ 需要 go-common + go-middleware ✓ 已完成
    ↓
    ↓
Phase 5 (验证 + 文档)
```

Phase 1-3 **不可并行**（有依赖关系）。

## 七、风险矩阵

| 风险 | 概率 | 影响 | 缓解措施 |
|------|------|------|---------|
| Kafka 库切换（sarama → kafka-go）重写工作量大 | 高 | 高 | 已确认 D1，Phase 2 专门分配 4h 重写 Writer/Reader 工厂 |
| ES/ClickHouse 为全新中间件，反哺经验有限 | 中 | 中 | 参考 optional/clickhouse.go 和 optional/es.go 模板，优先保证基本连接能力 |
| go-tools 测试不全，搬迁后暴露 bug | 中 | 中 | Phase 1-3 每步都跑测试，提前发现 |
| 已有用户的项目 import 路径硬编码 | 低 | 低 | 旧路径保留 deprecated alias，给迁移时间 |
| oops 与 pkg/errors 迁移矛盾 | 低 | 中 | 已确认 D3 ✅ — oops 已全面替代 ErrorType 枚举 |

## 八、已确认关键决策

| # | 决策 | 影响范围 | 结论 | 状态 |
|---|------|---------|------|------|
| D1 | Kafka 库选择 | **kafka-go**，sarama 标记 deprecated | ✅ |
| D2 | 配置时间单位 | **time.Duration**（YAML 写 `30s`） | ✅ |
| D3 | 错误库 | **oops 为主** | ✅ |
| D4 | 发布策略 | **独立发版** | ✅ |
| D5 | 旧模块 | **已删除** | ✅ |

## 九、回滚计划

| 阶段 | 回滚方式 | 回滚时间 |
|------|---------|---------|
| Phase 1 (go-common) | 删除 go-common 仓库，go-tools 原 tools/ 目录未受影响 | < 5 分钟 |
| Phase 2 (go-middleware) | 删除 go-middleware 仓库，go-tools 原 middleware/ 目录仍在 | < 5 分钟 |
| Phase 3 (go-framework) | 删除 go-framework 仓库，go-tools 原 hertz/kitex/ 目录仍在 | < 5 分钟 |
| Phase 5 (验证文档) | 无需回滚（仅验证和文档） | — |
| 发布后发现 bug | go-tools 旧 import 路径在 deprecated 期仍可用，用户可 pin 旧版本 | — |
