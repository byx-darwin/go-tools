# Kafka DLQ 死信队列与 Offset 管理封装 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `go-middleware/kafka` 增加 DLQ 死信队列转发能力和 offset/lag 查询能力，减少业务方重复实现这部分基础设施代码。

**Architecture:** 新增 3 个职责单一的文件（`failure_counter.go`、`dlq.go`、`offset.go`），复用现有 `Writer`/`Consumer`/`ClientOption` 结构，不改变任何现有导出方法签名。DLQ 转发通过显式方法（`SendToDLQ`）+ 可选自动化封装（`HandleMessage`）两层提供；lag 查询严格限定在"当前 Consumer 自身进度"范围内，不引入 admin client。

**Tech Stack:** Go 1.25, `github.com/segmentio/kafka-go`, `github.com/byx-darwin/go-tools/go-common/error` (oops 封装), `testify`

**Spec:** `docs/superpowers/specs/2026-08-31-kafka-dlq-offset-design.md`

## Global Constraints

- 新增导出符号必须有以符号名开头的 godoc 注释（`.claude/rules/go.md` §8.3）
- 错误处理：`defer func() { _ = x.Close() }()` 显式忽略；`nolint` 必须带原因（§8.4）
- 使用 `any` 而非 `interface{}`（§8.5）
- 八进制字面量用 `0o` 写法；参数用 `//nolint:xxx // 原因` 格式
- 错误码范围 20201-20203 已被现有 kafka 错误占用，本次新增 20204-20206
- 测试不依赖真实 Kafka broker（沿用 `kafka_test.go` 现有策略）；网络相关方法（`SendToDLQ`/`PartitionOffsets`/`Lag`/`Seek`）仅测试参数校验与错误包装路径，纯逻辑（header 构造、失败计数、`HandleMessage` 决策流程）需要完整覆盖
- Functional Options 规范：新增 `WithXxx` 函数遵循 `.claude/rules/options-pattern.md`，防御无效输入，不覆盖已有值

## 当前代码状态（重要）

`go-middleware/kafka/` 目录中 `errors.go`、`options.go`、`trace.go`（及对应 `_test.go`）是**已存在于工作区但尚未提交**的文件（来自 kafka.go 拆分重构，不属于本次任务范围）。本计划的任务在这些**已存在的文件基础上追加内容**，不会重新创建它们。当前内容：

`errors.go`：定义 `CodeWrite=20201`/`CodeRead=20202`/`CodeCommit=20203` 及 `ErrWrite`/`ErrRead`/`ErrCommit`，`init()` 注册 HTTP 500。

`options.go`：
```go
package kafka

// ClientOption 定义 Kafka Writer/Consumer 创建选项。
type ClientOption func(*clientOptions)

type clientOptions struct {
	trace bool
}

// WithTrace 启用 OpenTelemetry 消息追踪。
func WithTrace() ClientOption {
	return func(o *clientOptions) {
		o.trace = true
	}
}
```

`consumer.go` 的 `Consumer` 结构体：`type Consumer struct { r *kafka.Reader; tracer trace.Tracer }`，`NewConsumer(cfg ReaderConfig, opts ...ClientOption) *Consumer`。

`producer.go` 的 `Writer` 结构体：`type Writer struct { w *kafka.Writer; tracer trace.Tracer }`，`func (w *Writer) WriteMessages(ctx context.Context, msgs ...kafka.Message) error`。

---

### Task 1: 错误码扩展

**Files:**
- Modify: `go-middleware/kafka/errors.go`
- Modify: `go-middleware/kafka/errors_test.go`

**Interfaces:**
- Produces: `kafka.CodeDLQForward = 20204`, `kafka.CodeOffsetQuery = 20205`, `kafka.CodeSeek = 20206`, `kafka.ErrDLQForward`, `kafka.ErrOffsetQuery`, `kafka.ErrSeek`（均为 `goerror.Code(...).Public(...)` 构造器，供后续任务 `.Wrap(err)`）

- [ ] **Step 1: 编写失败的错误码断言测试**

在 `go-middleware/kafka/errors_test.go` 的 `TestCodeValues` 函数体末尾追加：

```go
	assert.Equal(t, 20204, kafka.CodeDLQForward)
	assert.Equal(t, 20205, kafka.CodeOffsetQuery)
	assert.Equal(t, 20206, kafka.CodeSeek)
```

在 `TestPredefinedErrors` 函数体末尾追加：

```go
	code, public = goerror.Extract(kafka.ErrDLQForward.Wrap(errors.New("x")))
	assert.Equal(t, 20204, code)
	assert.Equal(t, "kafka_dlq_forward_error", public)

	code, public = goerror.Extract(kafka.ErrOffsetQuery.Wrap(errors.New("x")))
	assert.Equal(t, 20205, code)
	assert.Equal(t, "kafka_offset_query_error", public)

	code, public = goerror.Extract(kafka.ErrSeek.Wrap(errors.New("x")))
	assert.Equal(t, 20206, code)
	assert.Equal(t, "kafka_seek_error", public)
```

