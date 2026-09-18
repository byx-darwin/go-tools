# go-common/log OTel Trace Context Association Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `go-common/log` reliably attach `trace_id`/`span_id` to every log line when an active OTel span exists in the context, unify the two divergent Logger construction paths (`New` vs `NewLogger`) onto one implementation, and let `NewLogger` accept a custom handler-wrapping option.

**Architecture:** `contextHandler` (in `handler.go`) becomes the single place that extracts `trace_id`/`span_id`: first from an active OTel span via `trace.SpanContextFromContext(ctx)`, falling back to the existing `ContextKeyTraceID`/`ContextKeySpanID` manual context values if no span is active. The duplicate `otelHandler` in `logger.go` (used only by `New`/`NewFromLegacyConfig`) is deleted, and `buildLogger` is rewired to use `NewContextHandler` instead — so both construction paths get identical behavior. `NewLogger` gains a variadic `LoggerOption` parameter (`WithExtraHandler`) that lets a caller wrap the final handler chain.

**Tech Stack:** Go `log/slog`, `go.opentelemetry.io/otel/trace` (already a direct dependency of `go-common`, see `go-common/go.mod:12`).

**Spec:** `docs/superpowers/specs/2026-09-18-log-otel-trace-context-design.md`

## Global Constraints

- `go-common` must not gain any new external dependency — `otel/trace` is already in `go-common/go.mod`.
- Priority order when both an OTel span and a manually-set context value are present: **active span wins**.
- No span and no manual value → no `trace_id`/`span_id` attrs added, no error (same behavior as `request_id` today).
- `NewLogger(cfg, release)` (2-arg call) must keep compiling — the new option parameter must be variadic and additive, never a breaking signature change.
- All new/changed exported symbols need `// Name ...`-style godoc comments per `.claude/rules/go.md` §8.3.
- Run `gofmt -l` and `go vet ./go-common/...` before each commit that touches `go-common`.

---

### Task 1: Consolidate trace/span extraction into `contextHandler`

**Files:**
- Modify: `go-common/log/handler.go:78-99` (the `contextHandler` type and its `Handle` method)
- Test: `go-common/log/handler_test.go` (package `log_test`)

**Interfaces:**
- Consumes: `ContextValue(ctx, key string) string` and `ContextKeyTraceID`/`ContextKeySpanID` constants from `go-common/log/context.go` (already exported); `trace.SpanContextFromContext(ctx context.Context) trace.SpanContext` from `go.opentelemetry.io/otel/trace`.
- Produces: `contextHandler.Handle` now also writes `trace_id`/`span_id` slog attrs. No new exported symbols — behavior change only, callers of `NewContextHandler` are unaffected in signature.

- [ ] **Step 1: Write the failing tests**

Add to `go-common/log/handler_test.go` (this file is `package log_test`, so import `go.opentelemetry.io/otel/trace`):

```go
func TestContextHandler_WithActiveSpan(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	handler := log.NewContextHandler(inner)

	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	require.NoError(t, err)
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	require.NoError(t, err)
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	logger := slog.New(handler)
	logger.InfoContext(ctx, "test")

	require.Contains(t, buf.String(), `"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"`)
	require.Contains(t, buf.String(), `"span_id":"00f067aa0ba902b7"`)
}

func TestContextHandler_NoSpan(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	handler := log.NewContextHandler(inner)

	logger := slog.New(handler)
	logger.InfoContext(context.Background(), "test")

	require.NotContains(t, buf.String(), "trace_id")
	require.NotContains(t, buf.String(), "span_id")
}

func TestContextHandler_ManualFallback(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	handler := log.NewContextHandler(inner)

	ctx := log.WithContextValue(context.Background(), log.ContextKeyTraceID, "manual-trace")
	ctx = log.WithContextValue(ctx, log.ContextKeySpanID, "manual-span")
	logger := slog.New(handler)
	logger.InfoContext(ctx, "test")

	require.Contains(t, buf.String(), `"trace_id":"manual-trace"`)
	require.Contains(t, buf.String(), `"span_id":"manual-span"`)
}

func TestContextHandler_AllThreeCoexist(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	handler := log.NewContextHandler(inner)

	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	require.NoError(t, err)
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	require.NoError(t, err)
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	ctx = log.WithRequestID(ctx, "req-789")

	logger := slog.New(handler)
	logger.InfoContext(ctx, "test")

	require.Contains(t, buf.String(), `"request_id":"req-789"`)
	require.Contains(t, buf.String(), `"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"`)
	require.Contains(t, buf.String(), `"span_id":"00f067aa0ba902b7"`)
}

func TestContextHandler_SpanTakesPriorityOverManual(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
	handler := log.NewContextHandler(inner)

	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	require.NoError(t, err)
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	require.NoError(t, err)
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	ctx = log.WithContextValue(ctx, log.ContextKeyTraceID, "manual-trace")
	ctx = log.WithContextValue(ctx, log.ContextKeySpanID, "manual-span")

	logger := slog.New(handler)
	logger.InfoContext(ctx, "test")

	require.Contains(t, buf.String(), `"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"`)
	require.NotContains(t, buf.String(), "manual-trace")
}
```

