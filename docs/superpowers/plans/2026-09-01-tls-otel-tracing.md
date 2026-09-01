# go-middleware/tls OTel Tracing 接入 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给 `go-middleware/tls` 的 `Producer` 接入 `WithTrace()` OpenTelemetry 追踪，与 `db`/`es`/`clickhouse`/`kafka` 四个包风格一致。

**Architecture:** 只给 `flush`（真实网络往返）起 span；`SendLog` 记录调用方 span 上下文到 pending 列表，由触发的 `flush` 通过 `trace.LinkFromContext`（`trace.WithLinks`）关联，覆盖 `flushLoop`/`FileShipper` 场景下无业务 trace 上下文时自然退化为无 Link。不做 metrics。

**Tech Stack:** `go.opentelemetry.io/otel`（已在 `go-middleware/go.mod` 中，无需新增依赖）、`go.opentelemetry.io/otel/trace`、`go.opentelemetry.io/otel/sdk/trace/tracetest`（测试）

**Spec:** `docs/superpowers/specs/2026-09-01-tls-otel-tracing-design.md`

## Global Constraints

- 不改变未启用 `WithTrace()` 时的既有行为（`p.tracer == nil` 时零开销、零行为变化）
- `NewProducer` 签名向后兼容：新增 `opts ...ProducerOption` 可变参数，不破坏现有调用方
- 遵循 `.claude/rules/go.md` 静态分析规则：导出符号 godoc 注释、错误处理不忽略、`gofmt` 干净
- 不接入 metrics（仅 tracing，与 es/clickhouse/kafka 保持一致）

---

### Task 1: `trace.go` — instrumentationName + endSpan + spanLinks

**Files:**
- Create: `go-middleware/tls/trace.go`
- Test: `go-middleware/tls/trace_test.go`（本任务只写 `spanLinks` 的单元测试；`flush`/`SendLog` 相关测试见 Task 3）

**Interfaces:**
- Produces: `const instrumentationName string`；`func endSpan(span trace.Span, err error)`；`func spanLinks(scs []trace.SpanContext) []trace.Link`

- [ ] **Step 1: 写失败测试** — 在 `go-middleware/tls/trace_test.go` 新建文件：

```go
package tls

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func TestSpanLinks_Deduplicates(t *testing.T) {
	sc1 := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1},
		SpanID:     trace.SpanID{1},
		TraceFlags: trace.FlagsSampled,
	})
	sc2 := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{2},
		SpanID:     trace.SpanID{2},
		TraceFlags: trace.FlagsSampled,
	})

	links := spanLinks([]trace.SpanContext{sc1, sc1, sc2})

	require.Len(t, links, 2)
	require.Equal(t, sc1, links[0].SpanContext)
	require.Equal(t, sc2, links[1].SpanContext)
}

func TestSpanLinks_EmptyInput(t *testing.T) {
	links := spanLinks(nil)
	require.Empty(t, links)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-middleware/tls/... -run TestSpanLinks -v`
Expected: FAIL，编译错误 `undefined: spanLinks`（`trace.go` 尚未创建）

- [ ] **Step 3: 写最小实现** — 新建 `go-middleware/tls/trace.go`：

```go
package tls

import (
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// instrumentationName OTel instrumentation 标识。
const instrumentationName = "github.com/byx-darwin/go-tools/go-middleware/tls"

// endSpan 出错时记录错误并设置 span 状态，最终结束 span。
func endSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

// spanLinks 把待关联的 SpanContext 按 SpanID 去重后转换为 trace.Link，
// 用于关联触发本次 flush 的各 SendLog 调用方 span。
func spanLinks(scs []trace.SpanContext) []trace.Link {
	if len(scs) == 0 {
		return nil
	}
	seen := make(map[trace.SpanID]struct{}, len(scs))
	links := make([]trace.Link, 0, len(scs))
	for _, sc := range scs {
		if _, ok := seen[sc.SpanID()]; ok {
			continue
		}
		seen[sc.SpanID()] = struct{}{}
		links = append(links, trace.Link{SpanContext: sc})
	}
	return links
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-middleware/tls/... -run TestSpanLinks -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd go-middleware && go build ./tls/... && gofmt -l tls/trace.go tls/trace_test.go
git add go-middleware/tls/trace.go go-middleware/tls/trace_test.go
git commit -m "feat(go-middleware/tls): 新增 OTel trace helper（instrumentationName/endSpan/spanLinks）"
```