在 `TestHTTPStatusRegistration` 函数体末尾追加：

```go
	assert.Equal(t, 500, goerror.HTTPStatus(kafka.ErrDLQForward.Wrap(errors.New("x"))))
	assert.Equal(t, 500, goerror.HTTPStatus(kafka.ErrOffsetQuery.Wrap(errors.New("x"))))
	assert.Equal(t, 500, goerror.HTTPStatus(kafka.ErrSeek.Wrap(errors.New("x"))))
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-middleware/kafka/... -run TestCodeValues -v`
Expected: FAIL — `undefined: kafka.CodeDLQForward`

- [ ] **Step 3: 在 errors.go 追加错误码定义**

将 `go-middleware/kafka/errors.go` 顶部注释从 `// Kafka 错误码 20201-20203。` 改为 `// Kafka 错误码 20201-20206。`，并在现有 `const` 块内追加：

```go
const (
	// CodeWrite Kafka 消息写入失败
	CodeWrite = 20201
	// CodeRead Kafka 消息读取失败
	CodeRead = 20202
	// CodeCommit Kafka offset 提交失败
	CodeCommit = 20203
	// CodeDLQForward DLQ 转发失败
	CodeDLQForward = 20204
	// CodeOffsetQuery offset/lag 查询失败
	CodeOffsetQuery = 20205
	// CodeSeek Seek 失败
	CodeSeek = 20206
)
```

在 `var` 块内追加：

```go
	// ErrDLQForward DLQ 转发失败
	ErrDLQForward = goerror.Code(CodeDLQForward).Public("kafka_dlq_forward_error")
	// ErrOffsetQuery offset/lag 查询失败
	ErrOffsetQuery = goerror.Code(CodeOffsetQuery).Public("kafka_offset_query_error")
	// ErrSeek Seek 失败
	ErrSeek = goerror.Code(CodeSeek).Public("kafka_seek_error")
```

在 `init()` 的 `RegisterHTTPStatuses` map 内追加：

```go
		CodeDLQForward:  500,
		CodeOffsetQuery: 500,
		CodeSeek:        500,
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-middleware/kafka/... -run "TestCodeValues|TestPredefinedErrors|TestHTTPStatusRegistration" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-middleware/kafka/errors.go go-middleware/kafka/errors_test.go
git commit -m "feat(kafka): add DLQ/offset error codes 20204-20206"
```

---

### Task 2: FailureCounter 接口与内存默认实现

**Files:**
- Create: `go-middleware/kafka/failure_counter.go`
- Test: `go-middleware/kafka/failure_counter_test.go`

**Interfaces:**
- Produces: `type FailureCounter interface { Incr(key string) int; Reset(key string) }`，`func newMemFailureCounter() *memFailureCounter`（`*memFailureCounter` 实现 `FailureCounter`）

- [ ] **Step 1: 编写失败的测试**

创建 `go-middleware/kafka/failure_counter_test.go`：

```go
package kafka

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemFailureCounter_IncrReturnsRunningCount(t *testing.T) {
	c := newMemFailureCounter()
	assert.Equal(t, 1, c.Incr("k1"))
	assert.Equal(t, 2, c.Incr("k1"))
	assert.Equal(t, 3, c.Incr("k1"))
}

func TestMemFailureCounter_KeysAreIsolated(t *testing.T) {
	c := newMemFailureCounter()
	assert.Equal(t, 1, c.Incr("k1"))
	assert.Equal(t, 1, c.Incr("k2"))
	assert.Equal(t, 2, c.Incr("k1"))
}

func TestMemFailureCounter_ResetClearsCount(t *testing.T) {
	c := newMemFailureCounter()
	c.Incr("k1")
	c.Incr("k1")
	c.Reset("k1")
	assert.Equal(t, 1, c.Incr("k1"))
}

func TestMemFailureCounter_ResetUnknownKeyNoop(t *testing.T) {
	c := newMemFailureCounter()
	c.Reset("unknown")
	assert.Equal(t, 1, c.Incr("unknown"))
}

func TestMemFailureCounter_ConcurrentIncrSameKey(t *testing.T) {
	c := newMemFailureCounter()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Incr("k1")
		}()
	}
	wg.Wait()
	assert.Equal(t, 101, c.Incr("k1"))
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-middleware/kafka/... -run TestMemFailureCounter -v`
Expected: FAIL — `undefined: newMemFailureCounter`

- [ ] **Step 3: 实现 failure_counter.go**