Add `"go.opentelemetry.io/otel/trace"` to the import block at the top of `go-common/log/handler_test.go`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./go-common/log/... -run TestContextHandler -v`
Expected: `TestContextHandler_WithActiveSpan`, `TestContextHandler_ManualFallback`, `TestContextHandler_AllThreeCoexist`, `TestContextHandler_SpanTakesPriorityOverManual` FAIL (missing `trace_id`/`span_id` in output). `TestContextHandler_NoSpan` passes trivially (no assertion can fail yet since there's nothing to add) — that's fine, it's the regression guard.

- [ ] **Step 3: Implement `contextHandler.Handle`**

In `go-common/log/handler.go`, add the import `"go.opentelemetry.io/otel/trace"` to the import block, then replace the `contextHandler.Handle` method:

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
// 没有有效 span 时回退到手工通过 WithContextValue 设置的
// ContextKeyTraceID/ContextKeySpanID。
func traceAndSpanIDFromContext(ctx context.Context) (traceID, spanID string) {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		return sc.TraceID().String(), sc.SpanID().String()
	}
	return ContextValue(ctx, ContextKeyTraceID), ContextValue(ctx, ContextKeySpanID)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./go-common/log/... -run TestContextHandler -v`
Expected: all 7 `TestContextHandler_*` tests PASS.

- [ ] **Step 5: Update godoc on `ContextKeyTraceID`/`ContextKeySpanID`**

In `go-common/log/context.go`, replace the comments on the two constants:

```go
	// ContextKeyTraceID Trace ID 的 context key。仅在没有活跃 OTel span 时，
	// 作为 contextHandler 的手工回退值生效——有活跃 span 时优先读 span 的 TraceID。
	ContextKeyTraceID = "trace_id"

	// ContextKeySpanID Span ID 的 context key。仅在没有活跃 OTel span 时，
	// 作为 contextHandler 的手工回退值生效——有活跃 span 时优先读 span 的 SpanID。
	ContextKeySpanID = "span_id"
```

- [ ] **Step 6: Run the full package test suite**

Run: `go test ./go-common/log/... -count=1`
Expected: PASS, no regressions.

- [ ] **Step 7: Commit**

```bash
git add go-common/log/handler.go go-common/log/handler_test.go go-common/log/context.go
git commit -m "fix(log): contextHandler now extracts trace_id/span_id from active OTel span (#108)"
```

---

### Task 2: Remove duplicate `otelHandler`, unify `buildLogger` on `contextHandler`

**Files:**
- Modify: `go-common/log/logger.go:222` (`buildLogger`) and `go-common/log/logger.go:263-293` (delete `otelHandler` type and its methods)
- Test: `go-common/log/logger_test.go` (package `log`, internal — can reach `buildLogger`/`otelHandler` before this task, and `New`/`NewFromLegacyConfig` after)

**Interfaces:**
- Consumes: `NewContextHandler(next slog.Handler) slog.Handler` from `go-common/log/handler.go` (already exported, used by Task 1's updated `contextHandler`).
- Produces: `New(opts ...Option) *Logger` and `NewFromLegacyConfig(c LegacyConfig) *Logger` now inject `trace_id`/`span_id`/`request_id` via the same `contextHandler` logic as `NewLogger`. No signature changes.

- [ ] **Step 1: Write the failing test**

Add to `go-common/log/logger_test.go` (package `log`, so `trace` import path is the same as production code):

```go
func TestNew_ContextHandler_RequestID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l := New(WithFilePath(path))
	defer func() { _ = l.Close() }()

	ctx := WithRequestID(context.Background(), "req-new-path")
	l.InfoContext(ctx, "request id via New()")

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"request_id":"req-new-path"`)
}
```

Add `"github.com/stretchr/testify/require"` is already imported; no new import needed beyond what's already at the top of the file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./go-common/log/... -run TestNew_ContextHandler_RequestID -v`
Expected: FAIL — current `buildLogger` wires `otelHandler` only (no `request_id` support), so `request_id` never appears in output.

- [ ] **Step 3: Delete `otelHandler` and rewire `buildLogger`**

