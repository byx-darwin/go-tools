# go-middleware/tls OTel Tracing 接入 — 设计文档

- Issue: #54
- Workflow: wf-2026-09-01-002
- 模块: `go-middleware/tls`

## 背景

`go-middleware` 的 `db`/`es`/`clickhouse`/`kafka` 四个包已统一接入 `WithTrace()` OTel 追踪钩子（走全局 `otel.Tracer()`/`otel.GetTextMapPropagator()`，不引入 go-middleware 自己的 Provider 管理）。`tls`（火山引擎日志服务 TLS）是最后一个未接入的包。

`tls` 与前面四个包性质不同：传输的是日志数据而非查询/RPC；`Producer.SendLog` 只是写入内存缓冲区，真正的网络请求发生在 `flush`（达到 `BatchSize` 或 `FlushInterval` 触发，或 `Close` 时最终 flush）；`FileShipper` 在 `Producer` 基础上定时 tail 本地日志文件再喂给 `SendLog`。

代码现状（`go-middleware/tls/producer.go`）：
- `Producer.flush(_ context.Context) error` —— **传入的 ctx 参数当前被完全忽略**
- `flushLoop` 的定时/关闭触发的 flush 用全新的 `context.Background()` + 10s timeout，与任何 `SendLog` 调用方的 ctx 无关联
- `FileShipper.run()` 调用 `Producer.SendLog` 时用的是 `s.ctx`（shipper 自身生命周期 ctx，不携带业务 trace 上下文）

## 设计问题与决策

| # | 问题 | 决策 |
|---|------|------|
| 1 | Span 粒度 | 只给 `flush` 起 span（代表一次批量发送的真实网络往返）；`SendLog` 本身不单独起 span，避免对无网络 I/O 的内存写入产生大量低信噪比 span |
| 2 | 跨异步边界的关联方式 | `SendLog` 若发现调用方 `ctx` 携带有效 span，就把该 span 的 `SpanContext` 记入一个与 `buf` 平行的 pending-links 切片；触发 `flush` 时用 `trace.LinkFromContext` 把这些（去重后的）上游 span Link 到 `flush` span，随 `buf` 一起清空——类比 kafka 生产/消费端的 Link 模式，这里是"多个 `SendLog` 调用 Link 到一个 `flush` span" |
| 3 | FileShipper 场景的上下文差异化 | 不做特殊处理。`flushLoop` 定时/关闭触发用 `context.Background()`、`FileShipper.run()` 用 `s.ctx`，两者均不携带业务 span，对应的 `flush` span 自然没有 Link；决策 #2 的逻辑统一适用，无需额外分支 |
| 4 | 是否接入 metrics | 不做。除 `db`（借助现成的 `otelsql` 顺带获得连接池指标）外，`es`/`clickhouse`/`kafka` 均只做 tracing，`tls` 保持一致；metrics 留待后续独立 issue |

## 架构

新增 2 个文件，`Producer` 构造函数追加可变参数（向后兼容，无 opts 调用方不受影响）：

```
go-middleware/tls/
├── trace.go     # instrumentationName 常量 + endSpan(span, err) helper
├── options.go   # ProducerOption 类型 + WithTrace()
└── producer.go  # 追加 tracer 字段、pending-links 缓冲、flush 内起 span（改动既有文件）
```

## API 设计

```go
// ProducerOption 定义 NewProducer 的配置选项函数。
type ProducerOption func(*Producer)

// WithTrace 启用 OpenTelemetry 追踪：为每次 flush（批量发送）起一个 span，
// 并把触发本次 flush 的各 SendLog 调用方 span（若携带）Link 到该 span。
func WithTrace() ProducerOption {
	return func(p *Producer) {
		p.tracer = otel.Tracer(instrumentationName)
	}
}

// NewProducer 创建 TLS Producer，支持 Options 配置（如 WithTrace）。
func NewProducer(cfg ProducerConfig, opts ...ProducerOption) (*Producer, error)
```

`Producer` 新增字段：

```go
type Producer struct {
	// ...existing fields...
	tracer      trace.Tracer          // nil 时不追踪（零值安全，未启用 WithTrace 时行为不变）
	pendingCtx  []trace.SpanContext   // 与 buf 平行，记录待 flush 的各 SendLog 调用方有效 span
}
```

### flush 的 span 行为

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
		ctx, span = p.tracer.Start(ctx, "tls.flush", trace.WithLinks(spanLinks(links)...),
			trace.WithAttributes(
				attribute.String("tls.topic_id", p.config.TopicID),
				attribute.Int("tls.batch_size", len(logs)),
			))
		defer func() { endSpan(span, err) }()
	}

	_, err = p.client.PutLogsV2(&tls.PutLogsV2Request{
		TopicID:      p.config.TopicID,
		CompressType: "lz4",
		Source:       p.config.Source,
		Logs:         logs,
	})
	if err != nil {
		err = oops.With("tls.flush").Code(CodeSend).Wrap(err)
	}
	return err
}
```

（命名返回值 `err`，配合 `defer endSpan(span, err)` 在函数返回前拿到最终错误值，与 `flush` 现有的 `oops` 包装逻辑保持一致。）

（`SendLog` 记录 pending span：若 `ctx` 携带非零 `trace.SpanContextFromContext(ctx)`，追加到 `p.pendingCtx`；`links` 去重按 `TraceID+SpanID` 判重，避免同一上游 span 多条 `SendLog` 产生重复 Link。）

## 错误处理

- `flush` 出错时：`span.RecordError(err)` + `span.SetStatus(codes.Error, err.Error())`，与现有 `kafka`/`clickhouse` 的 `endSpan` helper 完全一致写法
- 未启用 `WithTrace()` 时 `p.tracer == nil`，跳过所有 span 相关逻辑，`SendLog`/`flush` 行为与当前实现完全一致（零开销、零行为变化）

## 测试

新增 `go-middleware/tls/trace_test.go`，使用内存 exporter（参照 `go-middleware/kafka/trace_test.go`、`go-middleware/clickhouse/trace_test.go` 写法）断言：

1. `flush` 产生的 span 名称为 `tls.flush`，属性含 `tls.topic_id`、`tls.batch_size`（等于实际发送条数）
2. 失败路径（`PutLogsV2` 返回 error）：span 记录 error 且 status 为 `codes.Error`
3. 多个携带 span 上下文的 `ctx` 触发 `SendLog` 后由同一次 flush 处理：`flush` span 的 Link 数量等于去重后的上游 span 数
4. 不携带 span 的 `ctx`（如 `context.Background()`，模拟 `flushLoop`/`FileShipper` 场景）触发的 `SendLog` → `flush`：span 正常产生但无 Link
5. 未调用 `WithTrace()` 时 `SendLog`/`flush` 行为不变（无 panic、无空指针）

## 文档

`go-middleware/README.md` 的 tls 章节补充 `WithTrace()` 用法示例，与 db/es/clickhouse/kafka 章节风格一致。

## Acceptance Criteria（同步自 Issue #54）

- [x] 确认 span 粒度方案（本设计文档）
- [ ] `tls.Producer` 新增 `WithTrace()` Option，风格与其余四个包一致
- [ ] 补充相应单元测试
- [ ] `go-middleware/README.md` 同步更新