```go
package kafka

import "sync/atomic"

// FailureCounter 记录并查询按 key 维度的失败次数，供 Consumer.HandleMessage
// 判断是否达到 DLQ 转发阈值。默认实现为进程内存级（memFailureCounter），可通过
// WithFailureCounter 替换为跨实例存储实现（如 Redis，本包不提供）。
type FailureCounter interface {
	// Incr 递增 key 的失败计数并返回递增后的值。
	Incr(key string) int
	// Reset 清零 key 的失败计数（成功处理或已转发 DLQ 后调用）。
	Reset(key string)
}

// memFailureCounter 是 FailureCounter 的进程内存实现，跟随 Consumer 生命周期，
// 不做跨实例/跨进程累计。
type memFailureCounter struct {
	counts sync.Map // map[string]*int64
}

// newMemFailureCounter 创建内存失败计数器。
func newMemFailureCounter() *memFailureCounter {
	return &memFailureCounter{}
}

// Incr 递增 key 的失败计数并返回递增后的值。
func (c *memFailureCounter) Incr(key string) int {
	v, _ := c.counts.LoadOrStore(key, new(int64))
	n := atomic.AddInt64(v.(*int64), 1)
	return int(n)
}

// Reset 清零 key 的失败计数。
func (c *memFailureCounter) Reset(key string) {
	c.counts.Delete(key)
}
```

注意顶部 import 需要同时引入 `"sync"` 与 `"sync/atomic"`：

```go
import (
	"sync"
	"sync/atomic"
)
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-middleware/kafka/... -run TestMemFailureCounter -v -race`
Expected: PASS（`-race` 验证并发安全）

- [ ] **Step 5: Commit**

```bash
git add go-middleware/kafka/failure_counter.go go-middleware/kafka/failure_counter_test.go
git commit -m "feat(kafka): add FailureCounter interface with in-memory default"
```

---

### Task 3: DLQSender 接口与 Writer.SendToDLQ

**Files:**
- Create: `go-middleware/kafka/dlq.go`
- Test: `go-middleware/kafka/dlq_test.go`

**Interfaces:**
- Consumes: `Writer.WriteMessages(ctx, msgs...)`（`producer.go` 已有，供 `SendToDLQ` 内部复用）；`kafka.ErrDLQForward`（Task 1）
- Produces: `type DLQSender interface { SendToDLQ(ctx context.Context, dlqTopic string, msg kafka.Message, reason string) error }`；`func (w *Writer) SendToDLQ(ctx context.Context, dlqTopic string, msg kafka.Message, reason string) error`；纯函数 `func buildDLQHeaders(msg kafka.Message, reason string) []kafka.Header`（供 Task 5 与本任务测试复用，同包可见）

- [ ] **Step 1: 编写失败的 header 构造测试**

创建 `go-middleware/kafka/dlq_test.go`：

```go
package kafka

import (
	"strconv"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
)

func headerValue(headers []kafka.Header, key string) (string, bool) {
	for _, h := range headers {
		if h.Key == key {
			return string(h.Value), true
		}
	}
	return "", false
}

func TestBuildDLQHeaders_AppendsMetadata(t *testing.T) {
	msg := kafka.Message{
		Topic:     "orders",
		Partition: 2,
		Offset:    42,
		Headers:   []kafka.Header{{Key: "trace-id", Value: []byte("abc")}},
	}

	headers := buildDLQHeaders(msg, "handler panicked")

	v, ok := headerValue(headers, "x-dlq-reason")
	assert.True(t, ok)
	assert.Equal(t, "handler panicked", v)

	v, ok = headerValue(headers, "x-dlq-original-topic")
	assert.True(t, ok)
	assert.Equal(t, "orders", v)

	v, ok = headerValue(headers, "x-dlq-original-partition")
	assert.True(t, ok)
	assert.Equal(t, "2", v)

	v, ok = headerValue(headers, "x-dlq-original-offset")
	assert.True(t, ok)
	assert.Equal(t, "42", v)

	// 原有 Header 保留
	v, ok = headerValue(headers, "trace-id")
	assert.True(t, ok)
	assert.Equal(t, "abc", v)
}

func TestBuildDLQHeaders_DoesNotMutateOriginalMessage(t *testing.T) {
	original := []kafka.Header{{Key: "trace-id", Value: []byte("abc")}}
	msg := kafka.Message{Topic: "orders", Headers: original}

	buildDLQHeaders(msg, "boom")

	assert.Len(t, original, 1, "原始 Headers 切片不应被追加操作污染")
}

func TestPartitionOffsetToString(t *testing.T) {
	assert.Equal(t, "2", strconv.Itoa(2))
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-middleware/kafka/... -run TestBuildDLQHeaders -v`
Expected: FAIL — `undefined: buildDLQHeaders`

- [ ] **Step 3: 实现 buildDLQHeaders 与 SendToDLQ**

