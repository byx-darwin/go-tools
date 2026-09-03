# Observability Provider Options Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `NewProvider` in `go-framework/hertz/observability` and `go-framework/kitex/observability` a Functional-Options surface (custom exporter/sampler/propagator/TLS) and flip the default OTLP gRPC transport from hardcoded plaintext to TLS-by-default, closing Issue #96.

**Architecture:** Add a package-local `providerOptions` struct + `Option` type to each of the two `observability` packages (parallel, not shared — matches existing duplication style between hertz/kitex). `NewProvider` gains a trailing `opts ...Option` parameter (backward compatible). `config.ObservabilityConfig` gains an `Insecure bool` field. Exporter TLS credential selection follows a 3-way priority: custom `*tls.Config` (via `WithTLSConfig`) > `cfg.Insecure` (plaintext) > default TLS with system root cert pool.

**Tech Stack:** Go 1.25 workspace mode, OpenTelemetry SDK v1.44.0 (`otlptracegrpc`, `otlpmetricgrpc`), `google.golang.org/grpc/credentials`, testify, `go.opentelemetry.io/otel/sdk/trace/tracetest` (in-memory exporter for tests).

**Spec:** `docs/superpowers/specs/2026-09-03-observability-provider-options-design.md`

## Global Constraints

- `NewProvider` signature change MUST stay backward compatible: `opts ...Option` is a trailing variadic parameter; existing two-arg call sites (`NewProvider(ctx, cfg)`) must keep compiling unchanged.
- `config.ObservabilityConfig.Insecure` zero value is `false` → **default behavior changes from plaintext to TLS**. This is an intentional, user-approved behavior change; document it, don't hide it.
- `WithTLSConfig` takes priority over `cfg.Insecure` when both given.
- Do NOT touch `otel.SetTracerProvider` / `otel.SetMeterProvider` / `otel.SetTextMapPropagator` global-setting behavior, and do NOT touch any downstream `otel.GetTracerProvider()` / `otel.GetMeterProvider()` / `otel.GetTextMapPropagator()` call sites in `tracer.go` / `client.go` / `suite.go` / `grpc_metadata.go` — out of scope per design doc.
- Do NOT add certificate file path parsing — `WithTLSConfig` only accepts an already-built `*tls.Config`.
- All exported symbols need godoc comments starting with the symbol name (`.claude/rules/go.md` § 8.3); errors must be checked or explicitly discarded with `_ =` (§ 8.4); use `any` not `interface{}` (§ 8.5).
- Reuse existing error constructors `frameworkerror.ErrObsTraceExport` / `frameworkerror.ErrObsMetricExport` (codes 20603/20604) — no new error codes.
- Existing kitex `TracerOption`/`tracerConfig` (in `suite.go`) is a separate, unrelated type system — do not touch it, do not let names collide with the new `Option`/`providerOptions`.

---

### Task 1: Add `Insecure` field to `config.ObservabilityConfig`

**Files:**
- Modify: `go-framework/config/observability.go`
- Test: `go-framework/config/observability_test.go`

**Interfaces:**
- Produces: `config.ObservabilityConfig.Insecure bool` field, consumed by Task 2 and Task 3 (`NewProvider` in hertz/kitex).

- [ ] **Step 1: Write the failing test**

Add to `go-framework/config/observability_test.go` (new test function, keep existing ones untouched):

```go
func TestObservabilityConfig_InsecureDefaultsToFalse(t *testing.T) {
	c := ObservabilityConfig{}
	assert.False(t, c.Insecure, "Insecure 零值应为 false（默认走 TLS）")
}

func TestObservabilityConfig_InsecureExplicitTrue(t *testing.T) {
	c := ObservabilityConfig{Insecure: true}
	assert.True(t, c.Insecure)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./go-framework/config/... -run TestObservabilityConfig_Insecure -v`
Expected: FAIL with `c.Insecure undefined (type ObservabilityConfig has no field or method Insecure)`

