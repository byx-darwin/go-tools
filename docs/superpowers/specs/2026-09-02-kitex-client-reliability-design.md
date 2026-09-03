# Kitex 客户端可靠性配置设计（熔断接线 + 超时默认值 + 重试退避）

- Issue: #93
- Date: 2026-09-02
- Classification: bounded（已有代码流程，改动范围为 `go-framework/config/kitex/client.go` + `go-framework/kitex/option/option.go`，单模块内）
- Status: approved

## 背景

`go-framework/config/kitex/client.go` 中定义了 `CBSuite`（熔断器配置）字段，但
`go-framework/kitex/option/option.go` 的 `NewClientOption` 从未消费该字段构造熔断
Option，是一处"看起来支持、实际空转"的死配置。同时：

- `RPCTimeout` 仅在显式配置 `>0` 时才设置，不配置即无超时，请求可无限悬挂。
- `ConnectTimeout` 硬编码 50ms，弱网/跨 AZ 场景易误判失败触发重试。
- 重试策略 `retry.NewFailurePolicy()+MaxRetryTimes` 无退避配置。

"有重试、无熔断"的组合下，下游抖动容易被放大成重试风暴。

## 设计

### 1. CBSuite 接线

Kitex（v0.16.3，本仓库当前依赖版本）内置 `pkg/circuitbreak.CBSuite`，客户端侧提供
`client.WithCircuitBreaker(*circuitbreak.CBSuite)`。不引入新依赖。

- `co.CBSuite.Enable == true` 时构造 `circuitbreak.NewCBSuite(keyFunc)` 并
  `client.WithCircuitBreaker(cb)`。
- **Key 函数可自定义**（本次评审补充要求）：`NewClientOption` 增加
  `Option`（functional option，符合 `.claude/rules/options-pattern.md`）
  `WithCircuitBreakerKeyFunc(func(ctx context.Context, ri rpcinfo.RPCInfo) string)`，
  未设置时回退到 SDK 默认 `circuitbreak.RPCInfo2Key`。
- 熔断阈值（错误率、最小样本量等）暂用 SDK 默认值，不在本次扩展（YAGNI）；后续如
  需要业务方自定义阈值再加字段/Option。

### 2. 超时默认值

| 配置项 | 现状 | 新默认值 |
|---|---|---|
| `ConnectTimeout` | 未配置时硬编码 50ms | 包级常量 `defaultConnectTimeout = 200 * time.Millisecond` |
| `RPCTimeout` | 未配置时不设置（无限等待） | 包级常量 `defaultRPCTimeout = 3 * time.Second`（评审确认） |

两者均保持"显式配置 `>0` 则覆盖默认值"的语义，不破坏现有配置行为。

### 3. 重试退避（Backoff）

`go-framework/config/kitex/client.go` 的 `FailureRetry` 结构体新增可选字段：

```go
// FailureRetry 重试机制
type FailureRetry struct {
    Enable        bool     `json:"enable"  yaml:"enable"`
    MaxRetryTimes int      `json:"max_retry_times" yaml:"max_retry_times"`
    BackOff       BackOff  `json:"backoff" yaml:"backoff"`
}

// BackOff 重试退避策略。Type 为空（默认）保持现状：不启用退避。
type BackOff struct {
    Type    string `json:"type" yaml:"type"` // "" | "fixed" | "random"
    FixedMS int    `json:"fixed_ms" yaml:"fixed_ms"`
    MinMS   int    `json:"min_ms" yaml:"min_ms"`
    MaxMS   int    `json:"max_ms" yaml:"max_ms"`
}
```

`NewClientOption` 在 `fp.WithMaxRetryTimes(...)` 之后按 `Type` 分支调用
`fp.WithFixedBackOff(FixedMS)` / `fp.WithRandomBackOff(MinMS, MaxMS)`；
`Type == ""` 时不调用（保持当前行为，向后兼容现有配置）。

### 4. Functional Option 扩展点

`NewClientOption` 签名扩展为 `NewClientOption(ctx, cfg, opts ...ClientOptionOpt) ([]client.Option, error)`，
新增 `Option` 类型仅用于承载 `WithCircuitBreakerKeyFunc`（当前唯一可选项），遵循
`.claude/rules/options-pattern.md` 的 `WithXxx` 命名与"默认值 + 应用 opts"顺序。

### 5. 模块边界

改动全部位于 `go-framework` 模块内（`config/kitex` + `kitex/option`），不新增对
`go-middleware` 的依赖，符合 `CLAUDE.md` 的 DAG 约束。

## 测试计划

`go-framework/kitex/option/option_test.go` 当前只覆盖 Server 侧，本次补充
`NewClientOption` 用例：

- CBSuite enable → `client.Options().CBSuite != nil`；disable → 为 nil。
- 自定义 `WithCircuitBreakerKeyFunc` 生效；未设置时使用 SDK 默认 key 函数（间接验证不 panic 且可正常构造）。
- `ConnectTimeout` / `RPCTimeout` 未配置时应用默认值；显式配置时覆盖默认值。
- `BackOff.Type` 为 `fixed` / `random` / `""` 三种场景下 `FailurePolicy.BackOffPolicy` 符合预期。

## 风险与兼容性

- 默认值变更（ConnectTimeout 50ms→200ms，RPCTimeout 无→3s）是行为变更，但方向是
  "更安全的默认值"，且都可被显式配置覆盖，风险可控。
- `FailureRetry.BackOff` 为新增可选字段，零值等价于"不启用"，不影响现有 YAML 配置。
- `NewClientOption` 增加可变参数 `opts ...Option`，是向后兼容的签名扩展（新增可变参数不破坏现有调用方）。
