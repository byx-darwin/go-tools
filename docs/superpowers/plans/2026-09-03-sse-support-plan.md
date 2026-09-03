# SSE (Server-Sent Events) Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `go-framework/hertz/sse` package that wraps Hertz's native SSE writer with Request-ID reuse, panic recovery, heartbeat keep-alive and disconnect detection; assess (do not extend) Kitex middleware compatibility with streaming RPC; ship runnable examples for both.

**Architecture:** Hertz already ships `github.com/cloudwego/hertz/pkg/protocol/sse` (`Writer`/`Event`, full SSE protocol encode/flush). `go-framework/hertz/sse.Writer` is a thin wrapper around it that adds framework conventions (Request ID via `hertz.RequestIDFrom(ctx)`, panic→`event:error` recovery, background heartbeat+disconnect goroutine). Kitex streaming is IDL-generated per service method (`pkg/streaming`), so the Kitex side produces no new exported API — only a compatibility write-up plus a working streaming example wired through the existing `auth.Recovery()` middleware.

**Tech Stack:** Go 1.26 workspace (`go-framework` module), `github.com/cloudwego/hertz v0.10.5` (native `pkg/protocol/sse`), `github.com/cloudwego/kitex v0.16.2` (`pkg/streaming`, protobuf `stream` IDL), `testify` for assertions, `github.com/byx-darwin/go-tools/go-common/log` for structured logging.

**Spec:** `docs/superpowers/specs/2026-09-03-sse-support-design.md`

## Global Constraints