创建 `go-middleware/kafka/dlq.go`：

```go
package kafka

import (
	"context"
	"strconv"

	"github.com/segmentio/kafka-go"
)

// DLQSender 定义 DLQ 转发能力，*Writer 天然实现该接口。抽出接口是为了让
// Consumer.HandleMessage（见 Task 5）在单元测试中可以注入 fake 实现，
// 不需要连接真实 Kafka broker。
type DLQSender interface {
	SendToDLQ(ctx context.Context, dlqTopic string, msg kafka.Message, reason string) error
}

// buildDLQHeaders 在原始消息 Headers 基础上追加 DLQ 元信息，不修改入参 msg。
func buildDLQHeaders(msg kafka.Message, reason string) []kafka.Header {
	headers := make([]kafka.Header, 0, len(msg.Headers)+4)
	headers = append(headers, msg.Headers...)
	headers = append(headers,
		kafka.Header{Key: "x-dlq-reason", Value: []byte(reason)},
		kafka.Header{Key: "x-dlq-original-topic", Value: []byte(msg.Topic)},
		kafka.Header{Key: "x-dlq-original-partition", Value: []byte(strconv.Itoa(msg.Partition))},
		kafka.Header{Key: "x-dlq-original-offset", Value: []byte(strconv.FormatInt(msg.Offset, 10))},
	)
	return headers
}

// SendToDLQ 将消息转发到死信 topic dlqTopic，并附加失败原因（x-dlq-reason）
// 与原始消息元信息（x-dlq-original-topic/partition/offset）作为 Header。
// dlqTopic 由调用方显式指定，Writer 实例不绑定固定的 DLQ topic。
func (w *Writer) SendToDLQ(ctx context.Context, dlqTopic string, msg kafka.Message, reason string) error {
	dlqMsg := kafka.Message{
		Topic:   dlqTopic,
		Key:     msg.Key,
		Value:   msg.Value,
		Headers: buildDLQHeaders(msg, reason),
	}
	if err := w.WriteMessages(ctx, dlqMsg); err != nil {
		return ErrDLQForward.Wrap(err)
	}
	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-middleware/kafka/... -run "TestBuildDLQHeaders|TestPartitionOffsetToString" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-middleware/kafka/dlq.go go-middleware/kafka/dlq_test.go
git commit -m "feat(kafka): add Writer.SendToDLQ and DLQSender interface"
```

---

### Task 4: Consumer DLQ Options（WithDLQ / WithFailureCounter）

**Files:**
- Modify: `go-middleware/kafka/options.go`
- Modify: `go-middleware/kafka/options_test.go`

**Interfaces:**
- Consumes: `DLQSender`（Task 3）、`FailureCounter`（Task 2）
- Produces: `clientOptions` 新增字段 `dlqSender DLQSender`、`dlqTopic string`、`dlqMaxAttempts int`、`failureCounter FailureCounter`；`func WithDLQ(sender DLQSender, dlqTopic string, maxAttempts int) ClientOption`；`func WithFailureCounter(counter FailureCounter) ClientOption`

- [ ] **Step 1: 编写失败的测试**

在 `go-middleware/kafka/options_test.go` 追加：

```go
func TestClientOption_WithDLQ(t *testing.T) {
	o := &clientOptions{}
	sender := &Writer{}
	WithDLQ(sender, "orders-dlq", 3)(o)
	assert.Equal(t, sender, o.dlqSender)
	assert.Equal(t, "orders-dlq", o.dlqTopic)
	assert.Equal(t, 3, o.dlqMaxAttempts)
}

func TestClientOption_WithDLQ_IgnoresNonPositiveMaxAttempts(t *testing.T) {
	o := &clientOptions{}
	WithDLQ(&Writer{}, "orders-dlq", 0)(o)
	assert.Nil(t, o.dlqSender)
	assert.Equal(t, 0, o.dlqMaxAttempts)
}

func TestClientOption_WithFailureCounter(t *testing.T) {
	o := &clientOptions{}
	counter := newMemFailureCounter()
	WithFailureCounter(counter)(o)
	assert.Equal(t, counter, o.failureCounter)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-middleware/kafka/... -run "TestClientOption_WithDLQ|TestClientOption_WithFailureCounter" -v`
Expected: FAIL — `undefined: WithDLQ`

- [ ] **Step 3: 扩展 options.go**

将 `go-middleware/kafka/options.go` 整体替换为：