- [ ] **Step 3: Add the field**

In `go-framework/config/observability.go`, add after the `EnableGRPCMetadata` field (keep struct field order: append at end):

```go
	// Insecure 是否允许 OTLP gRPC exporter 使用明文传输（默认 false，即默认走 TLS）。
	// 零值 false = 默认使用系统根证书池建立 TLS 连接；设为 true 才走明文。
	// 行为变更提示：升级前版本硬编码明文传输，升级后默认改为 TLS——
	// 如果你的 collector 未启用 TLS，需要显式设置 insecure: true。
	Insecure bool `json:"insecure" yaml:"insecure"`
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./go-framework/config/... -v`
Expected: PASS (all `TestObservabilityConfig_*` tests green, including the two new ones and the pre-existing `TestObservabilityConfig_Full` which doesn't set `Insecure` and must still pass since zero value is fine there)

- [ ] **Step 5: Commit**

```bash
git add go-framework/config/observability.go go-framework/config/observability_test.go
git commit -m "feat(config): add Insecure field to ObservabilityConfig (default false = TLS)"
```

---

### Task 2: Hertz `observability` package — Options + secure-by-default `NewProvider`

**Files:**
- Create: `go-framework/hertz/observability/options.go`
- Modify: `go-framework/hertz/observability/provider.go`
- Test: `go-framework/hertz/observability/provider_test.go`

**Interfaces:**
- Consumes: `config.ObservabilityConfig.Insecure` (Task 1)
- Produces:
  - `type Option func(*providerOptions)`
  - `func WithTraceExporter(exp sdktrace.SpanExporter) Option`
  - `func WithMetricExporter(exp sdkmetric.Exporter) Option`
  - `func WithSampler(sampler sdktrace.Sampler) Option`
  - `func WithPropagator(p propagation.TextMapPropagator) Option`
  - `func WithTLSConfig(cfg *tls.Config) Option`
  - `func NewProvider(ctx context.Context, cfg config.ObservabilityConfig, opts ...Option) (*Provider, error)` (signature change, backward compatible)

- [ ] **Step 1: Write the failing tests**

Add to `go-framework/hertz/observability/provider_test.go` (new imports needed: `"go.opentelemetry.io/otel/sdk/trace/tracetest"`, add to the existing import block):

```go
func TestNewProvider_WithTraceExporter_UsesInjectedExporter(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		ServiceName: "test-svc",
	}
	p, err := NewProvider(context.Background(), cfg, WithTraceExporter(exp))
	require.NoError(t, err)
	require.NotNil(t, p)

	_, span := p.tracer.Start(context.Background(), "test-span")
	span.End()
	require.NoError(t, p.Shutdown())

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "test-span", spans[0].Name)
}

func TestNewProvider_DefaultsToTLS_NotInsecure(t *testing.T) {
	// Endpoint 指向不存在的地址；默认走 TLS 时 grpc.NewClient 仍能成功构造
	// （lazy connect），不应该出现 exporter 构造期错误。这里只验证
	// NewProvider 在默认（Insecure 未设置）情况下不会因为强制走 TLS 而
	// 在构造阶段直接报错。
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		Endpoint:    "127.0.0.1:1",
		ServiceName: "test-svc",
	}
	p, err := NewProvider(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.NoError(t, p.Shutdown())
}

func TestNewProvider_InsecureTrue_SkipsTLS(t *testing.T) {
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		Endpoint:    "127.0.0.1:1",
		ServiceName: "test-svc",
		Insecure:    true,
	}
	p, err := NewProvider(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.NoError(t, p.Shutdown())
}

func TestNewProvider_WithTLSConfig_TakesPriorityOverInsecure(t *testing.T) {
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		Endpoint:    "127.0.0.1:1",
		ServiceName: "test-svc",
		Insecure:    true, // should be overridden by WithTLSConfig
	}
	p, err := NewProvider(context.Background(), cfg, WithTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
	require.NoError(t, err)
	require.NotNil(t, p)
	require.NoError(t, p.Shutdown())
}

func TestNewProvider_WithSamplerAndPropagator(t *testing.T) {
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		ServiceName: "test-svc",
	}
	customPropagator := propagation.TraceContext{}
	p, err := NewProvider(context.Background(), cfg,
		WithSampler(sdktrace.AlwaysSample()),
		WithPropagator(customPropagator),
	)
	require.NoError(t, err)
	require.NotNil(t, p)
	require.NoError(t, p.Shutdown())
}
```

Add `"crypto/tls"` and `"go.opentelemetry.io/otel/sdk/trace/tracetest"` and `sdktrace "go.opentelemetry.io/otel/sdk/trace"` and `"go.opentelemetry.io/otel/propagation"` to the test file's import block (dedupe against what's already imported — `assert`/`require` already present).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./go-framework/hertz/observability/... -run TestNewProvider_With -v`
Expected: FAIL with `undefined: WithTraceExporter` (compile error) — this is expected since `options.go` doesn't exist yet.

- [ ] **Step 3: Create `go-framework/hertz/observability/options.go`**

```go
package observability