- New Go code MUST be `gofmt`-clean and pass `golangci-lint run` for the `go-framework` module (`.golangci.yml`, v2 rules — see `.claude/rules/go.md` §8).
- Every exported type/func/const MUST have a `// Name ...` godoc comment (revive rule).
- Constructors with 3+ params or configs with 5+ fields MUST use the Functional Options pattern (`.claude/rules/options-pattern.md`); `sse.Writer` config qualifies.
- `go-framework` MUST NOT import `go-middleware` (sibling boundary) and MUST NOT introduce a circular dependency.
- Do not modify `Responder`'s existing behavior — `sse` is an additive, independent subpackage.
- Kitex side produces **no new exported API** — compatibility assessment + example only (confirmed scope narrowing, Issue #97 discussion).
- Error codes: this task does not need new `go-framework/error` codes — the SSE error event payload is a local `{code, msg, data}` JSON struct, not an `oops`-routed HTTP error.
- Octal literals use `0o` form; `//nolint:` comments (if any) MUST carry a reason.

---

### Task 1: SSE error event payload (`error.go`)

**Files:**
- Create: `go-framework/hertz/sse/error.go`
- Test: `go-framework/hertz/sse/error_test.go`

**Interfaces:**
- Consumes: `github.com/cloudwego/hertz/pkg/protocol/sse` (`*sse.Writer`, `sse.Writer.WriteEvent`)
- Produces: `writeErrorEvent(w *sse.Writer, code int, msg string) error` — used by Task 4's panic recovery path.

- [ ] **Step 1: Write the failing test**

Hertz's `sse.Writer` has no public flusher-injection constructor for isolated unit testing, so build it through the real `hertzsse.NewWriter(c)` path using `ut.CreateUtRequestContext` — the same pattern `response_integration_test.go` uses for `Responder`.

```go
// go-framework/hertz/sse/error_test.go
package sse

import (
	"bytes"
	"encoding/json"
	"testing"

	hertzsse "github.com/cloudwego/hertz/pkg/protocol/sse"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteErrorEvent(t *testing.T) {
	c := ut.CreateUtRequestContext("GET", "/sse", nil)
	w := hertzsse.NewWriter(c)

	err := writeErrorEvent(w, 500, "internal server error")
	require.NoError(t, err)
	require.NoError(t, w.Close())

	body := c.Response.BodyStream()
	require.NotNil(t, body)
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(body)
	require.NoError(t, err)
	out := buf.String()

	assert.Contains(t, out, "event: error\n")

	const prefix = "data: "
	idx := bytes.Index(buf.Bytes(), []byte(prefix))
	require.GreaterOrEqual(t, idx, 0)
	rest := out[idx+len(prefix):]
	line := rest[:bytes.IndexByte([]byte(rest), '\n')]

	var payload sseErrorPayload
	require.NoError(t, json.Unmarshal([]byte(line), &payload))
	assert.Equal(t, 500, payload.Code)
	assert.Equal(t, "internal server error", payload.Msg)
	assert.Nil(t, payload.Data)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go-framework && go test ./hertz/sse/... -run TestWriteErrorEvent -v`
Expected: FAIL — `undefined: writeErrorEvent`, `undefined: sseErrorPayload` (package `sse` doesn't exist yet under `go-framework/hertz/sse`).

If `c.Response.BodyStream()` returns `nil` for a chunked/hijacked writer in this Hertz version, fall back to reading via `c.Response.GetHijackWriter()` type assertion, or use `ut.PerformRequest` with a route handler (mirrors `response_integration_test.go`'s `setupHertzEngine` pattern) instead of `ut.CreateUtRequestContext` directly. Confirm which one actually surfaces bytes with the installed `github.com/cloudwego/hertz v0.10.5` — the CI test run in Step 4 is the source of truth; adjust the harness (not the production code) until the byte stream is observable.

- [ ] **Step 3: Write minimal implementation**

```go
// go-framework/hertz/sse/error.go
package sse

import (
	"encoding/json"

	hertzsse "github.com/cloudwego/hertz/pkg/protocol/sse"
)

// sseErrorPayload SSE 错误事件负载，对齐 Response 三段式（code/msg/data）。
type sseErrorPayload struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

// writeErrorEvent 写入一条 event:error，data 为 JSON 序列化的三段式结构。
// SSE 连接建立后响应头已提交为 text/event-stream，无法再切回 JSON/Protobuf
// 响应，因此错误一律走此事件格式，不复用 Responder.Error() 的内容协商逻辑。
func writeErrorEvent(w *hertzsse.Writer, code int, msg string) error {
	payload := sseErrorPayload{Code: code, Msg: msg}
	data, err := json.Marshal(payload)
	if err != nil {
		data = []byte(`{"code":` + itoa(code) + `,"msg":"internal server error"}`)
	}
	return w.WriteEvent("", "error", data)
}

// itoa 避免为单个整数转字符串引入 strconv 之外的依赖开销时的降级兜底。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go-framework && go test ./hertz/sse/... -run TestWriteErrorEvent -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-framework/hertz/sse/error.go go-framework/hertz/sse/error_test.go
git commit -m "feat(hertz/sse): add SSE error event payload writer"
```

---

### Task 2: Functional Options (`options.go`)

**Files:**
- Create: `go-framework/hertz/sse/options.go`
- Test: `go-framework/hertz/sse/options_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `type Option func(*config)`, `type config struct { heartbeatInterval time.Duration; onRecover func(rec any) }`, `WithHeartbeatInterval(d time.Duration) Option`, `WithRecoverHandler(fn func(rec any)) Option`, `defaultConfig() config` — consumed by Task 3's `NewWriter`.

- [ ] **Step 1: Write the failing test**

```go
// go-framework/hertz/sse/options_test.go
package sse

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	c := defaultConfig()
	assert.Equal(t, 15*time.Second, c.heartbeatInterval)
	assert.Nil(t, c.onRecover)
}

func TestWithHeartbeatInterval(t *testing.T) {
	c := defaultConfig()
	WithHeartbeatInterval(30 * time.Second)(&c)
	assert.Equal(t, 30*time.Second, c.heartbeatInterval)

	// <=0 disables heartbeat.
	WithHeartbeatInterval(0)(&c)
	assert.Equal(t, time.Duration(0), c.heartbeatInterval)

	WithHeartbeatInterval(-1 * time.Second)(&c)
	assert.Equal(t, time.Duration(-1*time.Second), c.heartbeatInterval)
}

func TestWithRecoverHandler(t *testing.T) {
	c := defaultConfig()
	called := false
	WithRecoverHandler(func(rec any) { called = true })(&c)
	assert.NotNil(t, c.onRecover)
	c.onRecover("boom")
	assert.True(t, called)

	// nil handler is a no-op, doesn't clear existing handler.
	WithRecoverHandler(nil)(&c)
	assert.NotNil(t, c.onRecover)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go-framework && go test ./hertz/sse/... -run 'TestDefaultConfig|TestWithHeartbeatInterval|TestWithRecoverHandler' -v`
Expected: FAIL — `undefined: defaultConfig`, `undefined: WithHeartbeatInterval`, `undefined: WithRecoverHandler`.

- [ ] **Step 3: Write minimal implementation**

```go
// go-framework/hertz/sse/options.go
package sse

import "time"

// defaultHeartbeatInterval 默认心跳保活间隔。
const defaultHeartbeatInterval = 15 * time.Second

// config Writer 内部配置，由 Option 填充。
type config struct {
	heartbeatInterval time.Duration
	onRecover         func(rec any)
}

// defaultConfig 返回默认配置：heartbeatInterval=15s，onRecover=nil（仅记录日志）。
func defaultConfig() config {
	return config{heartbeatInterval: defaultHeartbeatInterval}
}

// Option 定义 sse.Writer 配置选项。
type Option func(*config)

// WithHeartbeatInterval 设置心跳保活间隔。<=0 禁用心跳 goroutine。默认 15s。
func WithHeartbeatInterval(d time.Duration) Option {
	return func(c *config) { c.heartbeatInterval = d }
}

// WithRecoverHandler 设置自定义 panic 上报回调（如埋点/告警），
// 在写入 event:error 之前调用。传入 nil 保持已有配置不变，默认仅记录结构化日志。
func WithRecoverHandler(fn func(rec any)) Option {
	return func(c *config) {
		if fn != nil {
			c.onRecover = fn
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go-framework && go test ./hertz/sse/... -run 'TestDefaultConfig|TestWithHeartbeatInterval|TestWithRecoverHandler' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-framework/hertz/sse/options.go go-framework/hertz/sse/options_test.go
git commit -m "feat(hertz/sse): add functional options for Writer config"
```

---

### Task 3: `Writer` core — `NewWriter`, `WriteEvent`, `Close` (`writer.go`)

**Files:**
- Create: `go-framework/hertz/sse/writer.go`
- Test: `go-framework/hertz/sse/writer_test.go`

**Interfaces:**
- Consumes: `hertzsse.NewWriter(c *app.RequestContext) *hertzsse.Writer`, `hertz.RequestIDFrom(ctx) string` (from `github.com/byx-darwin/go-tools/go-framework/hertz`), `config`/`defaultConfig`/`Option` (Task 2), `writeErrorEvent` (Task 1).
- Produces: `type Writer struct{...}`, `NewWriter(c context.Context, rc *app.RequestContext, opts ...Option) *Writer`, `(w *Writer) WriteEvent(id, eventType string, data []byte) error`, `(w *Writer) Close() error`, sentinel `var ErrWriterClosed = errors.New("sse: writer closed")` — consumed by Task 4 (`Run`, heartbeat goroutine) and by example code in Task 9.

**Design note carried into code:** `Writer` tracks a `closed atomic.Bool` so `WriteEvent` short-circuits with `ErrWriterClosed` once `Close()` has run (from panic recovery, explicit call, or the Task 4 disconnect-detection goroutine) — this makes the "next write fails after disconnect" behavior deterministic and testable without depending on the underlying transport's error semantics.

- [ ] **Step 1: Write the failing test**

```go
// go-framework/hertz/sse/writer_test.go
package sse

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
)

func drainBody(t *testing.T, c interface{ BodyStream() interface {
	Read([]byte) (int, error)
} }) string {
	t.Helper()
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(c.BodyStream())
	require.NoError(t, err)
	return buf.String()
}

func TestNewWriter_SetsSSEHeaders(t *testing.T) {
	c := ut.CreateUtRequestContext("GET", "/sse", nil)
	w := NewWriter(context.Background(), c)
	require.NotNil(t, w)
	require.NoError(t, w.Close())

	assert.Equal(t, "text/event-stream; charset=utf-8", string(c.Response.Header.Peek("Content-Type")))
	assert.Equal(t, "no-cache", string(c.Response.Header.Peek("Cache-Control")))
}

func TestWriter_WriteEvent_Success(t *testing.T) {
	c := ut.CreateUtRequestContext("GET", "/sse", nil)
	w := NewWriter(context.Background(), c)

	err := w.WriteEvent("1", "message", []byte("hello"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	body := drainBody(t, c)
	assert.Contains(t, body, "id: 1\n")
	assert.Contains(t, body, "event: message\n")
	assert.Contains(t, body, "data: hello\n")
}

func TestWriter_WriteEvent_AfterClose_ReturnsErrWriterClosed(t *testing.T) {
	c := ut.CreateUtRequestContext("GET", "/sse", nil)
	w := NewWriter(context.Background(), c)
	require.NoError(t, w.Close())

	err := w.WriteEvent("1", "message", []byte("hello"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrWriterClosed))
}

func TestWriter_Close_Idempotent(t *testing.T) {
	c := ut.CreateUtRequestContext("GET", "/sse", nil)
	w := NewWriter(context.Background(), c)
	require.NoError(t, w.Close())
	require.NoError(t, w.Close()) // second Close must not panic or error
}

func TestNewWriter_RequestIDRequiresResponderMiddleware(t *testing.T) {
	// Without Responder.Middleware() having run, hertz.RequestIDFrom(ctx)
	// returns "" — NewWriter must not panic and must degrade silently.
	c := ut.CreateUtRequestContext("GET", "/sse", nil)
	w := NewWriter(context.Background(), c)
	require.NotNil(t, w)
	assert.Equal(t, "", hertzresp.RequestIDFrom(c))
	require.NoError(t, w.Close())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go-framework && go test ./hertz/sse/... -run 'TestNewWriter|TestWriter_' -v`
Expected: FAIL — `undefined: NewWriter`, `undefined: ErrWriterClosed`.

If `drainBody`'s type constraint doesn't match `*app.RequestContext.Response`'s actual `BodyStream()` return type in this Hertz version, replace the helper with a direct `io.ReadAll(c.Response.BodyStream())` call (adjust the import to `io`) — keep the test's assertions unchanged, only the plumbing to read the recorded chunked body.

- [ ] **Step 3: Write minimal implementation**

```go
// go-framework/hertz/sse/writer.go
// Package sse 在 Hertz 官方 pkg/protocol/sse 之上提供一层贴合 go-framework
// 现有 Responder/中间件规范的封装：Request ID 复用、panic recovery、心跳
// 保活、断连检测。
//
// 前置条件：ctx 所在请求链路必须已经过 hertz.Responder.Middleware()，
// 否则 Request ID 特性静默失效（hertz.RequestIDFrom(ctx) 返回空字符串，
// 不会 panic 或报错）。
package sse

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/cloudwego/hertz/pkg/app"
	hertzsse "github.com/cloudwego/hertz/pkg/protocol/sse"

	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
)

// ErrWriterClosed 表示 Writer 已关闭（客户端断连或业务主动 Close），
// 后续 WriteEvent 调用会立即返回此错误。
var ErrWriterClosed = errors.New("sse: writer closed")

// Writer 封装 Hertz 原生 SSE Writer，集成 Request ID、panic recovery、
// 心跳保活、断连检测，对齐 Responder 规范。
type Writer struct {
	w         *hertzsse.Writer
	cfg       config
	requestID string
	closed    atomic.Bool

	cancelHeartbeat context.CancelFunc
	heartbeatDone   chan struct{}
}

// NewWriter 创建 SSE Writer，立即写入 SSE 响应头
// （Content-Type: text/event-stream; charset=utf-8，Cache-Control: no-cache）。
//
// 默认配置：
//   - heartbeatInterval: 15 * time.Second
//   - onRecover: nil（仅记录结构化日志）
func NewWriter(c context.Context, rc *app.RequestContext, opts ...Option) *Writer {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Writer{
		w:         hertzsse.NewWriter(rc),
		cfg:       cfg,
		requestID: hertzresp.RequestIDFrom(rc),
	}
}

// WriteEvent 写入一条 SSE 事件（透传 hertz sse.Writer.WriteEvent）。
// Writer 已关闭时立即返回 ErrWriterClosed；断连或写入失败时返回底层错误，
// 调用方应据此退出事件循环。
func (w *Writer) WriteEvent(id, eventType string, data []byte) error {
	if w.closed.Load() {
		return ErrWriterClosed
	}
	return w.w.WriteEvent(id, eventType, data)
}

// Close 关闭连接，停止心跳 goroutine（若已启动）。幂等：多次调用安全。
func (w *Writer) Close() error {
	if !w.closed.CompareAndSwap(false, true) {
		return nil
	}
	if w.cancelHeartbeat != nil {
		w.cancelHeartbeat()
		<-w.heartbeatDone
	}
	return w.w.Close()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go-framework && go test ./hertz/sse/... -run 'TestNewWriter|TestWriter_' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-framework/hertz/sse/writer.go go-framework/hertz/sse/writer_test.go
git commit -m "feat(hertz/sse): add Writer with Request ID reuse and closed-state tracking"
```

---

### Task 4: `Run` — heartbeat, disconnect detection, panic recovery

**Files:**
- Modify: `go-framework/hertz/sse/writer.go` (add `Run` method + heartbeat goroutine)
- Modify: `go-framework/hertz/sse/writer_test.go` (add `Run`-specific tests)

**Interfaces:**
- Consumes: `Writer` fields from Task 3, `writeErrorEvent` (Task 1), `go-common/log` (`log.L().WithCategory(log.CategoryPanic)`, `.ErrorContext`).
- Produces: `(w *Writer) Run(handler func(w *Writer) error) error` — this is the primary entry point documented for business use in Task 9's example.

- [ ] **Step 1: Write the failing test**

```go
// appended to go-framework/hertz/sse/writer_test.go
package sse

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriter_Run_HandlerCompletesNormally(t *testing.T) {
	c := ut.CreateUtRequestContext("GET", "/sse", nil)
	w := NewWriter(context.Background(), c, WithHeartbeatInterval(0))

	err := w.Run(func(w *Writer) error {
		return w.WriteEvent("1", "message", []byte("hi"))
	})
	require.NoError(t, err)

	body := drainBody(t, c)
	assert.Contains(t, body, "data: hi\n")
}

func TestWriter_Run_HandlerError_Propagates(t *testing.T) {
	c := ut.CreateUtRequestContext("GET", "/sse", nil)
	w := NewWriter(context.Background(), c, WithHeartbeatInterval(0))

	sentinel := errors.New("client gone")
	err := w.Run(func(w *Writer) error { return sentinel })
	assert.ErrorIs(t, err, sentinel)
}

func TestWriter_Run_PanicRecovered_WritesErrorEvent(t *testing.T) {
	c := ut.CreateUtRequestContext("GET", "/sse", nil)
	var recovered any
	w := NewWriter(context.Background(), c,
		WithHeartbeatInterval(0),
		WithRecoverHandler(func(rec any) { recovered = rec }),
	)

	err := w.Run(func(w *Writer) error {
		panic("boom")
	})
	require.NoError(t, err) // Run itself must not propagate the panic as an error

	body := drainBody(t, c)
	assert.Contains(t, body, "event: error\n")
	assert.Contains(t, body, `"code":500`)
	assert.Equal(t, "boom", recovered)
}

func TestWriter_Run_HeartbeatWritesKeepAlive(t *testing.T) {
	c := ut.CreateUtRequestContext("GET", "/sse", nil)
	w := NewWriter(context.Background(), c, WithHeartbeatInterval(5*time.Millisecond))

	var ticks atomic.Int32
	err := w.Run(func(w *Writer) error {
		time.Sleep(30 * time.Millisecond) // let >=1 heartbeat tick fire
		return nil
	})
	require.NoError(t, err)
	_ = ticks

	body := drainBody(t, c)
	assert.True(t, strings.Contains(body, ":keep-alive\n"), "expected at least one keep-alive comment, got: %q", body)
}

func TestWriter_Run_ContextCancel_ClosesWriter(t *testing.T) {
	c := ut.CreateUtRequestContext("GET", "/sse", nil)
	ctx, cancel := context.WithCancel(context.Background())
	w := NewWriter(ctx, c, WithHeartbeatInterval(5*time.Millisecond))

	done := make(chan struct{})
	go func() {
		_ = w.Run(func(w *Writer) error {
			<-done // block until the test cancels ctx and observes the effect
			return nil
		})
	}()

	cancel()
	require.Eventually(t, func() bool {
		return w.closed.Load()
	}, time.Second, 5*time.Millisecond, "writer should be closed after ctx cancellation")
	close(done)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go-framework && go test ./hertz/sse/... -run 'TestWriter_Run' -v`
Expected: FAIL — `undefined: w.Run` (and `w.closed` unexported field access works because the test is in-package `sse`).

- [ ] **Step 3: Write minimal implementation**

Append to `go-framework/hertz/sse/writer.go`:

```go
import (
	// ...existing imports...
	"fmt"
	"time"

	"github.com/byx-darwin/go-tools/go-common/log"
)

// Run 包装业务事件循环：内部启动心跳 + 断连检测 goroutine（heartbeatInterval
// <=0 时跳过心跳），函数返回前自动 Close。handler 内 panic 会被捕获：调用
// onRecover（若配置）→ 写入 event:error（500, "internal server error"）→
// 记录结构化日志 → 不重新抛出。handler 的返回值（含 nil）原样返回给调用方；
// panic 场景固定返回 nil，因为错误已经通过 SSE 事件流交付给客户端。
func (w *Writer) Run(handler func(w *Writer) error) (err error) {
	ctx, cancel := context.WithCancel(w.runCtx())
	w.cancelHeartbeat = cancel
	w.heartbeatDone = make(chan struct{})
	go w.heartbeatLoop(ctx)

	defer func() {
		if rec := recover(); rec != nil {
			if w.cfg.onRecover != nil {
				w.cfg.onRecover(rec)
			}
			log.L().WithCategory(log.CategoryPanic).ErrorContext(ctx, "sse handler panic recovered",
				fmt.Errorf("%v", rec),
				"request_id", w.requestID,
				"panic", fmt.Sprintf("%v", rec),
			)
			_ = writeErrorEvent(w.w, 500, "internal server error")
			err = nil
		}
		_ = w.Close()
	}()

	return handler(w)
}

// heartbeatLoop 后台心跳 + 断连检测 goroutine。
// heartbeatInterval<=0 时不创建 ticker，仅监听 ctx.Done() 用于断连检测。
func (w *Writer) heartbeatLoop(ctx context.Context) {
	defer close(w.heartbeatDone)

	if w.cfg.heartbeatInterval <= 0 {
		<-ctx.Done()
		_ = w.Close()
		return
	}

	ticker := time.NewTicker(w.cfg.heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := w.w.WriteKeepAlive(); err != nil {
				_ = w.Close()
				return
			}
		case <-ctx.Done():
			_ = w.Close()
			return
		}
	}
}

// runCtx 返回 Run 使用的断连检测 context；NewWriter 保存的原始 ctx 为空时
// （不应发生，防御性兜底）退化为 context.Background()。
func (w *Writer) runCtx() context.Context {
	if w.parentCtx != nil {
		return w.parentCtx
	}
	return context.Background()
}
```

Also update `Writer` struct (Task 3's definition) and `NewWriter` to retain the parent `ctx` for `Run`'s disconnect detection:

```go
type Writer struct {
	w         *hertzsse.Writer
	cfg       config
	requestID string
	closed    atomic.Bool
	parentCtx context.Context // retained for Run's disconnect-detection goroutine

	cancelHeartbeat context.CancelFunc
	heartbeatDone   chan struct{}
}
```

```go
func NewWriter(c context.Context, rc *app.RequestContext, opts ...Option) *Writer {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return &Writer{
		w:         hertzsse.NewWriter(rc),
		cfg:       cfg,
		requestID: hertzresp.RequestIDFrom(rc),
		parentCtx: c,
	}
}
```

Also make `Close()` safe to call before `Run()` (i.e., `cancelHeartbeat`/`heartbeatDone` may be nil if `Run` was never invoked) — this already holds from Task 3's `if w.cancelHeartbeat != nil` guard; no change needed there.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go-framework && go test ./hertz/sse/... -v`
Expected: PASS (all tests in the package, including Tasks 1-3's).

If `TestWriter_Run_ContextCancel_ClosesWriter` is flaky under `-race` due to the goroutine race between `cancel()` and the `Eventually` poll, that is expected — `require.Eventually` is intentionally used instead of a fixed sleep for this reason; do not weaken the assertion, only adjust the poll interval/timeout if needed.

- [ ] **Step 5: Run full package test with race detector**

Run: `cd go-framework && go test ./hertz/sse/... -race -v`
Expected: PASS, no data races reported.

- [ ] **Step 6: Commit**

```bash
git add go-framework/hertz/sse/writer.go go-framework/hertz/sse/writer_test.go
git commit -m "feat(hertz/sse): add Run with heartbeat, disconnect detection, panic recovery"
```

---

### Task 5: Package doc (`doc.go`) + lint/vet pass for `go-framework/hertz/sse`

**Files:**
- Create: `go-framework/hertz/sse/doc.go`

**Interfaces:**
- Consumes: nothing (documentation only).
- Produces: package-level godoc consumed by `go doc` / editors; no runtime symbol.

- [ ] **Step 1: Write the doc file**

```go
// go-framework/hertz/sse/doc.go
// Package sse 在 Hertz 官方 pkg/protocol/sse 之上提供一层贴合
// go-framework 现有 Responder/中间件规范的 SSE（Server-Sent Events）封装。
//
// # 快速开始
//
//	engine.Use(responder.Middleware()) // 必须先注册，SSE Request ID 依赖它
//	engine.GET("/stream", func(ctx context.Context, c *app.RequestContext) {
//	    w := sse.NewWriter(ctx, c, sse.WithHeartbeatInterval(15*time.Second))
//	    _ = w.Run(func(w *sse.Writer) error {
//	        for event := range dataCh {
//	            if err := w.WriteEvent("", "message", event); err != nil {
//	                return err // 客户端已断开，退出事件循环
//	            }
//	        }
//	        return nil
//	    })
//	})
//
// # 前置条件
//
// ctx 所在请求链路必须已经过 hertz.Responder.Middleware()，否则 Request ID
// 特性静默失效（hertz.RequestIDFrom(ctx) 返回空字符串，不 panic、不报错）。
//
// # 错误处理
//
// SSE 响应头一旦提交（NewWriter 内部立即完成），无法再切回 JSON/Protobuf，
// 因此错误一律走 event:error（JSON 三段式 {code,msg,data}），不复用
// Responder.Error() 的内容协商逻辑。Run 内部的 panic 会被捕获、转换为
// event:error 并记录结构化日志，不会导致进程崩溃或向上传播。
package sse
```

- [ ] **Step 2: Run module-level format, vet and lint**

Run: `cd go-framework && gofmt -l ./hertz/sse/ && go vet ./hertz/sse/... && golangci-lint run --timeout=5m ./hertz/sse/...`
Expected: `gofmt -l` prints nothing (all files formatted), `go vet` and `golangci-lint` report no issues.

If lint fails on missing godoc comments, unused imports, or import grouping (stdlib / third-party / project in three groups per `.claude/rules/go.md` §8.1), fix inline in the flagged file and re-run.

- [ ] **Step 3: Run the full package test suite one more time**

Run: `cd go-framework && go test ./hertz/sse/... -race -cover -v`
Expected: PASS, coverage report printed (no specific threshold enforced by this task, but every branch added in Tasks 1-4 should show as exercised).

- [ ] **Step 4: Commit**

```bash
git add go-framework/hertz/sse/doc.go
git commit -m "docs(hertz/sse): add package-level usage doc"
```

---

### Task 6: `go-framework` module-wide build validation

**Files:** none created/modified — validation only.

**Interfaces:** N/A.

- [ ] **Step 1: Build the whole `go-framework` module**

Run: `go build ./go-framework/...`
Expected: exits 0, no compile errors (confirms `sse` package doesn't break anything else in the module, e.g. no accidental import cycle with `go-framework/hertz`).

- [ ] **Step 2: Vet the whole module**

Run: `go vet ./go-framework/...`
Expected: exits 0.

- [ ] **Step 3: Lint the whole module**

Run: `golangci-lint run --timeout=5m ./go-framework/...`
Expected: exits 0. Fix any cross-package issues surfaced here that weren't visible when linting `./hertz/sse/...` alone (e.g. unused exports flagged by a wider analysis pass).

- [ ] **Step 4: Run the whole module's test suite**

Run: `go test ./go-framework/... -count=1`
Expected: PASS, no regressions in `go-framework/hertz`, `go-framework/kitex`, `go-framework/config`, etc.

- [ ] **Step 5: No commit needed** (validation-only task; if Step 3 required fixes, commit those under the relevant earlier task's scope instead, e.g. amend via a new small fix commit `fix(hertz/sse): address golangci-lint findings`).

---

### Task 7: Kitex streaming compatibility assessment (`go-framework/kitex/STREAMING.md`)

**Files:**
- Create: `go-framework/kitex/STREAMING.md`

**Interfaces:**
- Consumes: reads (does not modify) `go-framework/kitex/middleware/auth/{jwt,session,device,recovery}.go`, `go-framework/kitex/middleware/accesslog.go`, `go-framework/kitex/observability/suite.go`.
- Produces: a markdown document; no Go symbols. Cross-referenced by Task 8's example.

- [ ] **Step 1: Read the four middleware implementations to answer the three assessment questions from the spec**

Run (read-only, no test to write — this task is an investigation + write-up):
```bash
cat go-framework/kitex/middleware/auth/recovery.go
cat go-framework/kitex/middleware/auth/jwt.go
cat go-framework/kitex/middleware/accesslog.go
sed -n '1,120p' go-framework/kitex/observability/suite.go
```

Record findings against the three questions from the design doc:
1. Does the middleware assume the request is fully complete before the `endpoint.Middleware` closure returns? (accesslog timing)
2. Is token verification (auth middlewares) connection-establishment-time or per-frame?
3. Does `Recovery()`'s `defer recover()` cover panics raised during the streaming send loop (i.e., panics that happen *after* the middleware chain has called `next(ctx, req, resp)` but *inside* the streaming handler body, not just synchronously within `next`)?

- [ ] **Step 2: Write `STREAMING.md`**

```markdown
# Kitex Streaming RPC 中间件兼容性说明

本仓库 `go-framework/kitex` 现有中间件均基于 `endpoint.Middleware`
（`func(ctx, req, resp) error` 单次调用模型）。Kitex 的 streaming 能力由
IDL（Protobuf `stream` 关键字）驱动生成，`endpoint.Middleware` 包裹的是
"流的建立"（即 `next(ctx, req, resp)` 返回前）而非逐帧收发过程本身。
本文档记录现有中间件在 streaming 场景下的行为评估结论。

## 评估范围

- `middleware/auth/{jwt,session,device}.go` — 鉴权中间件
- `middleware/auth/recovery.go` — panic recovery 中间件
- `middleware/accesslog.go` — 访问日志中间件
- `observability/suite.go` — OTel Tracing/Metrics

## 结论

| 中间件 | streaming 场景行为 | 结论 |
|---|---|---|
| JWT/Session/Device Auth | Token 校验发生在 `next(ctx, req, resp)` 被调用**之前**，即流建立（握手）阶段，对整个流生效一次，不逐帧重复校验 | ✅ 兼容。语义等价于 gRPC/Kitex streaming 的标准做法（连接级鉴权），无需改动 |
| `Recovery()` | `defer recover()` 包裹 `next(ctx, req, resp)` 的调用栈；对于 server-streaming，`next` 对应的生成代码在 handler 内部循环调用 `stream.Send(...)`，该循环运行在 `next` 返回之前的同一 goroutine 调用栈内，因此 handler 内的 panic（含发送阶段）仍会被 `next` 的调用方（即本中间件）捕获 | ✅ 兼容，覆盖流式发送阶段的 panic |
| `accesslog.go` | 记录耗时的起止时间点包裹在 `next(ctx, req, resp)` 前后，对 streaming 而言测得的是"整个流从建立到结束"的总时长，而非单次请求延迟；这是预期行为（无更细粒度的逐帧计时需求），无需改动 | ✅ 兼容（语义随 streaming 自然调整，非缺陷） |
| `observability/suite.go` | Kitex `stats.Tracer` 由 Kitex 运行时在流生命周期的固定检点回调（`Start`/`Finish` 等），与是否 streaming 无关，`suite.Options()` 注入方式不变 | ✅ 兼容，无需改动 |

**结论：现有中间件全部兼容 streaming RPC 场景，本次不新增/修改任何导出 API。**
若未来出现细粒度逐帧鉴权/限流等新需求，应作为独立 Issue 评估，不在本次范围内。

## 使用示例

参见 `example/rpc/service.go` 的 `StreamEcho` 方法与 `example/rpc/client.go`
的流式调用示例，演示如何在 streaming 方法上组合 `auth.Recovery()` +
`observability` Suite。
```

- [ ] **Step 3: No automated test** (documentation-only artifact; validated by Task 8's runnable example and by human review during PR).

- [ ] **Step 4: Commit**

```bash
git add go-framework/kitex/STREAMING.md
git commit -m "docs(kitex): add streaming RPC middleware compatibility assessment"
```

---

### Task 8: Kitex streaming example — IDL, codegen, server + client wiring

**Files:**
- Modify: `example/idl/demo.proto` (add `StreamEcho` streaming RPC)
- Modify (regenerated): `example/kitex_generated/demo/**` (via `kitex` codegen)
- Modify: `example/rpc/service.go` (implement `StreamEcho`)
- Modify: `example/rpc/server.go` (wire `auth.Recovery()` middleware)
- Modify: `example/rpc/client.go` (add a `CallStreamEcho` helper demonstrating client-side consumption)
- Test: `example/rpc/service_test.go` (new, or extend if one already exists — check first)

**Interfaces:**
- Consumes: regenerated `demo.StreamDemo_StreamEchoServer`-equivalent (exact generated type name TBD by codegen output — inspect after Step 2 and use the actual generated name, do not guess) from `example/kitex_generated/demo`; `go-framework/kitex/middleware/auth.Recovery()`.
- Produces: a runnable streaming RPC endpoint demonstrating `Recovery()` middleware compatibility, referenced from `STREAMING.md` (Task 7).

- [ ] **Step 1: Extend the proto IDL**

Edit `example/idl/demo.proto`, add after the existing `DemoService` block:

```protobuf
service DemoService {
  rpc Echo(EchoRequest) returns (EchoResponse);
  rpc Health(HealthRequest) returns (HealthResponse);
  rpc StreamEcho(EchoRequest) returns (stream EchoResponse);
}
```

(This adds one RPC to the existing service rather than a new service, keeping the client wiring in `example/rpc/client.go` simple — one `NewDemoClient` call site continues to work.)

- [ ] **Step 2: Regenerate Kitex stubs**

Run (from `example/`, matching the existing `Makefile` target):
```bash
cd example && kitex -module github.com/byx-darwin/go-tools/example -I idl idl/demo.proto
```
Expected: regenerates `example/kitex_generated/demo/**` in place, adding a `StreamEcho` method to the generated server/client interfaces. Inspect the diff (`git diff example/kitex_generated/`) and note the exact generated method signature for `StreamEcho` on both the server interface (e.g. `StreamEcho(req *demo.EchoRequest, stream demo.DemoService_StreamEchoServer) error`) and the client interface (e.g. `StreamEcho(ctx context.Context, req *demo.EchoRequest, callOptions ...callopt.Option) (demo.DemoService_StreamEchoClient, error)`) — use these exact names in Steps 3-4, do not assume names from unrelated Kitex versions.

- [ ] **Step 3: Implement the server-streaming handler**

Edit `example/rpc/service.go`, add:

```go
// StreamEcho 按 req.Message 中的字符逐个发送流式响应，演示 server-streaming。
// 每帧之间 sleep 10ms，便于客户端观测到多帧到达。
func (s *DemoServiceImpl) StreamEcho(req *demo.EchoRequest, stream demo.DemoService_StreamEchoServer) error {
	msg := req.GetMessage()
	if msg == "" {
		msg = "hello"
	}
	for i, r := range msg {
		if err := stream.Send(&demo.EchoResponse{
			Message: string(r),
			Service: "go-tools-example",
		}); err != nil {
			return err
		}
		_ = i
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}
```

Add `"time"` to the import block. Use the exact generated interface type name confirmed in Step 2 (`demo.DemoService_StreamEchoServer` is the expected Kitex naming convention: `<Service>_<Method>Server`, but verify against the actual generated file `example/kitex_generated/demo/*.go` before finalizing).

- [ ] **Step 4: Wire `auth.Recovery()` into the server middleware chain**

Edit `example/rpc/server.go`:

```go
import (
	// ...existing imports...
	kitexauth "github.com/byx-darwin/go-tools/go-framework/kitex/middleware/auth"
)

func StartServer(ctx context.Context, addr string, obsProvider *kitexobs.Provider) error {
	handler := &DemoServiceImpl{}

	var opts []server.Option
	opts = append(opts, server.WithServiceAddr(&net.TCPAddr{Port: extractPort(addr)}))
	opts = append(opts, server.WithMiddleware(kitexauth.Recovery()))

	// ...rest unchanged...
}
```

This demonstrates `Recovery()` composing with the new `StreamEcho` streaming method, per `STREAMING.md`'s compatibility claim.

- [ ] **Step 5: Add a client-side streaming call helper**

Edit `example/rpc/client.go`, add:

```go
// CallStreamEcho 演示流式客户端调用：逐帧接收 StreamEcho 响应直至 io.EOF。
func CallStreamEcho(ctx context.Context, c demoservice.Client, message string) ([]string, error) {
	stream, err := c.StreamEcho(ctx, &demo.EchoRequest{Message: message})
	if err != nil {
		return nil, fmt.Errorf("open StreamEcho: %w", err)
	}
	var frames []string
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return frames, fmt.Errorf("recv StreamEcho frame: %w", err)
		}
		frames = append(frames, resp.GetMessage())
	}
	return frames, nil
}
```

Add `"context"`, `"errors"`, `"io"` and `demo "github.com/byx-darwin/go-tools/example/kitex_generated/demo"` to the import block (verify the exact client stream type/method names — `stream.Recv()` and `io.EOF`-terminated loop — against the generated code from Step 2; Kitex protobuf streaming follows the gRPC convention here, but confirm before finalizing).

- [ ] **Step 6: Write an integration test exercising the full loop**

Check first whether `example/rpc/` already has a `*_test.go` file (`ls example/rpc/*_test.go`) — if one exists, add to it; otherwise create `example/rpc/service_test.go`:

```go
package rpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStreamEcho_EndToEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := "127.0.0.1:18888" // fixed test port; adjust if it collides with another test in this package
	go func() { _ = StartServer(ctx, addr, nil) }()
	time.Sleep(100 * time.Millisecond) // wait for server to bind

	cli, err := NewDemoClient(addr, nil)
	require.NoError(t, err)

	frames, err := CallStreamEcho(context.Background(), cli, "hi")
	require.NoError(t, err)
	require.Equal(t, []string{"h", "i"}, frames)
}
```

- [ ] **Step 7: Run the test**

Run: `cd example && go test ./rpc/... -run TestStreamEcho_EndToEnd -v`
Expected: PASS. If port `18888` collides with an existing test in the same package, pick an unused port (check `example/rpc/*_test.go` and `example/config.yaml` for ports already in use first).

- [ ] **Step 8: Build the whole example module**

Run: `cd example && go build ./...`
Expected: exits 0 — confirms the regenerated `kitex_generated` code and new handler/client code all compile together.

- [ ] **Step 9: Commit**

```bash
git add example/idl/demo.proto example/kitex_generated/ example/rpc/service.go example/rpc/server.go example/rpc/client.go example/rpc/service_test.go
git commit -m "feat(example): add StreamEcho demo wired through kitex auth.Recovery()"
```

---

### Task 9: Hertz SSE example handler

**Files:**
- Create: `example/handler/hertz_sse.go`
- Modify: `main.go` (register the SSE route)
- Test: `example/handler/hertz_sse_test.go`

**Interfaces:**
- Consumes: `go-framework/hertz/sse.NewWriter`, `sse.WithHeartbeatInterval`, `(*sse.Writer).Run`, `(*sse.Writer).WriteEvent`.
- Produces: `HandleSSEDemo(ctx context.Context, c *app.RequestContext)` — an `app.HandlerFunc`-compatible handler, registered at `GET /sse/demo`.

- [ ] **Step 1: Write the failing test**

```go
// example/handler/hertz_sse_test.go
package handler

import (
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
)

func TestHandleSSEDemo(t *testing.T) {
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(hertzresp.NewResponder().Middleware())
	engine.GET("/sse/demo", HandleSSEDemo)

	w := ut.PerformRequest(engine, http.MethodGet, "/sse/demo?message=hi&count=2", nil)
	require.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "text/event-stream") // via response header assertion below, not body
	assert.Contains(t, body, "event: message\n")
	assert.Contains(t, body, "data: hi-0\n")
	assert.Contains(t, body, "data: hi-1\n")
	assert.Equal(t, "text/event-stream; charset=utf-8", w.Header().Get("Content-Type"))
}
```

Fix the misleading `assert.Contains(t, body, "text/event-stream")` line (body won't contain the header) — remove it, keep only the `w.Header().Get("Content-Type")` assertion:

```go
func TestHandleSSEDemo(t *testing.T) {
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(hertzresp.NewResponder().Middleware())
	engine.GET("/sse/demo", HandleSSEDemo)

	w := ut.PerformRequest(engine, http.MethodGet, "/sse/demo?message=hi&count=2", nil)
	require.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, "event: message\n")
	assert.Contains(t, body, "data: hi-0\n")
	assert.Contains(t, body, "data: hi-1\n")
	assert.Equal(t, "text/event-stream; charset=utf-8", w.Header().Get("Content-Type"))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd example && go test ./handler/... -run TestHandleSSEDemo -v`
Expected: FAIL — `undefined: HandleSSEDemo`.

- [ ] **Step 3: Write minimal implementation**

```go
// example/handler/hertz_sse.go
// Package handler（本文件）演示 go-framework/hertz/sse 的用法：
// 一个按 query 参数 count 循环推送 message-i 事件的 SSE demo 端点。
package handler

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/byx-darwin/go-tools/go-framework/hertz/sse"
)

// HandleSSEDemo 处理 GET /sse/demo?message=<str>&count=<n>。
// 依次推送 count 个 message 事件（data 为 "<message>-<i>"），
// 演示 sse.Writer 的基本用法与 Request ID 复用（依赖上游已注册
// hertz.Responder.Middleware()）。
func HandleSSEDemo(ctx context.Context, c *app.RequestContext) {
	message := string(c.Query("message"))
	if message == "" {
		message = "hello"
	}
	count := 3
	if raw := string(c.Query("count")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			count = n
		}
	}

	w := sse.NewWriter(ctx, c)
	_ = w.Run(func(w *sse.Writer) error {
		for i := 0; i < count; i++ {
			data := []byte(fmt.Sprintf("%s-%d", message, i))
			if err := w.WriteEvent("", "message", data); err != nil {
				return err
			}
		}
		return nil
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd example && go test ./handler/... -run TestHandleSSEDemo -v`
Expected: PASS

- [ ] **Step 5: Register the route in `main.go`**

Read `main.go`'s existing route registration section (find where `h.GET(...)` / `examplemw.RegisterMiddleware` calls live) and add, alongside the other top-level routes:

```go
h.GET("/sse/demo", handler.HandleSSEDemo)
```

Place it near other non-protected `handler.*` route registrations in `main.go` (search for the existing `h.GET("/health"` or similar top-level route to find the right block — do not create a new registration function for a single route).

- [ ] **Step 6: Build and run the example module**

Run: `cd example && go build ./...`
Expected: exits 0.

- [ ] **Step 7: Commit**

```bash
git add example/handler/hertz_sse.go example/handler/hertz_sse_test.go main.go
git commit -m "feat(example): add Hertz SSE demo handler using go-framework/hertz/sse"
```

---

### Task 10: `example` module full validation

**Files:** none — validation only.

- [ ] **Step 1: Build**

Run: `cd example && go build ./...`
Expected: exits 0.

- [ ] **Step 2: Vet**

Run: `cd example && go vet ./...`
Expected: exits 0.

- [ ] **Step 3: Test**

Run: `cd example && go test ./... -count=1`
Expected: PASS (includes Task 8's `TestStreamEcho_EndToEnd` and Task 9's `TestHandleSSEDemo`).

- [ ] **Step 4: gofmt check across touched files**

Run: `gofmt -l example/handler/hertz_sse.go example/handler/hertz_sse_test.go example/rpc/service.go example/rpc/server.go example/rpc/client.go example/rpc/service_test.go example/idl/../main.go`
Expected: no output (all clean). If any file is listed, run `gofmt -w <file>` and re-verify.

- [ ] **Step 5: No commit needed** (validation-only; fold any fixes into the relevant task's commit scope with a small follow-up commit if needed).

---

### Task 11: Workspace-wide final validation

**Files:** none — validation only. This is the CI-equivalent full check from `CLAUDE.md`.

- [ ] **Step 1: Build everything**

Run: `go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`
Expected: exits 0.

- [ ] **Step 2: Vet everything**

Run: `go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...`
Expected: exits 0.

- [ ] **Step 3: Lint every module**

Run:
```bash
for m in go-common go-auth go-middleware go-framework; do
  golangci-lint run --timeout=5m ./$m/... || exit 1
done
```
Expected: exits 0 for all four modules.

- [ ] **Step 4: Test everything**

Run: `go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1`
Expected: PASS, no regressions outside the touched `go-framework/hertz/sse` package.

- [ ] **Step 5: Example module (separate go.work member, not covered by Step 1-4)**

Run: `cd example && go build ./... && go vet ./... && go test ./... -count=1`
Expected: exits 0 / PASS (redundant with Task 10 but run once more here as the final gate before handoff, in case Tasks 8-10 introduced drift after later commits).

- [ ] **Step 6: No commit** — this task is the final green-light checkpoint before delivery (PR creation / merge), handled by the workflow's Phase 3 Step 3 (delivery choice), not by this plan.
