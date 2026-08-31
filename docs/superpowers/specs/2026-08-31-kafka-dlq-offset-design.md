# Kafka DLQ 死信队列与 Offset 管理封装 — 设计文档

- Issue: #55
- Workflow: wf-2026-08-31-001
- 模块: `go-middleware/kafka`

## 背景

`go-middleware/kafka` 目前是 kafka-go 的薄封装（`Writer`/`Consumer`），已具备 SASL/TLS、重试退避、`WithTrace()` OTel 追踪。缺少两类微服务常见诉求：

1. **DLQ 死信队列**：消费失败或重试超限时，没有机制把消息转投到死信 topic。
2. **Offset 管理**：只有 `CommitMessages`，缺少 lag 查询、Seek、失败次数统计。

## 设计问题与决策

| # | 问题 | 决策 |
|---|------|------|
| 1 | DLQ 转发时机 | **两者都要**：`Writer.SendToDLQ` 工具方法（转发时机由业务决定）+ `Consumer.HandleMessage` 可选自动化封装（失败达阈值自动转发） |
| 2 | Lag 查询方式 | 基于 `kafka.Conn`（`ReadPartitions`/`ReadLastOffset`），不引入 admin `kafka.Client` |
| 3 | 失败次数存储 | 内存默认（进程级，跟随 Consumer 生命周期）+ `FailureCounter` 接口可扩展（为未来 Redis 实现预留，本次不实现） |
| 4 | Lag 功能边界 | 仅支持当前 Consumer 自身的 lag（`Reader.Stats()` 已消费 offset 与 `kafka.Conn` log-end offset 之差），不支持查询其他 consumer 的 lag（需要 admin API，超出本次范围） |

## 架构

新增 3 个文件 + 错误码扩展，不改动现有 `Writer`/`Consumer` 已有方法签名（向后兼容）。

```
go-middleware/kafka/
├── dlq.go              # DLQ 转发：Writer.SendToDLQ + Consumer.HandleMessage
├── failure_counter.go  # FailureCounter 接口 + 内存默认实现
├── offset.go           # PartitionOffsets / Consumer.Lag / Consumer.Seek
├── errors.go           # 追加 20204-20206
└── options.go           # 追加 WithDLQ / WithFailureCounter
```

## API 设计

### 1. DLQ 转发（`dlq.go`）

```go
// SendToDLQ 将消息转发到死信 topic，并附加失败原因与原始消息元信息作为 Header。
// dlqTopic 由调用方显式指定，Writer 实例不绑定固定的 DLQ topic，可复用任意 Writer。
func (w *Writer) SendToDLQ(ctx context.Context, dlqTopic string, msg kafka.Message, reason string) error
```

附加 Header：
- `x-dlq-reason`：失败原因（`reason` 参数原文）
- `x-dlq-original-topic`
- `x-dlq-original-partition`
- `x-dlq-original-offset`

内部复用 `WriteMessages`（含既有 trace 逻辑），保留原 msg.Key/Value/其余 Headers。

`DLQSender` 接口（供 `Consumer.HandleMessage` 依赖，解耦具体 `*Writer` 类型，便于单元测试注入 fake）：

```go
// DLQSender 定义 DLQ 转发能力，*Writer 天然实现该接口。
type DLQSender interface {
    SendToDLQ(ctx context.Context, dlqTopic string, msg kafka.Message, reason string) error
}
```

Consumer 侧自动化封装：

```go
// HandleMessage 执行 handler 处理消息；失败时按 msg.Key 累计失败次数，
// 达到 WithDLQ 配置的 maxAttempts 后自动转发 DLQ 并清零计数（返回 nil，
// 视为该消息已终结处理）；未达阈值则原样返回 handler 的 err 交由调用方决定重试/丢弃。
// 未通过 WithDLQ 配置 DLQ 时，等价于直接调用 handler（不做计数/转发）。
func (c *Consumer) HandleMessage(ctx context.Context, msg kafka.Message, handler func(context.Context, kafka.Message) error) error
```

### 2. 失败计数器（`failure_counter.go`）

```go
// FailureCounter 记录并查询按 key 维度的失败次数，供 Consumer.HandleMessage
// 判断是否达到 DLQ 转发阈值。默认实现为进程内存级（memFailureCounter），
// 可通过 WithFailureCounter 替换为跨实例存储（如 Redis，本次不提供实现）。
type FailureCounter interface {
    // Incr 递增 key 的失败计数并返回递增后的值。
    Incr(key string) int
    // Reset 清零 key 的失败计数（成功处理或已转发 DLQ 后调用）。
    Reset(key string)
}
```

