# go-tools 代码质量全面改进 — 设计文档

**日期**: 2026-07-13
**模式**: Full (方案 A：逐维度顺序推进)
**状态**: 待实施

## 1. 概述

对 go-tools 仓库进行四维度代码质量改进，按顺序逐个推进：

1. **错误处理统一** — fmt.Errorf → oops + 错误码体系
2. **API 设计改进** — interface{} → any，补充 Options 模式，完善 godoc
3. **测试覆盖率提升** — 补充缺失测试的核心包
4. **日志/可观测性清理** — TODO 收尾，结构化日志一致性检查

### 1.1 约束原则

- 每个维度独立成 spec/plan/PR，不混在一起
- 保持向后兼容：Deprecated 函数标记但不删除
- 不改功能行为，只改质量
- 每个维度内按 go-common → go-auth → go-middleware → go-framework 顺序推进

---

## 2. 维度 1：错误处理统一

### 2.1 目标

将所有 `fmt.Errorf` / `errors.New` 迁移到 `oops` + `go-common/error` 错误码体系，使错误：
- 有统一的结构化格式（oops 提供 stack trace + error code）
- 有明确的错误码（可被 API 层直接使用）
- 可被 `errors.Is` / `errors.As` 正常链式匹配

### 2.2 当前现状

| 类型 | 数量 | 位置 |
|------|------|------|
| `fmt.Errorf` | 24 处 | jwt(6), tls(6), observability(6), config(2), auth(1), clickhouse(1), response(1), tracer(2) |
| `errors.New` | 8 处 | 分散在各模块 |
| `oops` 已用 | 29 处 | 部分迁移完成 |

### 2.3 迁移清单

#### go-auth/jwt/token.go（6处 fmt.Errorf）

```go
// Before:
return "", fmt.Errorf("jwt.Sign: claims type %T does not implement jwt.Claims", claims)
return "", fmt.Errorf("jwt.Sign: failed to sign token: %w", err)
return nil, fmt.Errorf("jwt.Verify: claims type %T does not implement jwt.Claims", zero)
return nil, autherror.ErrTokenInvalid.Wrap(fmt.Errorf("jwt.Verify: invalid claims type"))
return "", fmt.Errorf("jwt.Refresh: %w", err)

// After:
// 在 go-auth/error/error.go 新增错误码：
//   CodeJWTSignFailed   = 40007
//   CodeJWTVerifyFailed = 40008
//   CodeJWTRefreshFailed = 40009
//
// 使用 oops 构造：
return "", oops.With("jwt.Sign").
    WithCode(autherror.CodeJWTSignFailed).
    Errorf("claims type %T does not implement jwt.Claims", claims)
```

#### go-middleware/tls/（producer.go 4处 + shipper.go 2处）

新增 `go-middleware/tls/error.go`，定义错误码：
```go
// TLS 错误码 20501-20599。
const (
    CodeMissingEndpoint   = 20501 // 缺少 endpoint 参数
    CodeMissingTopicID    = 20502 // 缺少 topic_id 参数
    CodeMissingRegion     = 20503 // 缺少 region 参数
    CodeMissingFilePath   = 20504 // 缺少 file_path 参数
    CodeCreateProducer    = 20505 // 创建 producer 失败
)
```

#### go-middleware/clickhouse/client.go（1处）

新增 `go-middleware/clickhouse/error.go`，定义错误码：
```go
// ClickHouse 错误码 20401-20499。
const (
    CodeParseDSN = 20401 // DSN 解析失败
)
```

#### go-framework/config/polaris.go（2处）

在 `go-framework/config/` 中新增错误码定义：
```go
// Config 错误码 10101-10199。
const (
    CodePolarisInit     = 10101 // Polaris 初始化失败
    CodePolarisGetConfig = 10102 // Polaris 获取配置文件失败
)
```

#### go-framework/hertz/observability/provider.go + kitex/observability/provider.go（各3处）