---

### Task 2: `options.go` — ProducerOption + WithTrace()

**Files:**
- Create: `go-middleware/tls/options.go`

**Interfaces:**
- Consumes: `instrumentationName`（Task 1）
- Produces: `type ProducerOption func(*Producer)`；`func WithTrace() ProducerOption`
- 依赖 Task 3 给 `Producer` 增加的 `tracer trace.Tracer` 字段——本任务先写 `options.go`，Task 3 里 `producer.go` 补齐该字段后两者才能一起编译通过，因此本任务不单独跑 `go build`，改为编译整包留到 Task 3 完成后验证（见 Task 3 Step 4）

- [ ] **Step 1: 直接写实现**（该文件是纯声明性代码，无独立可测行为，与 Task 3 的字段联动后才可编译验证，故不单列 RED 步骤）

新建 `go-middleware/tls/options.go`：

```go
package tls

import "go.opentelemetry.io/otel"

// ProducerOption 定义 NewProducer 的配置选项函数。
type ProducerOption func(*Producer)

// WithTrace 启用 OpenTelemetry 追踪：为每次 flush（批量发送）起一个 span，
// 并把触发本次 flush 的各 SendLog 调用方 span（若携带有效 trace 上下文）
// 通过 Link 关联到该 span。未启用时 Producer 行为与现状完全一致。
func WithTrace() ProducerOption {
	return func(p *Producer) {
		p.tracer = otel.Tracer(instrumentationName)
	}
}
```

- [ ] **Step 2: Commit**（与 Task 3 一起验证编译后提交，见 Task 3 Step 结尾；本步骤仅记录文件已写，暂不单独 commit，随 Task 3 一并提交）

---

### Task 3: `producer.go` — 接入 tracer 字段、pendingCtx、flush 起 span

**Files:**
- Modify: `go-middleware/tls/producer.go`
- Test: `go-middleware/tls/trace_test.go`（追加 flush/SendLog 相关测试）

**Interfaces:**
- Consumes: `instrumentationName`、`endSpan`、`spanLinks`（Task 1）；`ProducerOption`、`WithTrace()`（Task 2）
- Produces: `Producer.tracer trace.Tracer`（未导出字段，测试内可直接赋值覆盖）；`NewProducer(cfg ProducerConfig, opts ...ProducerOption) (*Producer, error)`（签名变更，追加可变参数）

- [ ] **Step 1: 写失败测试** — 在 `go-middleware/tls/trace_test.go` 追加（沿用 Step 1 已建立的 import 块，新增 `context`/`time`/`testify/assert`/`testify/require`/`tracetest`/`sdktrace`/`codes` 等）：

```go
package tls

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func attrsToMap(attrs []attribute.KeyValue) map[string]any {
	m := make(map[string]any, len(attrs))
	for _, a := range attrs {
		m[string(a.Key)] = a.Value.AsInterface()
	}
	return m
}

func newTestProducer(t *testing.T, opts ...ProducerOption) *Producer {
	t.Helper()
	p, err := NewProducer(ProducerConfig{
		Endpoint:        "tls.example.com", // 不可达域名，flush 一定失败，仅验证 span 行为
		AccessKeyID:     "ak",
		AccessKeySecret: "sk",
		Region:          "cn-beijing",
		TopicID:         "topic-123",
		BatchSize:       1, // 每次 SendLog 立即触发 flush
		FlushInterval:   time.Hour,
	}, opts...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func TestProducer_Flush_TracesSpanWithAttributesOnFailure(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	p := newTestProducer(t, WithTrace())
	p.tracer = tp.Tracer(instrumentationName)

	err := p.SendLog(context.Background(), map[string]string{"level": "info"})
	require.Error(t, err) // fake endpoint: 网络必然失败

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "tls.flush", spans[0].Name)
	assert.Equal(t, codes.Error, spans[0].Status.Code)

	attrs := attrsToMap(spans[0].Attributes)
	assert.Equal(t, "topic-123", attrs["tls.topic_id"])
	assert.EqualValues(t, 1, attrs["tls.batch_size"])
}

func TestProducer_Flush_NoTraceProducesNoSpans(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	p := newTestProducer(t) // 未启用 WithTrace，p.tracer 为 nil

	_ = p.SendLog(context.Background(), map[string]string{"level": "info"})

	assert.Empty(t, exporter.GetSpans(), "tracing 未启用时不应产生任何 span")
}

func TestProducer_SendLog_LinksCallerSpanToFlush(t *testing.T) {
	callerExporter := tracetest.NewInMemoryExporter()
	callerTP := sdktrace.NewTracerProvider(sdktrace.WithSyncer(callerExporter))
	t.Cleanup(func() { _ = callerTP.Shutdown(context.Background()) })
	callerCtx, callerSpan := callerTP.Tracer("caller").Start(context.Background(), "handle-request")

	flushExporter := tracetest.NewInMemoryExporter()
	flushTP := sdktrace.NewTracerProvider(sdktrace.WithSyncer(flushExporter))
	t.Cleanup(func() { _ = flushTP.Shutdown(context.Background()) })

	p := newTestProducer(t, WithTrace())
	p.tracer = flushTP.Tracer(instrumentationName)

	_ = p.SendLog(callerCtx, map[string]string{"level": "info"})
	callerSpan.End()

	spans := flushExporter.GetSpans()
	require.Len(t, spans, 1)
	require.Len(t, spans[0].Links, 1)
	assert.Equal(t, callerSpan.SpanContext().TraceID(), spans[0].Links[0].SpanContext.TraceID())
	assert.Equal(t, callerSpan.SpanContext().SpanID(), spans[0].Links[0].SpanContext.SpanID())
}

func TestProducer_SendLog_NoSpanContextProducesNoLinks(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	p := newTestProducer(t, WithTrace())
	p.tracer = tp.Tracer(instrumentationName)

	// 模拟 flushLoop / FileShipper 场景：ctx 不携带业务 span。
	_ = p.SendLog(context.Background(), map[string]string{"level": "info"})

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Empty(t, spans[0].Links)
}
```