```go
package kafka

// ClientOption 定义 Kafka Writer/Consumer 创建选项。
type ClientOption func(*clientOptions)

type clientOptions struct {
	trace bool

	dlqSender      DLQSender
	dlqTopic       string
	dlqMaxAttempts int
	failureCounter FailureCounter
}

// WithTrace 启用 OpenTelemetry 消息追踪。
func WithTrace() ClientOption {
	return func(o *clientOptions) {
		o.trace = true
	}
}

// WithDLQ 为 Consumer 启用失败自动转发 DLQ：sender 为 DLQ 转发目标（通常是
// 另一个 *Writer），dlqTopic 为死信 topic，maxAttempts 为转发前允许的最大失败
// 次数。maxAttempts <= 0 时忽略本 Option，不启用自动转发。仅 NewConsumer 消费
// 该 Option，NewWriter 忽略。
func WithDLQ(sender DLQSender, dlqTopic string, maxAttempts int) ClientOption {
	return func(o *clientOptions) {
		if maxAttempts > 0 {
			o.dlqSender = sender
			o.dlqTopic = dlqTopic
			o.dlqMaxAttempts = maxAttempts
		}
	}
}

// WithFailureCounter 替换 Consumer 默认的内存失败计数器实现。仅 NewConsumer
// 消费该 Option，NewWriter 忽略。
func WithFailureCounter(counter FailureCounter) ClientOption {
	return func(o *clientOptions) {
		if counter != nil {
			o.failureCounter = counter
		}
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-middleware/kafka/... -run "TestClientOption" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go-middleware/kafka/options.go go-middleware/kafka/options_test.go
git commit -m "feat(kafka): add WithDLQ and WithFailureCounter client options"
```

---

### Task 5: Consumer.HandleMessage 自动化封装

**Files:**
- Modify: `go-middleware/kafka/consumer.go`
- Modify: `go-middleware/kafka/dlq.go`
- Modify: `go-middleware/kafka/dlq_test.go`

**Interfaces:**
- Consumes: `clientOptions.{dlqSender,dlqTopic,dlqMaxAttempts,failureCounter}`（Task 4）、`FailureCounter`/`newMemFailureCounter()`（Task 2）、`DLQSender.SendToDLQ`（Task 3）
- Produces: `Consumer` 结构体新增字段 `dlqSender DLQSender`、`dlqTopic string`、`dlqMaxAttempts int`、`failureCounter FailureCounter`；`func (c *Consumer) HandleMessage(ctx context.Context, msg kafka.Message, handler func(context.Context, kafka.Message) error) error`；内部纯函数 `func failureKey(msg kafka.Message) string`

- [ ] **Step 1: 编写失败的测试**

在 `go-middleware/kafka/dlq_test.go` 追加（同包 `kafka`，可直接访问 `Consumer` 内部字段）：