import (
	"crypto/tls"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"go.opentelemetry.io/otel/propagation"
)

// Option 配置 Provider 的构造行为。
type Option func(*providerOptions)

// providerOptions 收集 NewProvider 构造期可选配置。
type providerOptions struct {
	traceExporter  sdktrace.SpanExporter
	metricExporter sdkmetric.Exporter
	sampler        sdktrace.Sampler
	propagator     propagation.TextMapPropagator
	tlsConfig      *tls.Config
}

// WithTraceExporter 注入自定义 trace exporter，跳过内置的 otlptracegrpc 构建。
// 主要用于单测场景注入内存 exporter（如 tracetest.NewInMemoryExporter()），
// 避免真实网络连接。
func WithTraceExporter(exp sdktrace.SpanExporter) Option {
	return func(o *providerOptions) {
		if exp != nil {
			o.traceExporter = exp
		}
	}
}

// WithMetricExporter 注入自定义 metric exporter，跳过内置的 otlpmetricgrpc 构建。
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

// WithTLSConfig 设置自定义 TLS 配置（自定义 CA、mTLS 证书等）。
// 优先级高于 cfg.Insecure：一旦提供，exporter 始终使用该 TLS 配置建连。
// 本函数不解析证书文件路径，证书加载由调用方负责。
func WithTLSConfig(cfg *tls.Config) Option {
	return func(o *providerOptions) {
		if cfg != nil {
			o.tlsConfig = cfg
		}
	}
}