注意：`attrsToMap` 用到 `attribute.KeyValue`，需在 import 块加入 `"go.opentelemetry.io/otel/attribute"`（与 Task 1 的 `trace.go` import 分开，此处属于 `_test.go`）。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-middleware/tls/... -run 'TestProducer_Flush|TestProducer_SendLog' -v`
Expected: FAIL，编译错误（`p.tracer` 未定义字段、`NewProducer` 不接受第二个参数）

- [ ] **Step 3: 写最小实现** — 修改 `go-middleware/tls/producer.go`：

在 import 块追加：

```go
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
```

（完整 import 块变为：`context`、`sync`、`time`、`github.com/samber/oops`、`github.com/volcengine/volc-sdk-golang/service/tls`、`go.opentelemetry.io/otel/attribute`、`go.opentelemetry.io/otel/trace`，按 goimports 标准库/第三方分组排序）

`Producer` 结构体追加两个字段（紧跟 `closeErr` 之后）：

```go
	// tracer 非 nil 时为 flush 记录 span；nil 表示未启用 WithTrace，
	// SendLog/flush 行为与未接入追踪前完全一致。
	tracer trace.Tracer
	// pendingCtx 记录自上次 flush 以来、携带有效 trace 上下文的 SendLog
	// 调用方 SpanContext，flush 时消费并清空，用于起 span 时建立 Link。
	pendingCtx []trace.SpanContext