```go
type fakeDLQSender struct {
	calls []struct {
		topic  string
		msg    kafka.Message
		reason string
	}
	err error
}

func (f *fakeDLQSender) SendToDLQ(_ context.Context, dlqTopic string, msg kafka.Message, reason string) error {
	f.calls = append(f.calls, struct {
		topic  string
		msg    kafka.Message
		reason string
	}{dlqTopic, msg, reason})
	return f.err
}

func newTestConsumerWithDLQ(sender DLQSender, dlqTopic string, maxAttempts int) *Consumer {
	return &Consumer{
		dlqSender:      sender,
		dlqTopic:       dlqTopic,
		dlqMaxAttempts: maxAttempts,
		failureCounter: newMemFailureCounter(),
	}
}

func TestFailureKey_UsesMessageKeyWhenPresent(t *testing.T) {
	msg := kafka.Message{Key: []byte("order-1"), Topic: "orders", Partition: 0, Offset: 5}
	assert.Equal(t, "order-1", failureKey(msg))
}

func TestFailureKey_FallsBackToTopicPartitionOffsetWhenKeyEmpty(t *testing.T) {
	msg := kafka.Message{Topic: "orders", Partition: 1, Offset: 9}
	assert.Equal(t, "orders/1/9", failureKey(msg))
}

func TestHandleMessage_SuccessResetsCounterAndReturnsNil(t *testing.T) {
	sender := &fakeDLQSender{}
	c := newTestConsumerWithDLQ(sender, "orders-dlq", 2)
	msg := kafka.Message{Key: []byte("k1")}

	err := c.HandleMessage(context.Background(), msg, func(context.Context, kafka.Message) error {
		return nil
	})

	assert.NoError(t, err)
	assert.Empty(t, sender.calls)
}

func TestHandleMessage_BelowThresholdReturnsOriginalError(t *testing.T) {
	sender := &fakeDLQSender{}
	c := newTestConsumerWithDLQ(sender, "orders-dlq", 3)
	msg := kafka.Message{Key: []byte("k1")}
	wantErr := errors.New("boom")

	err := c.HandleMessage(context.Background(), msg, func(context.Context, kafka.Message) error {
		return wantErr
	})

	assert.ErrorIs(t, err, wantErr)
	assert.Empty(t, sender.calls)
}

func TestHandleMessage_ThresholdReachedForwardsToDLQAndReturnsNil(t *testing.T) {
	sender := &fakeDLQSender{}
	c := newTestConsumerWithDLQ(sender, "orders-dlq", 2)
	msg := kafka.Message{Key: []byte("k1")}
	handlerErr := errors.New("boom")
	handler := func(context.Context, kafka.Message) error { return handlerErr }

	err1 := c.HandleMessage(context.Background(), msg, handler)
	assert.ErrorIs(t, err1, handlerErr)

	err2 := c.HandleMessage(context.Background(), msg, handler)
	assert.NoError(t, err2)

	assert.Len(t, sender.calls, 1)
	assert.Equal(t, "orders-dlq", sender.calls[0].topic)
	assert.Equal(t, "boom", sender.calls[0].reason)
}

func TestHandleMessage_ThresholdReachedResetsCounterAfterForward(t *testing.T) {
	sender := &fakeDLQSender{}
	c := newTestConsumerWithDLQ(sender, "orders-dlq", 1)
	msg := kafka.Message{Key: []byte("k1")}
	handler := func(context.Context, kafka.Message) error { return errors.New("boom") }

	_ = c.HandleMessage(context.Background(), msg, handler)
	assert.Len(t, sender.calls, 1)

	_ = c.HandleMessage(context.Background(), msg, handler)
	assert.Len(t, sender.calls, 1, "刚重置的 key 需要再次达到 maxAttempts 才应再次转发")
}

func TestHandleMessage_DLQForwardFailureWrapsBothErrors(t *testing.T) {
	sender := &fakeDLQSender{err: errors.New("dlq unreachable")}
	c := newTestConsumerWithDLQ(sender, "orders-dlq", 1)
	msg := kafka.Message{Key: []byte("k1")}
	handlerErr := errors.New("boom")

	err := c.HandleMessage(context.Background(), msg, func(context.Context, kafka.Message) error {
		return handlerErr
	})

	assert.Error(t, err)
	assert.ErrorIs(t, err, handlerErr)
}

func TestHandleMessage_NoDLQConfiguredReturnsHandlerErrorDirectly(t *testing.T) {
	c := &Consumer{}
	msg := kafka.Message{Key: []byte("k1")}
	wantErr := errors.New("boom")

	err := c.HandleMessage(context.Background(), msg, func(context.Context, kafka.Message) error {
		return wantErr
	})

	assert.ErrorIs(t, err, wantErr)
}
```

在 `dlq_test.go` 顶部 import 追加 `"context"` 与 `"errors"`：

```go
import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
)
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-middleware/kafka/... -run "TestHandleMessage|TestFailureKey" -v`
Expected: FAIL — `undefined: failureKey` / `c.HandleMessage undefined`

- [ ] **Step 3: 实现 failureKey 与 Consumer.HandleMessage**

在 `go-middleware/kafka/dlq.go` 追加（`strconv` 已 import）：

```go
// failureKey 计算消息的失败计数 key：优先用业务消息键（msg.Key），为空时退化
// 为 topic/partition/offset，避免所有空 key 消息共享一个计数器。
func failureKey(msg kafka.Message) string {
	if len(msg.Key) > 0 {
		return string(msg.Key)
	}
	return msg.Topic + "/" + strconv.Itoa(msg.Partition) + "/" + strconv.FormatInt(msg.Offset, 10)
}

// HandleMessage 执行 handler 处理消息；handler 成功时清零该消息的失败计数并
// 返回 nil。handler 失败且未通过 WithDLQ 配置 DLQ 时，直接返回 handler 的 err。
// 已配置 DLQ 时，按 failureKey 累计失败次数，达到 WithDLQ 配置的 maxAttempts
// 后自动转发 DLQ 并清零计数（视为该消息已终结处理，返回 nil）；未达阈值则
// 原样返回 handler 的 err，交由调用方决定重试或丢弃。
func (c *Consumer) HandleMessage(ctx context.Context, msg kafka.Message, handler func(context.Context, kafka.Message) error) error {
	key := failureKey(msg)
	err := handler(ctx, msg)
	if err == nil {
		if c.failureCounter != nil {
			c.failureCounter.Reset(key)
		}
		return nil
	}
	if c.dlqSender == nil {
		return err
	}

	counter := c.failureCounter
	if counter == nil {
		counter = newMemFailureCounter()
		c.failureCounter = counter
	}
	if counter.Incr(key) < c.dlqMaxAttempts {
		return err
	}

	counter.Reset(key)
	if dlqErr := c.dlqSender.SendToDLQ(ctx, c.dlqTopic, msg, err.Error()); dlqErr != nil {
		return ErrDLQForward.Wrap(dlqErr)
	}
	return nil
}
```

