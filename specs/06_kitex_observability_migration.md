# obs-opentelemetry 迁移评估与实施方案

> 日期：2026-06-25
> 状态：✅ 实施完成（Phase 1-4），Phase 5 验证+文档收尾

## 一、背景

[kitex-contrib/obs-opentelemetry](https://github.com/kitex-contrib/obs-opentelemetry) 是 Kitex 社区维护的 OpenTelemetry 扩展库（31 Stars, 22 Forks），为 Kitex 微服务提供 Tracing + Metrics + Logging 三大可观测性能力。

go-tools 当前已有基础的 OTel 集成（`go-framework/kitex/observability` 和 `go-framework/hertz/observability`），但功能覆盖度仅为 obs-opentelemetry 的 ~30%。本文档评估两者的能力差距，并制定渐进式迁移方案。

## 二、obs-opentelemetry 项目分析

### 2.1 项目结构

```text
obs-opentelemetry/
├── provider/              ← OTel Provider（Trace + Metrics 双通道）
│   ├── provider.go        ← NewOpenTelemetryProvider()
│   └── options.go         ← WithServiceName/WithExportEndpoint/...
├── tracing/               ← Kitex tracing 核心（~15 个文件）
│   ├── suite.go           ← NewServerSuite/NewClientSuite 入口
│   ├── middleware.go       ← ClientMiddleware/ServerMiddleware
│   ├── tracer_server.go   ← serverTracer（stats.Tracer 接口）
│   ├── tracer_client.go   ← clientTracer
│   ├── metrics.go         ← RPC metrics 语义约定 + RED 指标定义
│   ├── peer.go            ← Peer Service 自动传播
│   ├── propagator.go      ← TraceContext 注入/提取
│   ├── events.go          ← Span 事件注入
│   ├── semconv.go         ← 自定义语义约定常量
│   └── internal/context.go ← TraceCarrier 内部传递
└── logging/               ← Kitex 日志适配器
    ├── logrus/            ← logrus 适配器 + OTel hook
    ├── zap/               ← zap 适配器
    ├── slog/              ← slog 适配器
    └── zerolog/           ← zerolog 适配器
```

### 2.2 功能清单

| 模块 | 能力 | 实现方式 |
|------|------|---------|
| **Provider** | OTLP Trace 导出 | `otlptracegrpc` exporter + `sdktrace.NewTracerProvider` |
| **Provider** | OTLP Metrics 导出 | `otlpmetricgrpc` exporter + `metric.NewMeterProvider` |
| **Provider** | Go Runtime Metrics | `runtimemetrics.Start()` — goroutines/GC/memory |
| **Provider** | 多 Propagator | B3 + OT + Baggage + TraceContext 组合传播 |
| **Provider** | 环境变量配置 | `OTEL_EXPORTER_OTLP_ENDPOINT` 等 SDK 标准环境变量 |
| **Tracing** | Server stats.Tracer | 实现 Kitex `stats.Tracer` 接口（Start/Finish 生命周期） |
| **Tracing** | Client stats.Tracer | 同上，Client 端 |
| **Tracing** | RPC 元数据采集 | method, service, peer.service, transport protocol, send/recv size |
| **Tracing** | 错误记录 | RecordError + panic stack trace |
| **Tracing** | Peer Service 传播 | Client 端注入 service info → TTHeader → Server 端提取 |
| **Tracing** | gRPC Metadata | HTTP2/gRPC metadata 中的 trace context 传播 |
| **Tracing** | Suite 封装 | `NewServerSuite()` / `NewClientSuite()` 一键接入 |
| **Metrics** | RPC Duration | `rpc.server.duration` / `rpc.client.duration` Histogram |
| **Metrics** | RED 指标 | Rate / Errors / Duration（基于 PromQL 计算） |
| **Metrics** | 服务拓扑图 | `peer.service` 维度支持拓扑可视化 |
| **Logging** | logrus 适配 | Kitex logger + OTel trace_id/span_id 注入 |
| **Logging** | zap 适配 | 同上 |
| **Logging** | slog 适配 | 同上 |
| **Logging** | zerolog 适配 | 同上 |

### 2.3 关键依赖

| 依赖 | 版本 | 用途 |
|------|------|------|
| `go.opentelemetry.io/otel` | v1.19.0 | OTel 核心 API |
| `go.opentelemetry.io/otel/sdk/metric` | v1.19.0 | Metrics SDK |
| `go.opentelemetry.io/contrib/instrumentation/runtime` | v0.45.0 | Go runtime metrics |
| `github.com/cloudwego/kitex` | v0.7.3 | Kitex RPC 框架 |
| `github.com/bytedance/gopkg/cloud/metainfo` | — | TTHeader metadata 读写 |

## 三、go-tools 现有能力

### 3.1 已有实现

| 文件 | 能力 | 覆盖范围 |
|------|------|---------|
| `go-framework/kitex/observability/provider.go` | Kitex OTel Provider | Tracing only，简单 middleware |
| `go-framework/hertz/observability/provider.go` | Hertz OTel Provider | Tracing only，HTTP 中间件 + hertzCarrier |
| `go-framework/config/observability.go` | ObservabilityConfig | Enabled/Endpoint/ServiceName/SampleRate |
| `go-framework/config/observability_test.go` | Config 测试 | 基本 default/full 用例 |
| `go-common/log/logger.go` | slog 日志 + otelHandler | trace_id/span_id 自动注入 |
| `go-common/log/adapters/` | klog/hlog 适配器 | 将 slog 适配为 kitex/hertz logger |
| `go-framework/kitex/option/option.go` | Kitex Option 工厂 | 不含 OTel option |

### 3.2 架构设计特点

```text
go-tools observability 架构:
┌─────────────────────────────────────────┐
│ ObservabilityConfig (统一配置)           │
├──────────────────┬──────────────────────┤
│ kitex/observability │ hertz/observability │
│ Provider            │ Provider            │
│ - TracerProvider    │ - TracerProvider    │
│ - Middleware        │ - ServerMiddleware  │
│ - Shutdown          │ - Shutdown          │
└────────────────────┴─────────────────────┘
         │                      │
    ┌────▼────┐          ┌──────▼──────┐
    │  Kitex  │          │   Hertz     │
    │  RPC    │          │   HTTP      │
    └─────────┘          └─────────────┘
```

## 四、差距矩阵

### 4.1 能力对比总表

| # | 能力 | obs-opentelemetry | go-tools 现状 | 差距等级 | 迁移价值 |
|---|------|:---:|:---:|:---:|:---:|
| 1 | OTLP Trace 导出 | ✅ | ✅ | — | 已覆盖 |
| 2 | OTLP Metrics 导出 | ✅ | ❌ | 🔴 关键 | 极高 |
| 3 | Kitex stats.Tracer 接口 | ✅ | ❌ | 🔴 关键 | 极高 |
| 4 | RPC 元数据采集 | ✅ 全量 | ⚠️ 仅 `%T` | 🔴 关键 | 极高 |
| 5 | Peer Service 自动传播 | ✅ | ❌ | 🟡 重要 | 高 |
| 6 | Client 端 Tracing | ✅ | ❌ | 🟡 重要 | 高 |
| 7 | Suite 一键接入 | ✅ | ❌ | 🟢 改进 | 中 |
| 8 | 多 Propagator 支持 | ✅ B3+OT+Baggage+TC | ⚠️ 仅 TraceContext | 🟢 改进 | 中 |
| 9 | Go Runtime Metrics | ✅ | ❌ | 🟢 改进 | 中 |
| 10 | gRPC Metadata 传播 | ✅ | ❌ | 🟢 可选 | 低 |
| 11 | Error + Stack Trace | ✅ | ⚠️ 仅 err.Error() | 🟢 可选 | 低 |
| 12 | logrus 适配器 | ✅ | ❌ | — | 不需要 |
| 13 | zap 适配器 | ✅ | ❌ | — | 不需要 |
| 14 | slog 适配器 | ✅ | ✅ otelHandler | — | 已覆盖 |
| 15 | zerolog 适配器 | ✅ | ❌ | — | 不需要 |
| 16 | Hertz HTTP 可观测 | ❌ | ✅ | — | 独有优势 |

### 4.2 差距详解

#### 差距 #2：Metrics 通道（🔴 关键）

obs-opentelemetry 在 `provider.go` 中同时初始化双通道：

```go
// obs-opentelemetry 有，go-tools 没有:
metricExp, _ := otlpmetricgrpc.New(ctx, opts...)
meterProvider := metric.NewMeterProvider(
    metric.WithReader(metric.NewPeriodicReader(metricExp)),
)
otel.SetMeterProvider(meterProvider)
runtimemetrics.Start()  // Go runtime metrics
```

go-tools 当前只有 `otel.SetTracerProvider(tp)`，完全没有 metrics 导出通道。接入 Prometheus/Grafana 后无法看到 QPS、P99 延迟、错误率等指标。

#### 差距 #3：stats.Tracer 接口（🔴 关键）

obs-opentelemetry 使用 Kitex 的 `stats.Tracer` 接口，在 RPC 生命周期的关键节点注入逻辑：

```go
// obs-opentelemetry 的 serverTracer
type serverTracer struct { ... }

func (s *serverTracer) Start(ctx context.Context) context.Context {
    // 创建 TraceCarrier，注入 context
}

func (s *serverTracer) Finish(ctx context.Context) {
    ri := rpcinfo.GetRPCInfo(ctx)
    // 可访问:
    //   ri.To().Method()           → rpc 方法名
    //   ri.To().ServiceName()      → 服务名
    //   ri.Stats().RecvSize()      → 收包字节数
    //   ri.Stats().SendSize()      → 发包字节数
    //   ri.Config().TransportProtocol() → 传输协议
    //   ri.From().Method()         → 调用方方法
    //   ri.Stats().Level()         → stats 级别
    //   解析 error/panic           → RecordError
}
```

go-tools 当前仅使用简单 `endpoint.Middleware`：

```go
// go-tools 现状 — 信息极度有限
func (p *Provider) Middleware() func(...) {
    return func(ctx, req, resp) error {
        ctx, span := p.tracer.Start(ctx, "rpc")
        span.SetAttributes(
            attribute.String("rpc.method", fmt.Sprintf("%T", req)), // 只能拿到类型名
        )
    }
}
```

**信息量对比**：obs-opentelemetry 能采集 10+ 维度的 RPC 元数据，go-tools 只能拿到 Go 类型名字符串。

#### 差距 #5：Peer Service 传播（🟡 重要）

obs-opentelemetry 实现了完整的 peer service 自动发现：

```
Client 端 injectPeerServiceToMetaInfo():
  将自身 service.name / service.namespace / deployment.environment
  注入 TTHeader metadata

Server 端 extractPeerServiceAttributesFromMetaInfo():
  从 TTHeader metadata 提取上游信息
  → span.SetAttributes(peer.service=xxx, peer.namespace=xxx)
  → metrics 中记录 peer.service 维度
```

这使得可以在 Grafana 中渲染服务拓扑图：
```promql
sum(rate(rpc_server_duration_count{}[5m])) by (service_name, peer_service)
```

go-tools 完全没有此能力，无法知道调用方是谁。

#### 差距 #6：Client 端 Tracing（🟡 重要）

go-tools 的 `kitex/observability` 只实现了 server 端 middleware，Client 端完全没有 tracing。obs-opentelemetry 有完整的 `clientTracer` + `ClientMiddleware`。

## 五、迁移策略

### 5.1 核心原则

1. **增量增强，而非整体搬迁** — obs-opentelemetry 和 go-tools 的架构理念不同（前者 Kitex-only + 多日志库，后者 Hertz+Kitex 双框架 + 统一 slog）
2. **保持现有 API 兼容** — `ObservabilityConfig`、`Provider` 接口不破坏
3. **保持 go-tools 独特优势** — Hertz observability 继续强化，不因迁移而退化
4. **遵循项目规范** — Options 模式、godoc 注释、`golangci-lint` 全通过

### 5.2 迁移范围决策

| 组件 | 决策 | 原因 |
|------|:---:|------|
| provider/ metrics 通道 | ✅ 融入 | 核心缺口，补齐可观测性闭环 |
| tracing/ stats.Tracer | ✅ 融入 | 元数据采集量从 1 维 → 10+ 维 |
| tracing/ peer service | ✅ 融入 | 服务拓扑图基础 |
| tracing/ client tracer | ✅ 融入 | 补齐 Client 端可观测空白 |
| tracing/ Suite | ✅ 融入 | 降低接入成本 |
| provider/ 多 Propagator | ✅ 融入 | 兼容更多 trace 系统（如 Istio） |
| provider/ runtime metrics | ✅ 融入 | 低成本高价值 |
| tracing/ gRPC metadata | ❌ 暂缓 | go-tools 主用 TTHeader，非必须 |
| logging/ logrus | ❌ 不迁 | go-tools 统一 slog |
| logging/ zap | ❌ 不迁 | 同上 |
| logging/ zerolog | ❌ 不迁 | 同上 |
| logging/ slog | ❌ 不迁 | `go-common/log/otelHandler` 等价 |
| hertz/observability | ✅ 保留 + 增强 | go-tools 独有，后续也加 metrics |

### 5.3 不迁移的组件详细说明

**Logging 适配器（logrus/zap/zerolog/slog）**：

go-tools 的 `go-common/log` 已通过 `otelHandler` 实现了等价功能：
```go
// go-tools 的 otelHandler（logger.go:259-281）
func (h *otelHandler) Handle(ctx context.Context, r slog.Record) error {
    span := trace.SpanFromContext(ctx)
    if span.SpanContext().IsValid() {
        r.AddAttrs(
            slog.String("trace_id", span.SpanContext().TraceID().String()),
            slog.String("span_id", span.SpanContext().SpanID().String()),
        )
    }
    return h.next.Handle(ctx, r)
}
```

obs-opentelemetry 的 logging/slog 本质上做的是同一件事，只是多了一层 `CtxDebugf(ctx, ...)` 风格的封装。go-tools 通过 `go-common/log/adapters/` 的 klog/hlog 适配器已覆盖此场景，无需引入额外日志抽象层。

## 六、实施方案

### 6.1 总览

```text
Phase 1: Metrics 通道补齐               ██████░░░░ 1-2 天
Phase 2: stats.Tracer 深度集成           ██████████ 2-3 天
Phase 3: Peer Service + Client Tracing  ██████░░░░ 1-2 天
Phase 4: 多 Propagator + Runtime        ████░░░░░░ 0.5-1 天
Phase 5: 验证 + 文档                    ██████░░░░ 1 天
─────────────────────────────────────────────────────
合计: ~5-8 个工作日
```

每个 Phase 独立可测试、可合入，不阻塞后续 Phase。

### 6.2 Phase 1：Metrics 通道补齐（1-2 天）

#### 目标

在现有 Provider 中新增 OTLP Metrics 导出通道，使 go-tools 具备完整的 Tracing + Metrics 双通道能力。

#### 改动范围

| 文件 | 操作 | 说明 |
|------|------|------|
| `go-framework/config/observability.go` | 修改 | 新增 `EnableMetrics`, `MetricsInterval` 字段 |
| `go-framework/kitex/observability/provider.go` | 修改 | 新增 MeterProvider 初始化 + runtime metrics |
| `go-framework/hertz/observability/provider.go` | 修改 | 同上 |
| `go-framework/kitex/observability/provider_test.go` | 新增 | Provider 测试（含 metrics） |
| `go-framework/hertz/observability/provider_test.go` | 新增 | 同上 |

#### Config 变更

```go
// ObservabilityConfig 可观测性配置。
type ObservabilityConfig struct {
    // Enabled 是否启用可观测性
    Enabled bool `json:"enabled" yaml:"enabled"`

    // Endpoint OTLP Collector 地址（如 otelcol:4317）
    Endpoint string `json:"endpoint" yaml:"endpoint"`

    // ServiceName 服务名称
    ServiceName string `json:"service_name" yaml:"service_name"`

    // SampleRate 采样率（0.0-1.0，默认 1.0）
    SampleRate float64 `json:"sample_rate" yaml:"sample_rate"`

    // ── 新增字段 ──

    // EnableMetrics 是否启用 Metrics 导出（默认 true，当 Enabled=true 时生效）
    EnableMetrics bool `json:"enable_metrics" yaml:"enable_metrics"`

    // MetricsInterval Metrics 上报间隔（默认 15s）
    MetricsInterval time.Duration `json:"metrics_interval" yaml:"metrics_interval"`
}
```

#### Provider 改造要点

```go
func NewProvider(ctx context.Context, cfg config.ObservabilityConfig) (*Provider, error) {
    // ... 现有的 Trace 初始化逻辑保持不变 ...

    // 新增: Metrics 初始化
    if cfg.EnableMetrics {
        metricExp, err := otlpmetricgrpc.New(ctx,
            otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
            otlpmetricgrpc.WithInsecure(),
        )
        if err != nil {
            return nil, fmt.Errorf("observability: create metric exporter: %w", err)
        }

        interval := cfg.MetricsInterval
        if interval <= 0 {
            interval = 15 * time.Second
        }

        reader := metric.NewPeriodicReader(metricExp, metric.WithInterval(interval))
        meterProvider := metric.NewMeterProvider(
            metric.WithReader(reader),
            metric.WithResource(res),
        )
        otel.SetMeterProvider(meterProvider)
        p.meterProvider = meterProvider

        // Go runtime metrics
        if err := runtimemetrics.Start(runtimemetrics.WithMeterProvider(meterProvider)); err != nil {
            return nil, fmt.Errorf("observability: start runtime metrics: %w", err)
        }
    }

    return p, nil
}
```

#### 验收标准

- `go test ./go-framework/... -count=1` 通过
- `golangci-lint run --timeout=5m ./go-framework/...` 通过
- Provider 创建后可同时看到 `TracerProvider` 和 `MeterProvider` 被正确设置

### 6.3 Phase 2：stats.Tracer 深度集成（2-3 天）

#### 目标

将 Kitex tracing 从简单 `endpoint.Middleware` 升级为 `stats.Tracer` 接口实现，大幅提升 RPC 元数据采集的丰富度。

#### 架构对比

```
Phase 2 前 (go-tools 现状):
  Request → Middleware(ctx, req, resp) → 只能拿到 req 类型名

Phase 2 后 (对齐 obs-opentelemetry):
  Request → stats.Tracer.Start(ctx) → Middleware(span 创建) → Handler
          → stats.Tracer.Finish(ctx) → 拿到完整 RPCInfo:
            - rpc.method, rpc.service, rpc.system
            - peer.service, peer.namespace
            - transport protocol
            - send/recv size (bytes)
            - status_code (OK/Error)
            - error detail + stack trace
            - duration (ms)
```

#### 新增文件

| 文件 | 说明 |
|------|------|
| `go-framework/kitex/observability/tracer.go` | serverTracer 实现 `stats.Tracer` |
| `go-framework/kitex/observability/tracer_test.go` | tracer 单元测试 |
| `go-framework/kitex/observability/metrics.go` | RPC metrics 定义 (`rpc.server.duration` Histogram) |
| `go-framework/kitex/observability/suite.go` | `NewServerSuite()` / `NewClientSuite()` |
| `go-framework/kitex/observability/semconv.go` | 自定义语义约定常量 |

#### 核心实现

```go
// tracer.go — serverTracer 实现 stats.Tracer 接口

// serverTracer Kitex 服务端 OTel Tracer，实现 stats.Tracer 接口。
type serverTracer struct {
    cfg              config.ObservabilityConfig
    tracer           trace.Tracer
    meter            metric.Meter
    serverDuration   metric.Float64Histogram
}

// Start 在 RPC 开始时创建 TraceCarrier 并注入 context。
func (s *serverTracer) Start(ctx context.Context) context.Context {
    tc := &traceCarrier{}
    tc.SetTracer(s.tracer)
    return withTraceCarrier(ctx, tc)
}

// Finish 在 RPC 结束时记录 span 属性和 metrics。
func (s *serverTracer) Finish(ctx context.Context) {
    tc := traceCarrierFromContext(ctx)
    if tc == nil {
        return
    }

    ri := rpcinfo.GetRPCInfo(ctx)
    if ri.Stats().Level() == stats.LevelDisabled {
        return
    }

    st := ri.Stats()
    rpcStart := st.GetEvent(stats.RPCStart)
    rpcFinish := st.GetEvent(stats.RPCFinish)
    duration := float64(rpcFinish.Time().Sub(rpcStart.Time())) / float64(time.Millisecond)

    span := tc.Span()
    if span == nil || !span.IsRecording() {
        return
    }

    // 记录丰富的 RPC 属性
    span.SetAttributes(
        semconv.RPCMethodKey.String(ri.To().Method()),
        semconv.RPCServiceKey.String(ri.To().ServiceName()),
        semconv.RPCSystemKey.String("kitex"),
        attribute.Int64("rpc.kitex.recv_size", int64(st.RecvSize())),
        attribute.Int64("rpc.kitex.send_size", int64(st.SendSize())),
        attribute.String("rpc.transport", ri.Config().TransportProtocol().String()),
    )

    // 错误处理
    if rpcErr, panicMsg, panicStack := parseRPCError(ri); rpcErr != nil || panicMsg != "" {
        recordErrorSpanWithStack(span, rpcErr, panicMsg, panicStack)
    }

    span.End(trace.WithTimestamp(getEndTimeOrNow(ri)))

    // 记录 metrics
    metricAttrs := extractMetricsAttributes(span)
    s.serverDuration.Record(ctx, duration, metric.WithAttributes(metricAttrs...))
}
```

#### 改动文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `go-framework/kitex/observability/provider.go` | 修改 | Provider 新增 `ServerSuite()` / `ClientSuite()` 方法 |
| `go-framework/kitex/observability/tracer.go` | 新增 | serverTracer + clientTracer |
| `go-framework/kitex/observability/metrics.go` | 新增 | RPC duration histogram 定义 |
| `go-framework/kitex/observability/suite.go` | 新增 | Suite 封装 |
| `go-framework/kitex/observability/semconv.go` | 新增 | 自定义 attribute keys |
| `go-framework/kitex/observability/propagator.go` | 新增 | TraceContext 注入/提取 |
| `go-framework/kitex/observability/internal.go` | 新增 | TraceCarrier context 传递 |

#### 验收标准

- `go test ./go-framework/kitex/observability/... -count=1` 通过
- tracer 能正确记录 RPC method/service/duration/status 等属性
- span.End() 和 metrics.Record() 均在 Finish 中正确调用

### 6.4 Phase 3：Peer Service + Client Tracing（1-2 天）

#### 目标

实现服务拓扑图所需的 Peer Service 自动传播能力，并补齐 Client 端 tracing。

#### 新增/修改文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `go-framework/kitex/observability/peer.go` | 新增 | Peer service 注入/提取逻辑 |
| `go-framework/kitex/observability/peer_test.go` | 新增 | Peer service 单元测试 |
| `go-framework/kitex/observability/tracer.go` | 修改 | 增加 ClientMiddleware peer 注入 |

#### 核心逻辑

```
Client 端（注入自身信息到 metadata）:
  injectPeerServiceToMetaInfo():
    读取 resource attributes (service.name, service.namespace, deployment.environment)
    → 写入 metainfo (TTHeader metadata)
    → 下游 Server 端可读取

Server 端（提取上游信息）:
  extractPeerServiceAttributesFromMetaInfo():
    从 metainfo 读取上游 service.name → span.SetAttributes(peer.service=xxx)
    从 metainfo 读取上游 service.namespace → span.SetAttributes(peer.namespace=xxx)
    → metrics 中携带 peer.service 维度 → Grafana 服务拓扑图
```

#### 验收标准

- `go test ./go-framework/kitex/observability/... -count=1` 通过  
- Server span 中包含 `peer.service` 属性
- Metrics 中包含 `peer_service` label

### 6.5 Phase 4：多 Propagator + Runtime Metrics（0.5-1 天）

#### 目标

- Provider 支持多 Propagator 组合（B3 + TraceContext），兼容 Istio/Envoy 等 sidecar
- Runtime metrics 已在 Phase 1 引入，本 Phase 确认可配置化

#### 改动文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `go-framework/kitex/observability/provider.go` | 修改 | 默认 propagator 改为复合传播器 |

```go
// 默认传播器改为组合模式（对齐 obs-opentelemetry）
otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
    b3.New(),                    // B3 格式（Zipkin/Istio）
    propagation.TraceContext{},  // W3C TraceContext
    propagation.Baggage{},       // Baggage 传播
))
```

#### 验收标准

- 支持 W3C TraceContext 和 B3 两种格式的 trace header 解析
- Runtime metrics 可正常上报到 OTLP Collector

### 6.6 Phase 5：验证 + 文档（1 天）

#### 任务清单

| # | 任务 | 工作量 |
|---|------|--------|
| 5.1 | 全量测试 `go test ./go-common/... ./go-middleware/... ./go-framework/... -count=1` | 0.5h |
| 5.2 | golangci-lint 全量检查 | 0.5h |
| 5.3 | `go-framework/kitex/observability/` 包级 godoc 文档 | 1h |
| 5.4 | 更新 `CLAUDE.md` 中的模块说明 | 0.5h |
| 5.5 | 更新 `specs/00_overview.md` 中的架构图 | 0.5h |
| 5.6 | 端到端验证（本地 OTLP Collector + Jaeger + Prometheus） | 2h |

## 七、文件级改动汇总

### 7.1 新增文件

```text
go-framework/kitex/observability/
├── tracer.go            ← serverTracer + clientTracer（stats.Tracer 实现）
├── tracer_test.go       ← tracer 单元测试
├── metrics.go           ← RPC metrics 定义 + extractMetricsAttributes
├── suite.go             ← NewServerSuite / NewClientSuite
├── semconv.go           ← 自定义 attribute keys
├── propagator.go        ← TraceContext 注入/提取
├── internal.go          ← TraceCarrier context 传递
├── peer.go              ← Peer service 注入/提取
└── peer_test.go         ← Peer service 测试
```

### 7.2 修改文件

```text
go-framework/config/observability.go         ← 新增 EnableMetrics/MetricsInterval
go-framework/config/observability_test.go    ← 新增 metrics 配置测试
go-framework/kitex/observability/provider.go ← 新增 MeterProvider + runtime metrics + 多 propagator
go-framework/hertz/observability/provider.go ← 新增 MeterProvider（hertz 侧同步）
go-framework/kitex/option/option.go          ← 集成 OTel Suite
```

### 7.3 不修改文件

```text
go-common/log/logger.go        ← otelHandler 已满足需求，不变
go-common/log/adapters/        ← klog/hlog 适配器已满足需求，不变
go-middleware/redis/client.go  ← 已有 OTel tracing hook，不变
```

## 八、依赖变更

### 8.1 新增依赖

```
go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc
go.opentelemetry.io/contrib/instrumentation/runtime
go.opentelemetry.io/contrib/propagators/b3
go.opentelemetry.io/contrib/propagators/ot
```

### 8.2 已存在（无需新增）

```
go.opentelemetry.io/otel                                      ← 已有
go.opentelemetry.io/otel/trace                                ← 已有
go.opentelemetry.io/otel/metric                               ← 已有（间接依赖）
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc ← 已有
go.opentelemetry.io/otel/sdk/resource                         ← 已有
go.opentelemetry.io/otel/sdk/trace                            ← 已有
```

## 九、风险评估

| 风险 | 等级 | 影响 | 缓解措施 |
|------|:---:|------|------|
| otel SDK 版本升级导致 API 不兼容 | 低 | go-tools 已用 semconv v1.26/36，obs-opentelemetry 用 v1.12 | 直接沿用现有版本，不降级 |
| `stats.Tracer` 接口在 Kitex 版本间变化 | 低 | stats.Tracer 是 Kitex 长期稳定接口 | Phase 2 前确认 kitex 版本兼容性 |
| 新增依赖导致 go.sum 膨胀 | 低 | 约 3-5 个新包 | 增量可控 |
| tracer.go 逻辑复杂导致 bug | 中 | 事件解析、状态码映射、error/panic 处理 | 重点覆盖单元测试 |
| Hertz 和 Kitex provider 代码重复 | 中 | 两个 provider 结构高度相似 | Phase 2-3 后评估是否抽取公共 base provider |
| gRPC metadata 依赖 `nphttp2` 内部包 | 低 | gRPC metadata 是可选能力 | Phase 3 明确不迁移 gRPC metadata 部分 |

## 十、决策记录

| # | 决策 | 结论 | 日期 |
|---|------|------|------|
| D6 | 是否整体搬迁 obs-opentelemetry | **否**，增量增强现有架构 | 2026-06-24 |
| D7 | Metrics 通道是否独立可关闭 | **是**，`EnableMetrics` 字段控制 | 2026-06-24 |
| D8 | 是否保留现有 Middleware | **是**，stats.Tracer 作为增强路径，Middleware 保持兼容 | 2026-06-24 |
| D9 | logging 适配器是否迁移 | **否**，go-tools 的 otelHandler 已等价覆盖 | 2026-06-24 |
| D10 | gRPC metadata 传播是否迁移 | **否**，go-tools 主用 TTHeader，暂不需要 | 2026-06-24 |
| D11 | go-framework vs go-middleware 归属 | **go-framework**，observability 是框架适配层 | 2026-06-24 |

## 十一、后续规划

完成 Phase 1-5 后，可进一步考虑：

1. **Hertz observability 增强**：给 Hertz provider 也加上 metrics 通道（`http.server.duration` histogram）
2. **公共 base provider 抽取**：Kitex 和 Hertz provider 的 TracerProvider/MeterProvider 初始化逻辑高度相似，可抽取 `go-framework/observability/` 公共包
3. **Config 热更新**：支持运行时通过 `Update()` 方法调整采样率
4. **ncgo 模板对齐**：确保 ncgo 生成的 `main.go` 默认集成 Suite 模式
