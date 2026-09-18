# go-common/log 日志与 OTel Trace 关联设计

- Issue: [#108](https://github.com/byx-darwin/go-tools/issues/108)
- Status: Approved
- Date: 2026-09-18

## 背景

`go-common/log` 定义了 `ContextKeyTraceID` / `ContextKeySpanID` 两个常量（`context.go`），
但 `contextHandler.Handle`（`handler.go`）只消费 `request_id`，这两个常量从未被读取——是死代码。
调用方无法通过 `WithContextValue` 让 trace_id/span_id 出现在日志里，且没有任何报错或提示。

`go-framework/{hertz,kitex}/observability` 已经用标准 OTel instrumentation
（`trace.ContextWithSpan` 等）把 span 放进请求链路的 `context.Context`。框架侧把 trace
采出来了，但日志侧无法引用它——接入 OTel APM 后，日志与 trace 无法互相跳转。

## 关键发现

调查过程中发现比 Issue 描述更具体的根因：

1. **`go-common` 已经直接依赖 `go.opentelemetry.io/otel/trace`**（`go-common/go.mod:12`）。
   这个依赖不是新决策——它已经存在，只是没有被正确使用到所有代码路径上。
2. **包内有两条独立的 Logger 构造路径，行为不一致**：
   - `log.New(opts ...Option)`（`logger.go`，走 `buildLogger`）：已经有一个 `otelHandler`，
     用 `trace.SpanFromContext(ctx)` 正确注入 `trace_id`/`span_id`。但**零测试覆盖**。
   - `log.NewLogger(cfg Config, release ReleaseInfo)`（`new_logger.go`，YAML 配置场景，
     即 `go-framework/config` 实际会用到的路径）：只有 `contextHandler` 读 `request_id`，
     **完全没有** trace/span 注入逻辑。

   两条路径各自独立拼装 handler 链，从未统一过，这正是 Issue 描述的"静默不生效"的根因——
   不是缺功能，是功能只接到了一半的构造路径上。

## 依赖决策：不需要新决策

Issue 里提出的三个方案（直接依赖 / 拆子包 / Config 注入 TraceExtractor）都基于一个前提：
`go-common` 目前不依赖 otel。但这个前提不成立——依赖已经在，`otelHandler` 已经证明了
"go-common/log 直接调用 otel/trace API"这条路径完全可行且已经在生产代码里跑着（只是没被
`NewLogger` 路径引用）。

因此选定 **方案 1（直接依赖）**，且不是"新增依赖"，而是"消除两条路径的不一致"。
不需要引入 `go-framework` 侧的 `TraceExtractor` 桥接层——`trace.SpanContextFromContext(ctx)`
本身就是与框架无关的标准 OTel API，只要日志调用时传入的 ctx 来自请求链路（`go-framework`
的 observability 中间件已经保证这一点），就能自动生效，调用方不需要任何额外接线。

## 设计

### 1. 统一 trace/span 注入到 `contextHandler`

`handler.go` 的 `contextHandler.Handle` 从：

```go
func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		r.AddAttrs(slog.String("request_id", requestID))
	}
	return h.next.Handle(ctx, r)
}
```

改为：

```go
func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		r.AddAttrs(slog.String("request_id", requestID))
	}

	traceID, spanID := traceAndSpanIDFromContext(ctx)
	if traceID != "" {
		r.AddAttrs(slog.String("trace_id", traceID))
	}
	if spanID != "" {
		r.AddAttrs(slog.String("span_id", spanID))
	}

	return h.next.Handle(ctx, r)
}

// traceAndSpanIDFromContext 优先从活跃 OTel span 提取 trace_id/span_id；
// 没有有效 span 时回退到手工通过 WithContextValue 设置的 ContextKeyTraceID/ContextKeySpanID。
func traceAndSpanIDFromContext(ctx context.Context) (traceID, spanID string) {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		return sc.TraceID().String(), sc.SpanID().String()
	}
	return ContextValue(ctx, ContextKeyTraceID), ContextValue(ctx, ContextKeySpanID)
}
```

优先级：**活跃 OTel span > 手工设置的 context 值**。这样 `ContextKeyTraceID`/
`ContextKeySpanID` 不再是死代码（满足 Issue 的"保底"验收标准），同时自动路径不需要
调用方做任何事。

### 2. 消除 `logger.go` 里的重复实现

删除 `logger.go` 里的 `otelHandler` 类型（`trace.SpanFromContext` 版本，逻辑与上面重复
且没有 request_id 支持）。`buildLogger`（`New`/`NewFromLegacyConfig` 共用）改为：

```go
handler = NewContextHandler(handler)
```

替换原来的 `handler = &otelHandler{next: handler}`。这样两条构造路径统一走同一份
trace 注入逻辑，未来只需要改一处。

`logger.go` 顶部包注释已经写了"自动 OTel span 关联（TraceID/SpanID 注入）"——这句话
维持不变，只是现在对两条路径都成立了。

### 3. `NewLogger` 自定义 handler 注入 Option

`new_logger.go` 的 `NewLogger` 增加变参 Option，满足 Issue 附带建议 (a)：

```go
// LoggerOption 配置 NewLogger 构建过程的可选项。
type LoggerOption func(*loggerBuildOptions)

type loggerBuildOptions struct {
	extraHandlers []func(slog.Handler) slog.Handler
}

// WithExtraHandler 注入一个自定义 handler 包装函数，应用在标准 handler 链
// （output → context → release → mask）构建完成之后、作为最外层 handler。
// 可多次调用，按调用顺序层层包装（最后一次调用的在最外层，最先执行）。
func WithExtraHandler(wrap func(slog.Handler) slog.Handler) LoggerOption {
	return func(o *loggerBuildOptions) {
		if wrap != nil {
			o.extraHandlers = append(o.extraHandlers, wrap)
		}
	}
}

func NewLogger(cfg Config, release ReleaseInfo, opts ...LoggerOption) (*Logger, error) {
	o := &loggerBuildOptions{}
	for _, opt := range opts {
		opt(o)
	}
	// ...现有 output → context → release → mask 链构建逻辑不变...
	for _, wrap := range o.extraHandlers {
		handler = wrap(handler)
	}
	return &Logger{Logger: slog.New(handler), level: parseLevel(cfg.Level)}, nil
}
```

签名变化是**新增变参**，不破坏现有调用方（`NewLogger(cfg, release)` 仍然合法）。

不在这次改动范围内的：`Config.Categories` 未生效的 warning（已存在）、#105 的
"未消费配置键应 warn"通用约束——留给 #105 或后续独立 Issue。

## 测试计划

- `TestContextHandler_WithActiveSpan`：构造有效 `trace.SpanContext`，验证日志输出含正确
  `trace_id`/`span_id`
- `TestContextHandler_NoSpan`：无 span 时不产生空字段、不报错（对齐 `request_id` 现有行为）
- `TestContextHandler_ManualFallback`：只用 `WithContextValue(ctx, ContextKeyTraceID, ...)`
  手工设置（无 otel span）时生效
- `TestContextHandler_AllThreeCoexist`：span + request_id 并存时三者都出现
- `TestContextHandler_SpanTakesPriorityOverManual`：span 与手工值都存在时以 span 为准
- `buildLogger`（`New`/`NewFromLegacyConfig`）补一条回归测试，确认与 `NewLogger` 行为一致
- `NewLogger` 的 `WithExtraHandler` Option 补一条测试

## 文档改动

- `context.go`/`handler.go`：`ContextKeyTraceID`/`ContextKeySpanID`、`contextHandler` 的
  godoc 补充优先级说明（span 优先，手工值兜底）
- 补一句框架配合说明：`go-framework/{hertz,kitex}/observability` 的 OTel instrumentation
  已经把 span 放进请求 ctx，`go-common/log` 消费方不需要任何额外接线

## 验收标准对照（来自 Issue #108）

- [x] 有活跃 OTel span 的 context 下打日志，输出包含正确 `trace_id`/`span_id` —— 设计 §1
- [x] 无 span 时不产生空字段、不报错 —— 设计 §1，测试 `TestContextHandler_NoSpan`
- [x] `ContextKeyTraceID`/`ContextKeySpanID` 不再是死代码 —— 设计 §1（手工回退路径）
- [x] 单测覆盖 span / 无 span / 只有 request_id / 三者并存 —— 测试计划
- [x] 文档说明与 `go-framework/{hertz,kitex}/observability` 的配合方式 —— 文档改动
- [x] 依赖形态决策在设计文档中写明理由 —— "依赖决策：不需要新决策"一节