在 `go-middleware/kafka/consumer.go` 的 `Consumer` 结构体追加字段，并在 `NewConsumer` 中从 `clientOptions` 赋值：

```go
// Consumer Kafka 消息消费者。
// 封装 kafka-go Reader，支持消费者组和手动提交 offset。
type Consumer struct {
	r      *kafka.Reader
	tracer trace.Tracer

	dlqSender      DLQSender
	dlqTopic       string
	dlqMaxAttempts int
	failureCounter FailureCounter
}
```

在 `NewConsumer` 中，`c := &Consumer{r: kafka.NewReader(rCfg)}` 之后、`return c` 之前追加：

```go
	c.dlqSender = o.dlqSender
	c.dlqTopic = o.dlqTopic
	c.dlqMaxAttempts = o.dlqMaxAttempts
	c.failureCounter = o.failureCounter
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./go-middleware/kafka/... -v`
Expected: PASS（全部用例，含 Task 1-4 已有测试）

- [ ] **Step 5: Commit**

```bash
git add go-middleware/kafka/consumer.go go-middleware/kafka/dlq.go go-middleware/kafka/dlq_test.go
git commit -m "feat(kafka): add Consumer.HandleMessage automatic DLQ forwarding"
```

---

### Task 6: Offset/Lag 查询与 Seek

**Files:**
- Create: `go-middleware/kafka/offset.go`
- Test: `go-middleware/kafka/offset_test.go`
- Modify: `go-middleware/kafka/consumer.go`

**Interfaces:**
- Consumes: `kafka.ErrOffsetQuery`/`kafka.ErrSeek`（Task 1）、`Consumer.r *kafka.Reader`（已有）
- Produces: `func PartitionOffsets(ctx context.Context, brokers []string, topic string) (map[int]int64, error)`；`func (c *Consumer) Lag(ctx context.Context) (map[int]int64, error)`；`func (c *Consumer) Seek(ctx context.Context, offset int64) error`

- [ ] **Step 1: 编写失败的测试（参数校验 + 错误包装路径）**

创建 `go-middleware/kafka/offset_test.go`：

```go
package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPartitionOffsets_EmptyBrokersReturnsWrappedError(t *testing.T) {
	_, err := PartitionOffsets(context.Background(), nil, "orders")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrOffsetQuery)
}

func TestPartitionOffsets_UnreachableBrokerReturnsWrappedError(t *testing.T) {
	_, err := PartitionOffsets(context.Background(), []string{"127.0.0.1:1"}, "orders")
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrOffsetQuery)
}

func TestConsumer_Lag_UnreachableBrokerReturnsWrappedError(t *testing.T) {
	c := NewConsumer(ReaderConfig{
		Broker: []string{"127.0.0.1:1"},
		Topic:  "orders",
	})
	defer func() { _ = c.Close() }()

	_, err := c.Lag(context.Background())
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrOffsetQuery)
}

func TestConsumer_Seek_GroupModeReturnsWrappedError(t *testing.T) {
	c := NewConsumer(ReaderConfig{
		Broker:  []string{"127.0.0.1:1"},
		Topic:   "orders",
		GroupID: "orders-group",
	})
	defer func() { _ = c.Close() }()

	err := c.Seek(context.Background(), 10)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrSeek)
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./go-middleware/kafka/... -run "TestPartitionOffsets|TestConsumer_Lag|TestConsumer_Seek" -v`
Expected: FAIL — `undefined: PartitionOffsets`

- [ ] **Step 3: 在 Consumer 结构体追加 brokers/topic 字段**

`Lag` 需要 broker 地址和 topic 才能调用 `PartitionOffsets`，而 `kafka.Reader.Stats()` 不暴露 broker 列表，因此 `Consumer` 需要在创建时自行保存这两个字段。

在 `go-middleware/kafka/consumer.go` 的 `Consumer` 结构体追加字段（紧邻 Task 5 已加的 DLQ 相关字段）：

```go
	brokers []string
	topic   string
```

在 `NewConsumer` 中，把：

```go
	c := &Consumer{r: kafka.NewReader(rCfg)}
```

改为：

```go
	c := &Consumer{r: kafka.NewReader(rCfg), brokers: cfg.Broker, topic: cfg.Topic}
```

- [ ] **Step 4: 实现 offset.go**

