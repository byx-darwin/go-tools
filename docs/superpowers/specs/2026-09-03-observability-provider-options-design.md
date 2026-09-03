# Observability Provider Options 设计（Issue #96）

## 背景

`go-framework/hertz/observability/provider.go` 与 `go-framework/kitex/observability/provider.go`
中的 `NewProvider` 硬编码 OTLP gRPC exporter 为明文传输
（`otlptracegrpc.WithInsecure()` / `otlpmetricgrpc.WithInsecure()`），且
sampler（`TraceIDRatioBased`）、propagator（`b3 + tracecontext + baggage`）均写死，
`NewProvider` 无任何 `...Option` 参数，用户无法替换实现，也无法在测试中注入可控的
exporter 进行断言。

**安全影响**：遥测数据（可能含请求路径、错误详情等敏感上下文）默认经公网/内网明文传输。

**来源**：多角色框架评审（安全 + 可扩展性视角均独立发现），评估日期 2026-09-02。

## 范围收敛说明

`go-framework/hertz/observability` 与 `go-framework/kitex/observability` 的
`tracer.go` / `client.go` / `suite.go` / `grpc_metadata.go` 等下游文件广泛依赖
`otel.GetTracerProvider()` / `otel.GetMeterProvider()` / `otel.GetTextMapPropagator()`
**全局单例**读取 Provider，而不是从 `Provider` 实例注入。彻底解决"测试中无法隔离"
需要重构这些下游读取点，改动会扩散到两个包的几乎所有文件，属于比本 Issue 更大范围的
架构问题。

**本次决策（已与用户确认）**：范围收敛为仅给 `NewProvider` 增加构造期 Options，
`otel.Set*` 全局设置行为保持不变、下游读取方式不变。全局单例的彻底重构如有需要，
另开 Issue 跟踪。

"测试隔离"在本次范围内体现为：单测可通过 `WithTraceExporter` / `WithMetricExporter`
注入内存 exporter，验证 span/metric 内容而不依赖真实网络连接；但多个 `Provider` 实例
之间仍共享全局 `TracerProvider`（后创建的会覆盖先创建的全局态），这一限制保持现状，
在 godoc 中注明。

## 设计决策

### 1. TLS 默认行为（行为变更，已与用户确认接受）

- `config.ObservabilityConfig` 新增字段 `Insecure bool`（yaml: `insecure`）
- **零值 `false` = 默认走 TLS**（系统根证书池），这是一次行为语义变更：
  升级本库后，未显式配置 `insecure: true` 的用户，exporter 连接方式会从明文变为
  TLS。若目标 collector 未启用 TLS，连接将失败。
- **迁移说明**（需在 CHANGELOG / 迁移文档中提示）：
  > 升级后 OTLP gRPC exporter 默认改为 TLS 连接。如果你的 collector 未启用 TLS
  > （如本地开发环境的裸 gRPC collector），需要在配置中显式设置 `observability.insecure: true`
  > 以保留原明文行为。

### 2. TLS 证书配置粒度

- `Insecure bool`（配置项）控制"是否明文"这一开关，与现有 `EnableMetrics` /
  `EnableGRPCMetadata` 保持同一风格（配置驱动的布尔开关落在 `ObservabilityConfig`
  struct 上，而非构造期 Option）
- 新增 `WithTLSConfig(*tls.Config) Option`（构造期 Option，因为 `*tls.Config`
  不是可 YAML 序列化的值类型）：允许调用方传入自定义 CA / mTLS 证书。
  **库本身不解析证书文件路径**，加载证书的细节交给调用方。

**exporter TLS 凭据选择优先级**（`NewProvider` 内部）：

```go
switch {
case opts.tlsConfig != nil:
    // 自定义 CA / mTLS，优先级最高
    dialOpts = append(dialOpts, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(opts.tlsConfig)))
case cfg.Insecure:
    // 显式声明明文
    dialOpts = append(dialOpts, otlptracegrpc.WithInsecure())
default:
    // 默认：系统根证书池
    dialOpts = append(dialOpts, otlptracegrpc.WithTLSCredentials(credentials.NewTLS(&tls.Config{})))
}
```

`otlpmetricgrpc` 一侧采用同样的三段式优先级判断。

### 3. Functional Options（遵循 `.claude/rules/options-pattern.md`）

在 `go-framework/hertz/observability/options.go` 与
`go-framework/kitex/observability/options.go` 中各自新增（两个包并行实现，
与现有代码重复风格保持一致，不抽公共层，避免跨文件耦合和不必要的重构面）：

