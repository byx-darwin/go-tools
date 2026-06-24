# hertz-contrib/obs-opentelemetry 迁移评估与实施方案

> 日期：2026-06-25
> 状态：✅ 实施完成（Phase H1-H2）
> 来源：https://github.com/hertz-contrib/obs-opentelemetry

## 一、背景

[hertz-contrib/obs-opentelemetry](https://github.com/hertz-contrib/obs-opentelemetry) 是 Hertz 社区维护的 OpenTelemetry 扩展库（37 Stars, 29 Forks），为 Hertz HTTP 服务提供 Tracing + Metrics + Logging 三大可观测性能力。

go-tools 已在 Phase 1-4（参见 `specs/06_observability_migration.md`）中完成了 Kitex 侧的全面升级，并同步补齐了 Hertz 侧的 Provider 基础设施（MeterProvider、复合 Propagator、Runtime Metrics）。但 Hertz 侧仍缺少**HTTP 级别的 Metrics 采集**和 **tracer.Tracer 深度集成**，本评估量化剩余差距。

## 二、hertz-contrib/obs-opentelemetry 项目分析

### 2.1 项目结构

```text
obs-opentelemetry/
├── provider/              ← OTel Provider（Trace + Metrics）
│   ├── provider.go        ← NewOpenTelemetryProvider()
│   └── options.go
├── tracing/               ← Hertz tracing 核心（~15 文件）
│   ├── tracer_server.go   ← serverTracer（实现 tracer.Tracer 接口）
│   ├── middleware.go       ← ServerMiddleware + ClientMiddleware
│   ├── metrics.go         ← HTTP metrics 定义 + extractMetricsAttributes
│   ├── peer.go            ← Peer service 通过 HTTP header 传播
│   ├── propagator.go      ← TraceContext 注入/提取（HTTP header carrier）
│   ├── semconv.go         ← HTTP 特定 attribute keys
│   ├── options.go         ← Config + Options（span name formatter 等）
│   ├── events.go          ← Stats event 注入
│   └── internal/context.go
└── logging/               ← Hertz 日志适配器
    ├── logrus/            ← logrus + OTel hook
    ├── zap/               ← zap 适配器
    └── zerolog/           ← zerolog 适配器
```

### 2.2 功能清单

| 模块 | 能力 | 实现方式 |
|------|------|---------|
| **Provider** | OTLP Trace + Metrics 导出 | 与 go-tools 现有 Provider 等价 |
| **Tracing** | Server tracer.Tracer | `server.WithTracer(st)` — Start/Finish 生命周期 |
| **Tracing** | ServerMiddleware | `app.HandlerFunc`，提取 trace context + peer service |
| **Tracing** | ClientMiddleware | `client.Middleware`，注入 trace context + peer service |
| **Tracing** | HTTP Span Attributes | `semconv.NetAttributesFromHTTPRequest` 等标准属性 |
| **Tracing** | Peer Service | HTTP header `service-name` / `service-namespace` / `deployment-environment` |
| **Metrics** | `http.server.duration` | Float64Histogram |
| **Metrics** | `http.server.request_count` | Int64Counter |
| **Metrics** | `http.client.duration` | Float64Histogram |
| **Metrics** | `http.client.request_count` | Int64Counter |
| **Options** | ShouldIgnore | 条件跳过 tracing |
| **Options** | Span name formatter | 自定义 span 命名 |
| **Options** | CustomResponseHandler | 自定义响应处理 |

## 三、go-tools Hertz 现有能力

| 能力 | 状态 | 说明 |
|------|:---:|------|
| OTLP Trace 导出 | ✅ | `otlptracegrpc` |
| OTLP Metrics 导出 | ✅ Phase 1 | `otlpmetricgrpc` + `MeterProvider` |
| 复合 Propagator | ✅ Phase 4 | B3 + TraceContext + Baggage |
| Go Runtime Metrics | ✅ Phase 4 | goroutines/GC/memory 等 11 个指标 |
| ServerMiddleware | ✅ | `app.HandlerFunc`，trace context 提取 |
| HTTP Span Attributes | ⚠️ 基础 | 仅 method/path/status_code，缺 OTel 标准属性 |
| HTTP Metrics 采集 | ❌ | MeterProvider 就绪但未记录 HTTP metrics |
| ClientMiddleware | ❌ | 无客户端 HTTP tracing |
| tracer.Tracer 接口 | ❌ | 使用 middleware 而非 tracer 接口 |
| Peer Service（HTTP） | ❌ | 无 HTTP header 对端服务传播 |

## 四、差距矩阵

| # | 能力 | hertz-obs-otel | go-tools | 差距等级 | 迁移价值 |
|---|------|:---:|:---:|:---:|:---:|
| 1 | HTTP Metrics (`http.server.duration`) | ✅ | ❌ | 🔴 关键 | 极高 |
| 2 | HTTP Metrics (`http.server.request_count`) | ✅ | ❌ | 🔴 关键 | 极高 |
| 3 | HTTP Client Metrics | ✅ | ❌ | 🟡 重要 | 高 |
| 4 | tracer.Tracer 接口集成 | ✅ | ❌ | 🟡 重要 | 高 |
| 5 | HTTP 标准 Span Attributes | ✅ | ⚠️ | 🟡 重要 | 中 |
| 6 | Peer Service (HTTP headers) | ✅ | ❌ | 🟢 改进 | 中 |
| 7 | ClientMiddleware | ✅ | ❌ | 🟢 改进 | 中 |
| 8 | ShouldIgnore | ✅ | ❌ | 🟢 可选 | 低 |
| 9 | Span name formatter | ✅ | ❌ | 🟢 可选 | 低 |
| 10 | logrus/zap/zerolog 适配器 | ✅ | ❌ | — | 不需要 |

### 差距详解

#### 🔴 差距 1-2：HTTP Metrics（最关键）

obs-opentelemetry 在 `tracer_server.go::createMeasures()` 中定义了两个 HTTP Server metrics：

```go
// go-tools 目前完全没有这些：
serverRequestCountMeasure, _ := meter.Int64Counter(
    "http.server.request_count",
    metric.WithUnit("count"),
)
serverLatencyMeasure, _ := meter.Float64Histogram(
    "http.server.duration",
    metric.WithUnit("ms"),
)
```

对应的 Client metrics：
```go
clientRequestCountMeasure, _ := meter.Int64Counter("http.client.request_count", ...)
clientLatencyMeasure, _ := meter.Float64Histogram("http.client.duration", ...)
```

go-tools 的 Hertz Provider 已经在 `NewProvider()` 中创建了 `MeterProvider` 并注册到全局，但没有在任何地方创建这些 HTTP metrics 或调用 `Record()`/`Add()`。MeterProvider 只是个空壳。

#### 🟡 差距 4：tracer.Tracer 接口

obs-opentelemetry 使用 Hertz 原生的 `tracer.Tracer` 接口：

```go
server.WithTracer(st)  // st 实现 tracer.Tracer

// Start: 创建 TraceCarrier
func (s *serverTracer) Start(ctx context.Context, c *app.RequestContext) context.Context

// Finish: 记录 HTTP semconv 属性 + HTTP metrics
func (s *serverTracer) Finish(ctx context.Context, c *app.RequestContext)
```

go-tools 使用 `app.HandlerFunc` 中间件，缺少 `Finish` 中能获取的 `TraceInfo.Stats()` 事件（HTTPStart/HTTPFinish），无法精确测量耗时。且缺少 `adaptor.GetCompatRequest` 获取的标准 HTTP 属性。

#### 🟡 差距 6：Peer Service（HTTP Headers）

与 Kitex 不同，Hertz 通过 HTTP Headers 传播 peer service 信息（而非 metainfo）：

```go
// Client 端：将 semconv key 中的 "." 替换为 "-"
func semconvAttributeKeyToHTTPHeader(key string) string {
    return strings.ReplaceAll(key, ".", "-")
}
// → service.name 变为 service-name HTTP header
```

Server 端从 `c.Request.Header` 中读取这些 header 并提取 peer service 属性。

## 五、迁移策略

### 5.1 已有基础（无需迁移）

go-tools 在 Phase 1-4 中已完成以下 Hertz Provider 基础能力，**与 obs-opentelemetry 的 provider 包等价，无需重复迁移**：

- ✅ OTLP Trace + Metrics 导出
- ✅ 复合 Propagator（B3 + TraceContext + Baggage）
- ✅ Go Runtime Metrics（11 个指标）
- ✅ 统一 `ObservabilityConfig`

### 5.2 迁移范围决策

| 组件 | 决策 | 原因 |
|------|:---:|------|
| HTTP Metrics 采集 | ✅ 融入 | 核心缺口，无 metrics 等于 observability 只做了 30% |
| tracer.Tracer 接口 | ✅ 融入 | 精确耗时 + OTel 标准 HTTP 属性 |
| ClientMiddleware | ✅ 融入 | 补齐客户端 HTTP 可观测 |
| Peer Service（HTTP） | ✅ 融入 | HTTP 服务拓扑图基础 |
| ShouldIgnore | ✅ 融入 | 低成本高价值 |
| Span name formatter | ✅ 融入 | 灵活性提升 |
| logrus/zap/zerolog | ❌ 不迁 | go-tools 统一 slog + otelHandler |
| provider/ | ❌ 不迁 | 已等价覆盖 |

### 5.3 与 Kitex 迁移的差异

| 维度 | Kitex 迁移 | Hertz 迁移 |
|------|-----------|-----------|
| Provider 基础设施 | 需从零搭建 | ✅ 已有（Phase 1-4 同步） |
| Metrics 定义 | `rpc.server.duration` | `http.server.duration` + `request_count` |
| 核心接口 | `stats.Tracer` | `tracer.Tracer` |
| Peer 传播 | metainfo / TTHeader | HTTP Headers |
| 预估工时 | 5-8 天（4 Phases） | **1.5-2.5 天**（2 Phases） |

## 六、实施方案

### 6.1 总览

```text
Phase H1: HTTP Metrics + tracer.Tracer 集成   ██████████ 1-1.5 天
Phase H2: ClientMiddleware + Peer Service     ██████░░░░ 0.5-1 天
────────────────────────────────────────────────────────
合计: ~1.5-2.5 天（约 Kitex 迁移的 30% 工时）
```

### 6.2 Phase H1：HTTP Metrics + tracer.Tracer（1-1.5 天）

#### 新增文件

| 文件 | 说明 |
|------|------|
| `go-framework/hertz/observability/tracer.go` | `serverTracer` 实现 `tracer.Tracer` + HTTP metrics 定义 |
| `go-framework/hertz/observability/metrics.go` | `extractMetricsAttributesFromSpan` + HTTP metric 常量 |
| `go-framework/hertz/observability/semconv.go` | HTTP 特定 attribute keys |

#### 修改文件

| 文件 | 说明 |
|------|------|
| `go-framework/hertz/observability/provider.go` | 新增 `ServerTracer()` 方法；ServerMiddleware 升级 |

#### 核心实现

```go
// tracer.go
type serverTracer struct {
    cfg     config.ObservabilityConfig
    tracer  trace.Tracer
    meter   metric.Meter
    counters map[string]metric.Int64Counter
    histograms map[string]metric.Float64Histogram
}

func (s *serverTracer) Start(ctx context.Context, c *app.RequestContext) context.Context { ... }
func (s *serverTracer) Finish(ctx context.Context, c *app.RequestContext) {
    // 从 TraceCarrier 获取 span
    // 通过 adaptor.GetCompatRequest 设置 OTel 标准 HTTP 属性
    // 记录 http.server.request_count +1
    // 记录 http.server.duration
    // 注入 stats events
    // 错误/panic 处理
}
```

### 6.3 Phase H2：ClientMiddleware + Peer Service（0.5-1 天）

#### 新增文件

| 文件 | 说明 |
|------|------|
| `go-framework/hertz/observability/client.go` | `ClientMiddleware()` + client metrics |
| `go-framework/hertz/observability/peer.go` | HTTP header peer service 注入/提取 |

#### 核心实现

```go
// client.go
func ClientMiddleware(cfg config.ObservabilityConfig) client.Middleware {
    // 创建 client metrics (http.client.duration + http.client.request_count)
    // 注入 trace context 到 HTTP headers
    // 注入 peer service 到 HTTP headers
    // 返回 client.Endpoint
}

// peer.go
func injectPeerServiceToHTTPHeaders(attrs []attribute.KeyValue) map[string]string {
    // service.name → header "service-name"
}
func extractPeerServiceAttributesFromHTTPHeaders(headers *protocol.RequestHeader) []attribute.KeyValue {
    // header "service-name" → peer.service
}
```

## 七、文件级改动汇总

### 新增文件

```text
go-framework/hertz/observability/
├── tracer.go            ← serverTracer (tracer.Tracer) + HTTP metrics
├── tracer_test.go       ← tracer 单元测试
├── metrics.go           ← HTTP metrics 常量 + extractMetricsAttributes
├── semconv.go           ← HTTP 特定 attribute keys
├── client.go            ← ClientMiddleware + client metrics
├── client_test.go       ← client middleware 测试
├── peer.go              ← HTTP header peer service 注入/提取
└── peer_test.go         ← peer service 测试
```

### 修改文件

```text
go-framework/hertz/observability/provider.go  ← 新增 ServerTracer() + 升级 ServerMiddleware
go-framework/config/observability.go          ← 可能新增 ShouldIgnore 配置
```

## 八、风险评估

| 风险 | 等级 | 缓解 |
|------|:---:|------|
| Hertz `tracer.Tracer` API 稳定性 | 低 | 长期稳定接口 |
| `adaptor.GetCompatRequest` 依赖 | 低 | Hertz 标准包 |
| 与现有 Kitex observability 重复 | 低 | 各自独立，共享 Provider 基础设施 |

## 九、决策记录

| # | 决策 | 结论 | 日期 |
|---|------|------|------|
| D12 | 是否整体搬迁 hertz-obs-opentelemetry | **否**，增量增强现有 Provider | 2026-06-25 |
| D13 | HTTP Metrics 是否作为独立 Phase | **是**，Phase H1 | 2026-06-25 |
| D14 | logrus/zap/zerolog 是否迁移 | **否**，go-tools 统一 slog | 2026-06-25 |
| D15 | provider/ 包是否迁移 | **否**，go-tools 已等价覆盖 | 2026-06-25 |

## 十、后续规划

完成 Phase H1-H2 后：

1. **ncgo 模板对齐**：确保 ncgo 生成的 Hertz 项目默认集成 `ServerTracer` + `ClientMiddleware`
2. **全量可观测性测试**：Kitex + Hertz 联合场景端到端验证
3. **Grafana Dashboard 模板**：提供 RED + 拓扑图 JSON 模板