```go
package kafka

import (
	"context"
	"errors"

	"github.com/segmentio/kafka-go"
)

var errEmptyBrokers = errors.New("kafka: brokers must not be empty")

// PartitionOffsets 通过 kafka.Conn 查询 topic 各分区的 log-end offset
// （分区最新写入位置），不依赖 consumer group 状态，查询完成后立即关闭连接。
// 返回值不包含任何"已消费到"的信息——如需 lag，见 Consumer.Lag。
func PartitionOffsets(ctx context.Context, brokers []string, topic string) (map[int]int64, error) {
	if len(brokers) == 0 {
		return nil, ErrOffsetQuery.Wrap(errEmptyBrokers)
	}

	conn, err := kafka.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return nil, ErrOffsetQuery.Wrap(err)
	}
	defer func() { _ = conn.Close() }()

	partitions, err := conn.ReadPartitions(topic)
	if err != nil {
		return nil, ErrOffsetQuery.Wrap(err)
	}

	offsets := make(map[int]int64, len(partitions))
	for _, p := range partitions {
		pConn, dialErr := kafka.DialLeader(ctx, "tcp", brokers[0], topic, p.ID)
		if dialErr != nil {
			return nil, ErrOffsetQuery.Wrap(dialErr)
		}
		last, offErr := pConn.ReadLastOffset()
		_ = pConn.Close()
		if offErr != nil {
			return nil, ErrOffsetQuery.Wrap(offErr)
		}
		offsets[p.ID] = last
	}
	return offsets, nil
}

// Lag 返回当前 Consumer 自身各分区的消费延迟：PartitionOffsets 得到的
// log-end offset 减去 Reader.Stats() 中已消费的 offset（仅对本 Consumer
// 当前分配到的 partition 生效，其余分区直接返回 log-end offset 本身，因为
// 本 Consumer 尚未消费其中任何数据）。仅反映本 Consumer 实例已消费的进度，
// 不能查询同一 group 内其他 consumer 的 lag。
func (c *Consumer) Lag(ctx context.Context) (map[int]int64, error) {
	logEnd, err := PartitionOffsets(ctx, c.brokers, c.topic)
	if err != nil {
		return nil, err
	}

	stats := c.r.Stats()
	lag := make(map[int]int64, len(logEnd))
	for partition, end := range logEnd {
		if partition == stats.Partition {
			lag[partition] = end - stats.Offset
			continue
		}
		lag[partition] = end
	}
	return lag, nil
}

// Seek 定位到指定 offset 重新消费。要求 Consumer 以非 consumer-group 模式
// （ReaderConfig.GroupID 为空）创建，否则返回 ErrSeek（kafka-go 限制：
// consumer-group 模式不支持 Seek）。
func (c *Consumer) Seek(ctx context.Context, offset int64) error {
	if err := c.r.SetOffset(offset); err != nil {
		return ErrSeek.Wrap(err)
	}
	return nil
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./go-middleware/kafka/... -v`
Expected: PASS（全部用例）

- [ ] **Step 6: 运行完整校验**

```bash
go build ./go-middleware/... && \
  go vet ./go-middleware/... && \
  gofmt -l go-middleware/kafka/
```
Expected: 无输出（`gofmt -l` 无输出表示已格式化），`go build`/`go vet` 无错误

- [ ] **Step 7: Commit**

```bash
git add go-middleware/kafka/offset.go go-middleware/kafka/offset_test.go go-middleware/kafka/consumer.go
git commit -m "feat(kafka): add PartitionOffsets, Consumer.Lag and Consumer.Seek"
```

---

### Task 7: README 更新

**Files:**
- Modify: `go-middleware/README.md`

**Interfaces:**
- Consumes: 无（纯文档）

- [ ] **Step 1: 更新包一览表格中的 kafka 行**

在 `go-middleware/README.md` 中，将：

```markdown
| `kafka` | Kafka 生产者和消费者（基于 `github.com/segmentio/kafka-go`；OTel 追踪，可选） |
```

替换为：

```markdown
| `kafka` | Kafka 生产者和消费者（基于 `github.com/segmentio/kafka-go`；OTel 追踪，可选；DLQ 转发 + offset/lag 查询；含包内错误码 20201-20206） |
```

- [ ] **Step 2: 确认无遗留占位符**

Run: `grep -n "TBD\|TODO" go-middleware/README.md`
Expected: 无匹配（退出码非 0）

- [ ] **Step 3: Commit**

```bash
git add go-middleware/README.md
git commit -m "docs(kafka): document DLQ/offset error code range in README"
```

---

## 最终验收（全部任务完成后）

```bash
go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... && \
  go vet ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... && \
  gofmt -l $(find go-middleware/kafka -name '*.go') && \
  go test ./go-middleware/... -count=1
```

Expected: 全部通过，`gofmt -l` 无输出。

对照 Issue #55 验收标准逐项确认：
- [x] 确认上述设计问题 → 设计文档 + Issue 评论
- [x] 实现 DLQ 转发能力 → Task 3（`SendToDLQ`）+ Task 5（`HandleMessage`）
- [x] 实现 offset/lag 查询能力 → Task 6
- [x] 补充单元测试 → 每个 Task 均含测试
- [x] README / godoc 同步更新 → Task 7 + 各任务内联 godoc 注释