默认实现 `memFailureCounter`：基于 `sync.Map[string]*int64`，`Incr` 用 `atomic.AddInt64`。

计数 key 默认取 `string(msg.Key)`（业务消息键）；`msg.Key` 为空时退化为 `topic/partition/offset`，避免所有空 key 消息共享一个计数器。

### 3. Offset/Lag 查询（`offset.go`）

```go
// PartitionOffsets 通过 kafka.Conn 查询 topic 各分区的 log-end offset
// （分区最新写入位置），不依赖 consumer group 状态，用完即关闭连接。
func PartitionOffsets(ctx context.Context, brokers []string, topic string) (map[int]int64, error)

// Lag 返回当前 Consumer 自身各分区的消费延迟：log-end offset（PartitionOffsets）
// 减去 Reader.Stats() 中已消费的 offset。仅反映本 Consumer 实例已消费的进度，
// 不能查询同一 group 内其他 consumer 或未分配给本实例的分区。
func (c *Consumer) Lag(ctx context.Context) (map[int]int64, error)

// Seek 定位到指定 offset 重新消费。要求 Consumer 以非 consumer-group 模式
// （ReaderConfig.GroupID 为空、显式指定 Partition）创建，否则返回 ErrSeek
// （kafka-go 限制：consumer-group 模式不支持 Seek）。
func (c *Consumer) Seek(ctx context.Context, offset int64) error
```

### 4. 错误码（追加到 `errors.go`）

```go
// CodeDLQForward DLQ 转发失败
CodeDLQForward = 20204
// CodeOffsetQuery offset/lag 查询失败
CodeOffsetQuery = 20205
// CodeSeek Seek 失败
CodeSeek = 20206
```

对应 `ErrDLQForward` / `ErrOffsetQuery` / `ErrSeek`，HTTP 状态均注册为 500，遵循现有 `errors.go` 模式。

### 5. Options（追加到 `options.go`）

```go
// WithDLQ 为 Consumer 启用失败自动转发 DLQ：sender 为 DLQ 转发目标
// （通常是另一个 *Writer），dlqTopic 为死信 topic，maxAttempts 为转发前
// 允许的最大失败次数（<= 0 时忽略本 Option，不启用自动转发）。
func WithDLQ(sender DLQSender, dlqTopic string, maxAttempts int) ClientOption

// WithFailureCounter 替换 Consumer 默认的内存失败计数器实现。
func WithFailureCounter(counter FailureCounter) ClientOption
```

`clientOptions` 结构体新增字段：`dlqSender DLQSender`、`dlqTopic string`、`dlqMaxAttempts int`、`failureCounter FailureCounter`。仅 `NewConsumer` 消费这些字段，`NewWriter` 忽略（与现有 `trace` 字段共享单一 `ClientOption` 类型的模式一致）。

## 测试策略

沿用现有 `kafka_test.go` 策略：**不依赖真实 Kafka broker**，测试范围为构造逻辑、纯函数逻辑、错误包装路径：

- `failure_counter_test.go`：`memFailureCounter` 的 Incr/Reset 并发安全性、多 key 隔离 — 纯逻辑，完全可测。
- `dlq_test.go`：
  - Header 构造逻辑（`x-dlq-reason` 等字段正确性）— 抽出纯函数 `buildDLQHeaders(msg, reason) []kafka.Header` 便于测试。
  - `Consumer.HandleMessage` 的重试计数/阈值触发/DLQ 转发/计数清零逻辑 — 通过实现 `DLQSender` 接口的 fake 注入，不需要真实网络连接。
- `offset_test.go`：`PartitionOffsets`/`Lag`/`Seek` 对无法连接的 broker 返回包装后的 `ErrOffsetQuery`/`ErrSeek`（参数校验 + 错误路径），不做真实网络集成测试。

## 文档更新

- `go-middleware/README.md`：补充 DLQ 转发、lag 查询、Seek 的使用示例（参考现有 kafka 小节风格）。
- 新增导出符号均需 godoc 注释（遵循 `.claude/rules/go.md` §8.3）。

## 范围之外（Out of Scope）

- Redis 等跨实例失败计数存储的具体实现（仅预留 `FailureCounter` 接口）。
- 跨 consumer / 整个 consumer group 的 lag 聚合查询（需要 admin `kafka.Client`）。
- DLQ 消息的重放/回灌工具（从 DLQ topic 读回原 topic）。