In `go-common/log/logger.go`:

1. Delete the entire `otelHandler` type block (the struct definition and its four methods, currently lines 263-293 — from `// otelHandler 在每条日志中注入 TraceID 和 SpanID。` through the closing brace of `WithGroup`).
2. In `buildLogger`, replace:

```go
	handler = &otelHandler{next: handler}
```

with:

```go
	handler = NewContextHandler(handler)
```

3. Remove the now-unused `"go.opentelemetry.io/otel/trace"` import from `logger.go`'s import block if nothing else in the file references `trace` (check with `grep -n "trace\." go-common/log/logger.go` after the edit — if no matches remain, delete the import line).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./go-common/log/... -run TestNew_ContextHandler_RequestID -v`
Expected: PASS.

- [ ] **Step 5: Run the full package test suite and vet**

Run: `go test ./go-common/log/... -count=1 && go vet ./go-common/...`
Expected: PASS, no unused-import errors.

- [ ] **Step 6: Commit**

```bash
git add go-common/log/logger.go go-common/log/logger_test.go
git commit -m "fix(log): unify New()/NewFromLegacyConfig on contextHandler, remove duplicate otelHandler (#108)"
```

---

### Task 3: Add `WithExtraHandler` option to `NewLogger`

**Files:**
- Modify: `go-common/log/new_logger.go` (the `NewLogger` function signature and body)
- Test: `go-common/log/new_logger_test.go` (package `log_test`)

**Interfaces:**
- Consumes: nothing new — wraps the `slog.Handler` chain `NewLogger` already builds internally.
- Produces:
  - `type LoggerOption func(*loggerBuildOptions)` (exported type)
  - `func WithExtraHandler(wrap func(slog.Handler) slog.Handler) LoggerOption` (exported constructor)
  - `func NewLogger(cfg Config, release ReleaseInfo, opts ...LoggerOption) (*Logger, error)` (signature grows by one variadic trailing param — existing 2-arg call sites keep compiling unchanged)

- [ ] **Step 1: Write the failing test**

Add to `go-common/log/new_logger_test.go`:

```go
func TestNewLogger_WithExtraHandler(t *testing.T) {
	var captured []string
	wrap := func(next slog.Handler) slog.Handler {
		return &captureHandler{next: next, captured: &captured}
	}

	cfg := log.Config{
		Level:  "info",
		Format: "json",
		Mode:   "console",
	}
	l, err := log.NewLogger(cfg, log.ReleaseInfo{}, log.WithExtraHandler(wrap))
	require.NoError(t, err)
	require.NotNil(t, l)

	l.InfoContext(context.Background(), "wrapped message")

	require.Len(t, captured, 1)
	assert.Equal(t, "wrapped message", captured[0])
}

// captureHandler 记录经过的每条日志消息，验证 WithExtraHandler 注入生效。
type captureHandler struct {
	next     slog.Handler
	captured *[]string
}

func (h *captureHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *captureHandler) Handle(ctx context.Context, r slog.Record) error {
	*h.captured = append(*h.captured, r.Message)
	return h.next.Handle(ctx, r)
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &captureHandler{next: h.next.WithAttrs(attrs), captured: h.captured}
}

func (h *captureHandler) WithGroup(name string) slog.Handler {
	return &captureHandler{next: h.next.WithGroup(name), captured: h.captured}
}
```

Add `"log/slog"` to the import block at the top of `go-common/log/new_logger_test.go` (not yet imported there).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./go-common/log/... -run TestNewLogger_WithExtraHandler -v`
Expected: FAIL with a compile error (`log.WithExtraHandler` / `LoggerOption` undefined).

- [ ] **Step 3: Implement `LoggerOption` and `WithExtraHandler`, thread through `NewLogger`**

In `go-common/log/new_logger.go`, add near the top (after imports, before `NewLogger`):

```go
// loggerBuildOptions 收集 NewLogger 构建过程的可选项。
type loggerBuildOptions struct {
	extraHandlers []func(slog.Handler) slog.Handler
}

// LoggerOption 配置 NewLogger 构建过程的可选项。
type LoggerOption func(*loggerBuildOptions)