```

`NewProducer` 签名与末尾追加 opts 应用：

```go
func NewProducer(cfg ProducerConfig, opts ...ProducerOption) (*Producer, error) {
```

（函数体前半部分校验/默认值逻辑不变，仅在 `go p.flushLoop()` 之前插入）：

```go
	client := tls.NewClient(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret, "", cfg.Region)
	p := &Producer{
		client:  client,
		config:  cfg,
		buf:     make([]tls.Log, 0, cfg.BatchSize),
		closeCh: make(chan struct{}),
		done:    make(chan struct{}),
	}
	for _, opt := range opts {
		opt(p)
	}

	go p.flushLoop()
	return p, nil
}
```

`SendLog` 在追加 `buf` 的同一把锁内记录 pending span context：

```go
func (p *Producer) SendLog(ctx context.Context, fields map[string]string) error {
	log := tls.Log{}
	for k, v := range fields {
		log.Contents = append(log.Contents, tls.LogContent{Key: k, Value: v})
	}

	p.mu.Lock()
	p.buf = append(p.buf, log)
	if p.tracer != nil {
		if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
			p.pendingCtx = append(p.pendingCtx, sc)
		}
	}
	needFlush := len(p.buf) >= p.config.BatchSize
	p.mu.Unlock()

	if needFlush {
		return p.flush(ctx)
	}
	return nil
}
```

`flush` 改为命名返回值 `err`，消费 ctx 参数（不再用 `_` 忽略），起 span：

```go
func (p *Producer) flush(ctx context.Context) (err error) {
	p.mu.Lock()
	if len(p.buf) == 0 {
		p.mu.Unlock()
		return nil
	}
	logs := p.buf
	links := p.pendingCtx
	p.buf = make([]tls.Log, 0, p.config.BatchSize)
	p.pendingCtx = nil
	p.mu.Unlock()

	if p.tracer != nil {
		var span trace.Span
		_, span = p.tracer.Start(ctx, "tls.flush",
			trace.WithLinks(spanLinks(links)...),
			trace.WithAttributes(
				attribute.String("tls.topic_id", p.config.TopicID),
				attribute.Int("tls.batch_size", len(logs)),
			),
		)
		defer func() { endSpan(span, err) }()
	}

	_, err = p.client.PutLogsV2(&tls.PutLogsV2Request{
		TopicID:      p.config.TopicID,
		CompressType: "lz4",
		Source:       p.config.Source,
		Logs:         logs,
	})
	if err != nil {
		err = oops.With("tls.flush").
			Code(CodeSend).
			Wrap(err)
	}
	return err
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-middleware/tls/... -v`
Expected: PASS（含 Task 1/2/3 全部新增测试，以及原有 `producer_test.go`/`errors_test.go`/`shipper_test.go` 全部通过——尤其确认 `TestProducer_Close_FlushesRemainingBuffer` 等既有用例未被 `flush` 签名/行为改动破坏）

- [ ] **Step 5: 全量校验并提交**

```bash
cd /Users/xs/Documents/workspce/github.com/byx-darwin/go-tools
gofmt -l go-middleware/tls/*.go
go build ./go-middleware/...
go vet ./go-middleware/...
golangci-lint run --timeout=5m ./go-middleware/...
go test ./go-middleware/... -count=1
git add go-middleware/tls/producer.go go-middleware/tls/options.go go-middleware/tls/trace_test.go
git commit -m "feat(go-middleware/tls): Producer 接入 WithTrace() OTel 追踪 (#54)"
```

---

### Task 4: `README.md` 同步更新

**Files:**
- Modify: `go-middleware/README.md:22`

**Interfaces:**
- Consumes: 无代码接口，纯文档
- Produces: 无

- [ ] **Step 1: 修改包一览表格 tls 行**

将 `go-middleware/README.md` 第 22 行：

```
| `tls` | 火山引擎日志服务（Producer + FileShipper；含包内错误码 20501-20504） |
```

改为：

```
| `tls` | 火山引擎日志服务（Producer + FileShipper；含包内错误码 20501-20504；OTel 追踪，可选） |
```

（与 `redis`/`kafka`/`db`/`es`/`clickhouse` 各行"OTel 追踪，可选"的措辞保持一致）

- [ ] **Step 2: 校验渲染无误**

Run: `git diff go-middleware/README.md`
Expected: 只有第 22 行的单行文本变化，无其他改动

- [ ] **Step 3: Commit**

```bash
git add go-middleware/README.md
git commit -m "docs(go-middleware): tls 包一览标注 OTel 追踪支持 (#54)"
```

---

## Self-Review Notes

- **Spec 覆盖**：设计文档的 4 项决策（span 粒度、Link 关联、FileShipper 不区分对待、不做 metrics）分别对应 Task 3 的 flush 实现、SendLog pending 记录、无特殊分支处理（天然满足，未新增代码）、以及全篇未出现 metrics 相关代码——均已覆盖。Issue #54 四项 Acceptance Criteria（WithTrace Option、测试、README）对应 Task 2/3/4。
- **占位符扫描**：全部 Step 含可直接使用的完整代码块，无 TBD/TODO。
- **类型一致性**：`ProducerOption func(*Producer)`（Task 2）与 `NewProducer(cfg ProducerConfig, opts ...ProducerOption)`（Task 3）、`Producer.tracer trace.Tracer` / `Producer.pendingCtx []trace.SpanContext`（Task 3）与 `spanLinks(scs []trace.SpanContext) []trace.Link`（Task 1）、`endSpan(span trace.Span, err error)`（Task 1）与 Task 3 内 `defer func() { endSpan(span, err) }()` 调用点，签名全部对齐。