func newProviderOptions(opts []Option) *providerOptions {
	o := &providerOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
```

- [ ] **Step 4: Modify `go-framework/hertz/observability/provider.go`**

Add imports (`crypto/tls`, `google.golang.org/grpc/credentials`) to the existing import block:

```go
import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/tracer"
	"github.com/cloudwego/hertz/pkg/common/tracer/stats"
	runtimemetrics "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.36.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"

	"github.com/byx-darwin/go-tools/go-framework/config"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
)
```

Replace the `NewProvider` function body:

```go
// NewProvider 创建 Hertz OTel Provider（OTLP gRPC 导出）。
//
// 同时初始化 Tracing 和 Metrics 双通道：
//   - Tracing：OTLP gRPC exporter → TracerProvider
//   - Metrics：OTLP gRPC exporter → MeterProvider + Go runtime metrics
//
// 通过 cfg.EnableMetrics 控制是否启用 Metrics（默认 true，当 Enabled=true 时）。
//
// exporter 的 TLS 凭据按以下优先级选择：
//  1. WithTLSConfig 提供的自定义 *tls.Config（自定义 CA / mTLS）
//  2. cfg.Insecure = true → 明文传输
//  3. 默认：使用系统根证书池建立 TLS 连接
//
// 可通过 WithTraceExporter/WithMetricExporter 注入自定义 exporter（跳过 OTLP gRPC
// 构建，常用于测试隔离），WithSampler/WithPropagator 覆盖默认 sampler/propagator。
func NewProvider(ctx context.Context, cfg config.ObservabilityConfig, opts ...Option) (*Provider, error) {
	p := &Provider{cfg: cfg, shutdown: func(context.Context) error { return nil }}
	if !cfg.Enabled {
		return p, nil
	}

	popts := newProviderOptions(opts)

	res, _ := resource.Merge(resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
		),
	)

	// ── Tracing ──
	var exp sdktrace.SpanExporter
	if popts.traceExporter != nil {
		exp = popts.traceExporter
	} else {
		traceDialOpts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.Endpoint)}
		traceDialOpts = append(traceDialOpts, traceTLSOption(cfg, popts))

		var err error
		exp, err = otlptracegrpc.New(ctx, traceDialOpts...)
		if err != nil {
			return nil, frameworkerror.ErrObsTraceExport.Wrap(err)
		}
	}

	sampler := sdktrace.TraceIDRatioBased(1.0)
	if cfg.SampleRate > 0 {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}
	if popts.sampler != nil {
		sampler = popts.sampler
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	propagator := propagation.TextMapPropagator(propagation.NewCompositeTextMapPropagator(
		b3.New(),
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	if popts.propagator != nil {
		propagator = popts.propagator
	}
	otel.SetTextMapPropagator(propagator)

	p.tracer = tp.Tracer(cfg.ServiceName)
	p.shutdown = tp.Shutdown

	// ── Metrics ──
	if cfg.EnableMetrics {
		var metricExp sdkmetric.Exporter
		if popts.metricExporter != nil {
			metricExp = popts.metricExporter
		} else {
			metricDialOpts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(cfg.Endpoint)}
			metricDialOpts = append(metricDialOpts, metricTLSOption(cfg, popts))

			var err error
			metricExp, err = otlpmetricgrpc.New(ctx, metricDialOpts...)
			if err != nil {
				return nil, frameworkerror.ErrObsMetricExport.Wrap(err)
			}
		}

		interval := cfg.MetricsInterval
		if interval <= 0 {
			interval = 15 * time.Second
		}

		reader := sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(interval))
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(reader),
			sdkmetric.WithResource(res),
		)
		otel.SetMeterProvider(mp)
		p.meterProvider = mp

		// Go runtime metrics（goroutines, GC, memory 等）
		if err := runtimemetrics.Start(runtimemetrics.WithMeterProvider(mp)); err != nil {
			return nil, frameworkerror.ErrObsRuntimeMetrics.Wrap(err)
		}

		// 包装 shutdown 以同时关闭 metrics
		prevShutdown := p.shutdown
		p.shutdown = func(ctx context.Context) error {
			errT := prevShutdown(ctx)
			errM := mp.Shutdown(ctx)
			if errT != nil {
				return errT
			}
			return errM
		}
	}

	return p, nil
}

// traceTLSOption 按优先级（WithTLSConfig > cfg.Insecure > 默认 TLS）返回
// otlptracegrpc 的凭据 Option。
func traceTLSOption(cfg config.ObservabilityConfig, popts *providerOptions) otlptracegrpc.Option {
	switch {
	case popts.tlsConfig != nil:
		return otlptracegrpc.WithTLSCredentials(credentials.NewTLS(popts.tlsConfig))
	case cfg.Insecure:
		return otlptracegrpc.WithInsecure()
	default:
		return otlptracegrpc.WithTLSCredentials(credentials.NewTLS(&tls.Config{}))
	}
}