// WithExtraHandler 注入一个自定义 handler 包装函数，应用在标准 handler 链
// （output → context → release → mask）构建完成之后，作为最外层 handler。
// 可多次调用，按调用顺序层层包装（最后一次调用的在最外层，最先执行）。
func WithExtraHandler(wrap func(slog.Handler) slog.Handler) LoggerOption {
	return func(o *loggerBuildOptions) {
		if wrap != nil {
			o.extraHandlers = append(o.extraHandlers, wrap)
		}
	}
}
```

Then change the `NewLogger` signature and apply the options at the end, right before constructing the returned `*Logger`:

```go
// NewLogger 使用新 Config 和 ReleaseInfo 创建 Logger，支持可选的 LoggerOption。
func NewLogger(cfg Config, release ReleaseInfo, opts ...LoggerOption) (*Logger, error) {
	o := &loggerBuildOptions{}
	for _, opt := range opts {
		opt(o)
	}

	var handler slog.Handler
	// ... existing body unchanged down to the final `handler = NewMaskHandler(handler, masker)` block ...

	for _, wrap := range o.extraHandlers {
		handler = wrap(handler)
	}

	return &Logger{
		Logger: slog.New(handler),
		level:  parseLevel(cfg.Level),
	}, nil
}
```

Only the function signature line, the `o := &loggerBuildOptions{}` / options-apply block at the top, and the `for _, wrap := range o.extraHandlers` loop right before the `return` are new — everything else in the existing function body (output handler switch, context/release/mask handler wiring, the `Categories` warning) stays exactly as-is.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./go-common/log/... -run TestNewLogger_WithExtraHandler -v`
Expected: PASS.

- [ ] **Step 5: Run the full package test suite**

Run: `go test ./go-common/log/... -count=1`
Expected: PASS — confirms existing 2-arg `NewLogger(cfg, release)` call sites (used throughout `new_logger_test.go`) still compile and pass unchanged.

- [ ] **Step 6: Commit**

```bash
git add go-common/log/new_logger.go go-common/log/new_logger_test.go
git commit -m "feat(log): add WithExtraHandler option to NewLogger for custom handler injection (#108)"
```

---

### Task 4: Update package-level docs for OTel/framework integration

**Files:**
- Modify: `go-common/log/logger.go:1-19` (package doc comment)

**Interfaces:**
- Consumes: nothing (docs-only).
- Produces: nothing (docs-only).

- [ ] **Step 1: Update the package doc comment**

In `go-common/log/logger.go`, the package comment currently reads (top of file):

```go
// Package log 提供基于 Go 标准库 log/slog 的结构化日志封装。
//
// 核心功能：
//   - 文件轮转（lumberjack）
//   - 自动 OTel span 关联（TraceID/SpanID 注入）
//   - klog/hlog 适配器
//
// 用法：
//
//	// Options 模式（推荐）
//	l := log.New(
//	    log.WithLevel("info"),
//	    log.WithFilePath("/var/log/app.log"),
//	)
//
//	// Config 模式（YAML 加载场景）
//	l := log.NewFromConfig(cfg)
//
//	l.Info("server started", "port", 8080)
//	l.Error("something failed", "error", err)
package log
```

Replace it with (documents that both construction paths behave the same way now, and explains the framework integration contract from the design doc):

```go
// Package log 提供基于 Go 标准库 log/slog 的结构化日志封装。
//
// 核心功能：
//   - 文件轮转（lumberjack）
//   - 自动 OTel span 关联：调用方传入的 context.Context 中若存在活跃 OTel
//     span（例如经过 go-framework/{hertz,kitex}/observability 中间件处理的
//     请求 ctx），日志会自动带上 trace_id/span_id，无需任何额外接线；没有
//     活跃 span 时回退读取手工设置的 ContextKeyTraceID/ContextKeySpanID
//   - klog/hlog 适配器
//
// New()/NewFromLegacyConfig()（Options/Legacy 模式）与 NewLogger()（Config
// 模式）两条构造路径行为一致，都会注入 request_id/trace_id/span_id。
//
// 用法：
//
//	// Options 模式
//	l := log.New(
//	    log.WithLevel("info"),
//	    log.WithFilePath("/var/log/app.log"),
//	)
//
//	// Config 模式（YAML 加载场景）
//	l, err := log.NewLogger(cfg, release)
//
//	l.Info("server started", "port", 8080)
//	l.Error("something failed", "error", err)
package log
```

- [ ] **Step 2: Verify the package still builds and vets clean**

Run: `go build ./go-common/... && go vet ./go-common/...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add go-common/log/logger.go
git commit -m "docs(log): document unified OTel trace association across both Logger constructors (#108)"
```

---

## Final Validation

After all four tasks are complete, run the full workspace validation from `.claude/rules/agent-engineering.md` §6:

```bash
go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...
go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...
gofmt -l go-common/log/*.go
golangci-lint run --timeout=5m ./go-common/...
go test ./go-common/... -count=1
```

Cross-check against the design doc's acceptance criteria table
(`docs/superpowers/specs/2026-09-18-log-otel-trace-context-design.md` § 验收标准对照)
— all six boxes should now be satisfiable.