新增 `go-framework/` 共享错误码（或在各自包中定义）：
```go
// Observability 错误码 10201-10299。
const (
    CodeCreateTraceExporter  = 10201 // 创建 trace exporter 失败
    CodeCreateMetricExporter = 10202 // 创建 metric exporter 失败
    CodeStartRuntimeMetrics  = 10203 // 启动 runtime metrics 失败
)
```

#### go-framework/hertz/middleware/auth.go（1处）

```go
// Before:
return "", "", 0, fmt.Errorf("authorization base64 decode: %w", err)
// After: 使用 go-framework 已有的 auth 错误码
```

### 2.4 不迁移的情况

- `tracer.go` 中 `fmt.Errorf("panic: %s", msg)` — 这是记录 panic 信息到 OTel span，不需要错误码
- `response.go` 中 `fmt.Errorf("panic: %v", rec)` — 同上，panic 恢复记录
- 测试代码中的 `fmt.Errorf` — 保持不变

### 2.5 错误码总表（新增部分）

```
go-auth/error:
  40007 — JWT 签名失败 (CodeJWTSignFailed)
  40008 — JWT 验证失败 (CodeJWTVerifyFailed)
  40009 — JWT 刷新失败 (CodeJWTRefreshFailed)

go-middleware/tls:
  20501-20505 — TLS 模块错误码

go-middleware/clickhouse:
  20401 — DSN 解析失败

go-framework/config:
  10101-10102 — Polaris 配置错误

go-framework/hertz+kitex/observability:
  10201-10203 — OTel 初始化错误
```

---

## 3. 维度 2：API 设计改进

### 3.1 `interface{}` → `any` 迁移

| 文件 | 当前用法 | 处理方式 |
|------|---------|---------|
| `go-framework/config/config.go:25` | `Load(path string, v interface{}) error` | → `v any` |
| `go-framework/config/config.go:34` | `UnmarshalYAML(func(interface{}) error)` | 保留（外部库签名） |
| `go-middleware/tls/shipper.go:129` | `map[string]interface{}` | → `map[string]any` |
| `go-framework/kitex/middleware/accesslog.go:22` | `Endpoint func(ctx, req, resp interface{}) error` | 保留（兼容 Kitex） |
| `go-framework/kitex/log/adapter.go` | `v ...interface{}` (多处) | 保留（兼容 klog 接口） |

### 3.2 补充 Options 模式

#### middleware/db.NewDB

```go
// Before (4 params):
func NewDB(ctx context.Context, driver, source string, cfg *Config) (*DB, func(), error)

// After:
func NewDB(ctx context.Context, opts ...Option) (*DB, func(), error)

// WithDriver 设置数据库驱动（mysql, postgres 等）。
func WithDriver(driver string) Option

// WithSource 设置数据库连接字符串。
func WithSource(source string) Option

// WithConfig 设置数据库配置。
func WithConfig(cfg *Config) Option
```

#### framework/config.LoadPolarisConfig

```go
// Before (5 params):
func LoadPolarisConfig(namespace, fileGroup, fileName string, ...)

// After:
func LoadPolarisConfig(opts ...Option) (*PolarisConfigFile, error)

// WithNamespace 设置 Polaris 命名空间。
func WithNamespace(ns string) Option

// WithFileGroup 设置文件组。
func WithFileGroup(group string) Option

// WithFileName 设置文件名。
func WithFileName(name string) Option
```

### 3.3 Deprecated 函数确认

以下 Deprecated 函数保留，确认都有替代 API：

| Deprecated 函数 | 替代 API | 操作 |
|----------------|---------|------|
| `captcha.NewCacheStoreWithConfig` | `captcha.NewCacheStore(opts...)` | 补充迁移注释 |
| `captcha.NewCacheStoreWithTTL` | `captcha.NewCacheStore(opts...)` | 补充迁移注释 |
| `captcha.NewImageCaptchaLegacy` | `captcha.NewImageCaptcha(opts...)` | 补充迁移注释 |
| `log.NewFromLegacyConfig` | `log.New(opts...)` | 补充迁移注释 |