// metricTLSOption 按优先级（WithTLSConfig > cfg.Insecure > 默认 TLS）返回
// otlpmetricgrpc 的凭据 Option。
func metricTLSOption(cfg config.ObservabilityConfig, popts *providerOptions) otlpmetricgrpc.Option {
	switch {
	case popts.tlsConfig != nil:
		return otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(popts.tlsConfig))
	case cfg.Insecure:
		return otlpmetricgrpc.WithInsecure()
	default:
		return otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(&tls.Config{}))
	}
}
```

Do not change anything else in the file (`ServerMiddleware`, `ServerTracer`, `TracerServerMiddleware`, `Shutdown`, `hertzCarrier` stay as-is).

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./go-framework/hertz/observability/... -v`
Expected: PASS for all tests, including the pre-existing `TestNewProvider_Enabled_InvalidEndpoint` and `TestProvider_Enabled_WithMetrics` (their endpoints are unreachable strings — `otlptracegrpc.New`/`otlpmetricgrpc.New` with gRPC's lazy-connect behavior succeed at construction time regardless of TLS vs plaintext, so these don't need changes)

- [ ] **Step 6: Build check**

Run: `go build ./go-framework/...`
Expected: success, no compile errors

- [ ] **Step 7: Commit**

```bash
git add go-framework/hertz/observability/options.go go-framework/hertz/observability/provider.go go-framework/hertz/observability/provider_test.go
git commit -m "feat(hertz/observability): add Functional Options to NewProvider, default to TLS"
```

---

### Task 3: Kitex `observability` package — Options + secure-by-default `NewProvider`

**Files:**
- Create: `go-framework/kitex/observability/options.go`
- Modify: `go-framework/kitex/observability/provider.go`
- Test: `go-framework/kitex/observability/provider_test.go`