```go
// Option 配置 Provider 构造行为。
type Option func(*providerOptions)

type providerOptions struct {
    traceExporter  sdktrace.SpanExporter
    metricExporter sdkmetric.Exporter
    sampler        sdktrace.Sampler
    propagator     propagation.TextMapPropagator
    tlsConfig      *tls.Config
}

// WithTraceExporter 注入自定义 trace exporter（跳过 otlptracegrpc 构建）。
// 主要用于单测注入内存 exporter（如 tracetest.NewInMemoryExporter()）。
func WithTraceExporter(exp sdktrace.SpanExporter) Option {
    return func(o *providerOptions) {
        if exp != nil {
            o.traceExporter = exp
        }
    }
}

// WithMetricExporter 注入自定义 metric exporter（跳过 otlpmetricgrpc 构建）。
func WithMetricExporter(exp sdkmetric.Exporter) Option {
    return func(o *providerOptions) {
        if exp != nil {
            o.metricExporter = exp
        }
    }
}

// WithSampler 覆盖默认 sampler（TraceIDRatioBased）。
func WithSampler(sampler sdktrace.Sampler) Option {
    return func(o *providerOptions) {
        if sampler != nil {
            o.sampler = sampler
        }
    }
}

// WithPropagator 覆盖默认 propagator（b3 + tracecontext + baggage）。
func WithPropagator(p propagation.TextMapPropagator) Option {
    return func(o *providerOptions) {
        if p != nil {
            o.propagator = p
        }
    }
}

// WithTLSConfig 设置自定义 TLS 配置（自定义 CA / mTLS）。
// 优先级高于 cfg.Insecure；库不解析证书文件路径，由调用方自行加载证书。
func WithTLSConfig(cfg *tls.Config) Option {
    return func(o *providerOptions) {
        if cfg != nil {
            o.tlsConfig = cfg
        }
    }
}
```

`NewProvider` 签名变更为：

```go
func NewProvider(ctx context.Context, cfg config.ObservabilityConfig, opts ...Option) (*Provider, error)
```

新增的 `opts ...Option` 是尾部变长参数，**签名向后兼容**，不影响现有调用方
（`NewProvider(ctx, cfg)` 继续可用）。

若提供了 `WithTraceExporter` / `WithMetricExporter`，则跳过对应的
`otlptracegrpc.New` / `otlpmetricgrpc.New` 构建，直接使用注入的 exporter；
`WithSampler` / `WithPropagator` 若提供则直接使用，否则走现有默认逻辑
（`cfg.SampleRate` → `TraceIDRatioBased`；固定 `b3+tracecontext+baggage`）。

### 4. 错误处理

复用现有 `frameworkerror.ErrObsTraceExport` / `ErrObsMetricExport`
（20603 / 20604），无需新增错误码。

### 5. Kitex `ServerSuite`/`ClientSuite` 兼容性说明

Kitex 侧 `NewServerSuite(cfg, tracerOpts ...TracerOption)` /
`NewClientSuite(cfg, tracerOpts ...TracerOption)` 使用的是独立的
`TracerOption`/`tracerConfig`（控制 `recordSourceOperation` /
`enableGRPCMetadata`），与本次 `NewProvider` 新增的 `Option`/`providerOptions`
是两个不同的类型体系，互不冲突，本次不改动 `suite.go`。

## 测试计划

- `go-framework/config/observability_test.go`：补充 `Insecure` 字段默认值
  （零值 `false`）与显式赋值断言
- `go-framework/hertz/observability/provider_test.go` / `go-framework/kitex/observability/provider_test.go`：
  - `WithTraceExporter` 注入 `tracetest.NewInMemoryExporter()` 后，验证
    Provider 创建成功且不产生真实网络连接
  - `WithMetricExporter` 同理验证 metrics 侧
  - `cfg.Insecure = true` 与 `WithTLSConfig` 同时提供时，`WithTLSConfig`
    优先生效（通过内部可测的分支覆盖，或者以行为不 panic + 无网络报错验证）
  - 默认（`Insecure` 未设置）路径下 exporter 构建不再显式使用
    `otlptracegrpc.WithInsecure()`（回归防护，防止行为退化）

## 文档 / 迁移说明

- `CLAUDE.md` 或独立迁移文档记录本次默认行为变更（明文 → 默认 TLS）
- 两个 `provider.go` 的 godoc 补充新 Option 的用法说明

## 不做的事（Out of Scope）

- 不重构 `otel.GetTracerProvider()` / `GetMeterProvider()` / `GetTextMapPropagator()`
  全局单例读取点（`tracer.go` / `client.go` / `suite.go` / `grpc_metadata.go`）
- 不引入证书文件路径解析（PEM 文件加载等），`WithTLSConfig` 只接受已构造好的
  `*tls.Config`