### 3.4 godoc 补充

重点补充以下包的 godoc 注释：
- `go-framework/config/` — `Loader` 接口、`PolarisConfigFile` 类型、`LoadYAML`/`MustLoadYAML` 函数
- `go-common/crypto/` — `DecodeTeaStr`、`EncodeTeaStr`、`GetTeaPadLen`、`TeaHexDecode`
- `go-middleware/kafka/` — `NewWriter`、`NewConsumer`
- `go-middleware/es/` — `NewClient`
- `go-middleware/clickhouse/` — `NewClient`

---

## 4. 维度 3：测试覆盖率提升

### 4.1 目标

补充缺失测试的包，重点覆盖核心功能逻辑。

### 4.2 测试补充清单

| 包 | 源文件数 | 优先级 | 测试重点 |
|----|---------|--------|---------|
| `go-framework/hertz/observability` | 7 | 🔴 高 | Provider 初始化、Tracer 中间件、Runtime metrics、Propagator 配置 |
| `go-framework/kitex/option` | 1 | 🟡 中 | ServerOption/ClientOption 构造和参数映射 |
| `go-framework/kitex/middleware/compat` | 1 | 🟡 中 | 兼容性中间件逻辑 |

### 4.3 测试策略

- **单元测试**：使用 `testify/assert` + `testify/require`
- **Mock 策略**：observability 测试中使用 OTel `sdktest` 导出器，不依赖真实外部服务
- **Table-driven tests**：所有新测试使用表驱动模式
- **覆盖目标**：核心逻辑路径 >80% 覆盖率

### 4.4 observability 测试计划

`go-framework/hertz/observability` 测试文件规划：

```go
provider_test.go:
  TestNewProvider_Success
  TestNewProvider_TraceExporterError
  TestNewProvider_MetricExporterError
  TestProvider_Shutdown

tracer_test.go:
  TestNewServerTracer
  TestTracer_Middleware_TracesRequest
  TestTracer_Middleware_RecordsPanic

metrics_test.go:
  TestStartRuntimeMetrics
  TestRegisterCustomMetrics

propagator_test.go:
  TestPropagator_W3C
  TestPropagator_B3
  TestPropagator_Composite
```

---

## 5. 维度 4：日志/可观测性清理

### 5.1 目标

日志系统设计良好，此维度主要是收尾清理。

### 5.2 清理清单

| 项目 | 位置 | 操作 |
|------|------|------|
| TODO 清理 | `go-common/log/config.go:21` | 实现或移除 TODO |
| 结构化日志一致性 | 全局扫描 | 确认无 `log.Printf` 风格调用 |
| 日志级别一致性 | 全局扫描 | 确认无 `fmt.Println` 残留 |

### 5.3 范围说明

此维度工作量最小。如果扫描后确认没有问题，可直接标记完成。

---

## 6. 实施节奏

每个维度的实施流程：

```
设计文档 → Issue 创建 → 实施计划 → Worktree 开发 → 测试 → PR → Code Review → 合并
```

各维度预期工作量：

| 维度 | 预计文件改动数 | 预计新增文件 | 复杂度 |
|------|-------------|------------|--------|
| 1. 错误处理 | ~12 文件修改 | 4-5 新 error.go | 中 |
| 2. API 设计 | ~8 文件修改 | 0 | 低 |
| 3. 测试覆盖 | 0 修改 | 4-6 新 *_test.go | 中 |
| 4. 日志清理 | 1-3 文件修改 | 0 | 低 |

---

## 7. 风险与缓解

| 风险 | 缓解措施 |
|------|---------|
| 错误码变更影响下游 | 新增错误码不修改已有码；fmt.Errorf → oops 保持 errors.Is 链兼容 |
| Options 重构破坏调用方 | 旧签名保留为 Deprecated wrapper，新签名并行存在 |
| 测试依赖外部服务 | 使用 mock/in-memory 实现，CI 可独立运行 |
| 多 PR 合并冲突 | 每个维度按模块顺序推进，维度间独立分支 |