**Interfaces:**
- Consumes: `config.ObservabilityConfig.Insecure` (Task 1)
- Produces: same `Option`/`providerOptions`/`WithTraceExporter`/`WithMetricExporter`/`WithSampler`/`WithPropagator`/`WithTLSConfig` set as Task 2, but scoped to the `kitex/observability` package (separate type, no cross-package sharing — matches design doc's "两个包并行实现" decision). Must NOT collide with the existing `TracerOption`/`tracerConfig` types already defined in `suite.go`.
- `func NewProvider(ctx context.Context, cfg config.ObservabilityConfig, opts ...Option) (*Provider, error)` (signature change, backward compatible)

- [ ] **Step 1: Write the failing tests**

Add to `go-framework/kitex/observability/provider_test.go` (rewrite imports to add `"crypto/tls"`, `"testing"` already there, `"github.com/stretchr/testify/require"`, `"go.opentelemetry.io/otel/propagation"`, `sdktrace "go.opentelemetry.io/otel/sdk/trace"`, `"go.opentelemetry.io/otel/sdk/trace/tracetest"`):

```go
func TestNewProvider_WithTraceExporter_UsesInjectedExporter(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		ServiceName: "test-svc",
	}
	p, err := NewProvider(context.Background(), cfg, WithTraceExporter(exp))
	require.NoError(t, err)
	require.True(t, p.Enabled())

	_, span := p.tracer.Start(context.Background(), "test-span")
	span.End()
	require.NoError(t, p.Shutdown())

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "test-span", spans[0].Name)
}

func TestNewProvider_DefaultsToTLS_NotInsecure(t *testing.T) {
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		Endpoint:    "127.0.0.1:1",
		ServiceName: "test-svc",
	}
	p, err := NewProvider(context.Background(), cfg)
	require.NoError(t, err)
	require.NoError(t, p.Shutdown())
}

func TestNewProvider_InsecureTrue_SkipsTLS(t *testing.T) {
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		Endpoint:    "127.0.0.1:1",
		ServiceName: "test-svc",
		Insecure:    true,
	}
	p, err := NewProvider(context.Background(), cfg)
	require.NoError(t, err)
	require.NoError(t, p.Shutdown())
}

func TestNewProvider_WithTLSConfig_TakesPriorityOverInsecure(t *testing.T) {
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		Endpoint:    "127.0.0.1:1",
		ServiceName: "test-svc",
		Insecure:    true,
	}
	p, err := NewProvider(context.Background(), cfg, WithTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
	require.NoError(t, err)
	require.NoError(t, p.Shutdown())
}

func TestNewProvider_WithSamplerAndPropagator(t *testing.T) {
	cfg := config.ObservabilityConfig{
		Enabled:     true,
		ServiceName: "test-svc",
	}
	p, err := NewProvider(context.Background(), cfg,
		WithSampler(sdktrace.AlwaysSample()),
		WithPropagator(propagation.TraceContext{}),
	)
	require.NoError(t, err)
	require.NoError(t, p.Shutdown())
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./go-framework/kitex/observability/... -run TestNewProvider_With -v`
Expected: FAIL with `undefined: WithTraceExporter` (compile error)

- [ ] **Step 3: Create `go-framework/kitex/observability/options.go`**

```go
package observability

import (
	"crypto/tls"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"go.opentelemetry.io/otel/propagation"
)

// Option 配置 Provider 的构造行为。
type Option func(*providerOptions)

// providerOptions 收集 NewProvider 构造期可选配置。
type providerOptions struct {
	traceExporter  sdktrace.SpanExporter
	metricExporter sdkmetric.Exporter
	sampler        sdktrace.Sampler
	propagator     propagation.TextMapPropagator
	tlsConfig      *tls.Config
}

// WithTraceExporter 注入自定义 trace exporter，跳过内置的 otlptracegrpc 构建。
// 主要用于单测场景注入内存 exporter（如 tracetest.NewInMemoryExporter()），
// 避免真实网络连接。
func WithTraceExporter(exp sdktrace.SpanExporter) Option {
	return func(o *providerOptions) {
		if exp != nil {
			o.traceExporter = exp
		}
	}
}

// WithMetricExporter 注入自定义 metric exporter，跳过内置的 otlpmetricgrpc 构建。
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

// WithTLSConfig 设置自定义 TLS 配置（自定义 CA、mTLS 证书等）。
// 优先级高于 cfg.Insecure：一旦提供，exporter 始终使用该 TLS 配置建连。
// 本函数不解析证书文件路径，证书加载由调用方负责。
func WithTLSConfig(cfg *tls.Config) Option {
	return func(o *providerOptions) {
		if cfg != nil {
			o.tlsConfig = cfg
		}
	}
}

func newProviderOptions(opts []Option) *providerOptions {
	o := &providerOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
```

- [ ] **Step 4: Modify `go-framework/kitex/observability/provider.go`**

Add imports (`crypto/tls`, `google.golang.org/grpc/credentials`) to the existing import block:

```go
import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/byx-darwin/go-tools/go-framework/config"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
	runtimemetrics "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)
```

Replace the `NewProvider` function body (mirrors Task 2's hertz version, same TLS-priority helper functions duplicated in this package):

```go
// NewProvider 创建 Kitex OTel Provider（OTLP gRPC 导出）。
//
// 同时初始化 Tracing 和 Metrics 双通道：
//   - Tracing：OTLP gRPC exporter → TracerProvider
//   - Metrics：OTLP gRPC exporter → MeterProvider + Go runtime metrics
//
// 通过 cfg.EnableMetrics 控制是否启用 Metrics（默认 true，当 Enabled=true 时）。
//
// exporter 的 TLS 凭据按以下优先级选择：
//  1. WithTLSConfig 提供的自定义 *tls.Config（自定义 CA / mTLS）
//  2. cfg.Insecure = true → 明文传输
//  3. 默认：使用系统根证书池建立 TLS 连接
//
// 可通过 WithTraceExporter/WithMetricExporter 注入自定义 exporter（跳过 OTLP gRPC
// 构建，常用于测试隔离），WithSampler/WithPropagator 覆盖默认 sampler/propagator。
func NewProvider(ctx context.Context, cfg config.ObservabilityConfig, opts ...Option) (*Provider, error) {
	p := &Provider{cfg: cfg, shutdown: func(context.Context) error { return nil }}
	if !cfg.Enabled {
		return p, nil
	}

	popts := newProviderOptions(opts)

	res, _ := resource.Merge(resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
		),
	)

	// ── Tracing ──
	var exp sdktrace.SpanExporter
	if popts.traceExporter != nil {
		exp = popts.traceExporter
	} else {
		traceDialOpts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.Endpoint)}
		traceDialOpts = append(traceDialOpts, traceTLSOption(cfg, popts))

		var err error
		exp, err = otlptracegrpc.New(ctx, traceDialOpts...)
		if err != nil {
			return nil, frameworkerror.ErrObsTraceExport.Wrap(err)
		}
	}

	sampler := sdktrace.TraceIDRatioBased(1.0)
	if cfg.SampleRate > 0 {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}
	if popts.sampler != nil {
		sampler = popts.sampler
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	propagator := propagation.TextMapPropagator(propagation.NewCompositeTextMapPropagator(
		b3.New(),
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	if popts.propagator != nil {
		propagator = popts.propagator
	}
	otel.SetTextMapPropagator(propagator)

	p.tracer = tp.Tracer(cfg.ServiceName)
	p.shutdown = tp.Shutdown

	// ── Metrics ──
	if cfg.EnableMetrics {
		var metricExp sdkmetric.Exporter
		if popts.metricExporter != nil {
			metricExp = popts.metricExporter
		} else {
			metricDialOpts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(cfg.Endpoint)}
			metricDialOpts = append(metricDialOpts, metricTLSOption(cfg, popts))

			var err error
			metricExp, err = otlpmetricgrpc.New(ctx, metricDialOpts...)
			if err != nil {
				return nil, frameworkerror.ErrObsMetricExport.Wrap(err)
			}
		}

		interval := cfg.MetricsInterval
		if interval <= 0 {
			interval = 15 * time.Second
		}

		reader := sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(interval))
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(reader),
			sdkmetric.WithResource(res),
		)
		otel.SetMeterProvider(mp)
		p.meterProvider = mp

		// Go runtime metrics（goroutines, GC, memory 等）
		if err := runtimemetrics.Start(runtimemetrics.WithMeterProvider(mp)); err != nil {
			return nil, frameworkerror.ErrObsRuntimeMetrics.Wrap(err)
		}

		// 包装 shutdown 以同时关闭 metrics
		prevShutdown := p.shutdown
		p.shutdown = func(ctx context.Context) error {
			errT := prevShutdown(ctx)
			errM := mp.Shutdown(ctx)
			if errT != nil {
				return errT
			}
			return errM
		}
	}

	return p, nil
}

// traceTLSOption 按优先级（WithTLSConfig > cfg.Insecure > 默认 TLS）返回
// otlptracegrpc 的凭据 Option。
func traceTLSOption(cfg config.ObservabilityConfig, popts *providerOptions) otlptracegrpc.Option {
	switch {
	case popts.tlsConfig != nil:
		return otlptracegrpc.WithTLSCredentials(credentials.NewTLS(popts.tlsConfig))
	case cfg.Insecure:
		return otlptracegrpc.WithInsecure()
	default:
		return otlptracegrpc.WithTLSCredentials(credentials.NewTLS(&tls.Config{}))
	}
}

// metricTLSOption 按优先级（WithTLSConfig > cfg.Insecure > 默认 TLS）返回
// otlpmetricgrpc 的凭据 Option。
func metricTLSOption(cfg config.ObservabilityConfig, popts *providerOptions) otlpmetricgrpc.Option {
	switch {
	case popts.tlsConfig != nil:
		return otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(popts.tlsConfig))
	case cfg.Insecure:
		return otlpmetricgrpc.WithInsecure()
	default:
		return otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(&tls.Config{}))
	}
}
```

Do not change anything else in the file (`Enabled`, `MeterProvider`, `Middleware`, `ServerSuite`, `ClientSuite`, `Shutdown` stay as-is).

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./go-framework/kitex/observability/... -v`
Expected: PASS for all tests including pre-existing `TestNewProvider`, `TestProvider_Disabled`, `TestProvider_Shutdown`

- [ ] **Step 6: Build check**

Run: `go build ./go-framework/...`
Expected: success

- [ ] **Step 7: Commit**

```bash
git add go-framework/kitex/observability/options.go go-framework/kitex/observability/provider.go go-framework/kitex/observability/provider_test.go
git commit -m "feat(kitex/observability): add Functional Options to NewProvider, default to TLS"
```

---

### Task 4: Full validation + migration note

**Files:**
- Modify: `CLAUDE.md` (or a note under `specs/` — pick `CLAUDE.md`'s existing "Key Decisions" convention: add a one-row entry, no new file)

**Interfaces:**
- Consumes: nothing new — this task is validation + documentation only.

- [ ] **Step 1: Run full workspace build**

Run: `go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`
Expected: success, no errors

- [ ] **Step 2: Run go vet**

Run: `go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`
Expected: no issues

- [ ] **Step 3: Run golangci-lint on go-framework**

Run: `golangci-lint run --timeout=5m ./go-framework/...`
Expected: no issues (check specifically: godoc comments present on all new exported symbols — `Option`, `WithTraceExporter`, `WithMetricExporter`, `WithSampler`, `WithPropagator`, `WithTLSConfig`, `ObservabilityConfig.Insecore` field — and no errcheck violations)

If lint reports issues, fix them in the relevant Task's files and re-run this step (do not proceed until clean).

- [ ] **Step 4: Run full test suite**

Run: `go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1`
Expected: all PASS

- [ ] **Step 5: gofmt check**

Run: `gofmt -l go-framework/config/observability.go go-framework/config/observability_test.go go-framework/hertz/observability/options.go go-framework/hertz/observability/provider.go go-framework/hertz/observability/provider_test.go go-framework/kitex/observability/options.go go-framework/kitex/observability/provider.go go-framework/kitex/observability/provider_test.go`
Expected: empty output (no files listed)

- [ ] **Step 6: Add migration note to `CLAUDE.md`**

In `CLAUDE.md`, under the `## Key Decisions (Confirmed 2026-06-23)` table, add one row documenting the behavior change (append as a new row, keep table format intact):

```markdown
| D7 | Observability OTLP TLS default | **Default TLS** (system root cert pool); `ObservabilityConfig.Insecure=true` opts back into plaintext (#96) | ✅ active |
```

- [ ] **Step 7: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: record OTLP TLS-by-default migration note (D7, closes #96 groundwork)"
```

---

## Self-Review Notes

- **Spec coverage:** Task 1 covers `Insecure` config field + default-TLS behavior change; Task 2 + Task 3 cover all 5 Options (`WithTraceExporter`, `WithMetricExporter`, `WithSampler`, `WithPropagator`, `WithTLSConfig`) for both hertz and kitex packages per the design's "两个包并行实现" decision; Task 4 covers full validation + migration documentation. Test-isolation via `WithTraceExporter`/`WithMetricExporter` injection is covered in both Task 2 Step 1 and Task 3 Step 1. Out-of-scope items (global singleton refactor, cert file parsing) are explicitly called out in Global Constraints and left untouched.
- **Type consistency:** `Option`, `providerOptions`, `WithTraceExporter`, `WithMetricExporter`, `WithSampler`, `WithPropagator`, `WithTLSConfig`, `traceTLSOption`, `metricTLSOption` signatures are identical across Task 2 (hertz) and Task 3 (kitex) — verified consistent naming and parameter types throughout.
- **No placeholders:** all code blocks are complete, runnable Go — no TODO/TBD markers.
