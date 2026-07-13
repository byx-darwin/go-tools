# DDD 分层日志封装设计

## 概述

在 `go-common/log` 包中实现 DDD 风格的领域日志系统，让 DDD 各层有合适的日志封装方式，核心解决三个问题：

1. **可观测性** — 按领域/分层分类日志，自动注入 `domain` 和 `category` 字段
2. **开发体验** — 各层有专属日志入口，不用手动拼 category
3. **oops 对齐** — 日志的 `domain` 与 oops 错误的 `In(domain)` 对齐，统一领域上下文

## 约束

### 向后兼容
- 不修改现有 `Logger`、`WithCategory()`、`L()` 等公开 API 的行为
- 新增功能为纯增量，不破坏已有代码

### 模块边界
- 所有新增代码仅在 `go-common/log` 包内
- 不引入新的外部依赖（oops 已在项目中使用）

### 质量要求
- 所有新增导出类型和函数必须有 godoc 注释
- 必须通过 `golangci-lint` 检查（revive, errcheck, gocritic 等）
- 单元测试覆盖率 ≥ 80%

## 错误处理与边界情况

### DomainLogger.Error() 非 oops 错误
当传入的 `err` 不是 oops 错误时：
- `ErrorAttrs(err)` 返回空切片
- 日志只包含 `error` 字段（原始错误信息），不包含 `error.code` 等 oops 字段
- 行为与现有 `Logger.ErrorContext()` 一致

### 空 domain 名称
`NewDomainLogger("")` 应该：
- 不 panic
- `domain` 字段为空字符串或省略
- 建议：文档中说明应传入有意义的 domain 名称

### nil error
`DomainLogger.Error(msg, nil)` 应该：
- 仍然输出 Error 级别日志
- `error` 字段为空或省略
- 不 panic

## 并发安全

- `NewDomainLogger(domain)` 返回的 `DomainLogger` 是并发安全的
- 内部持有的 `*Logger` 由全局 `L()` 返回，本身并发安全
- `domainHandler` 是不可变结构（创建后字段不变），天然并发安全
- 分层便捷函数 `App()`、`DB()` 等每次调用返回新的 `*Logger`，无共享状态

## 设计决策

### 决策 1：领域层采用接口注入（端口模式）

**选择**：定义 `DomainLogger` 接口，通过构造函数注入。

**理由**：符合 DDD 依赖反转原则，领域层不依赖具体日志框架，测试时可替换为 mock。

**其他方案**：
- 严格 DDD（领域层无日志）→ 业务决策过程丢失，排查困难
- 约定式（统一 Logger + 约定）→ 无编译期约束，领域层可能写出基础设施内容

### 决策 2：领域层使用业务语义方法

**选择**：`Decision`、`Event`、`Error` 三个语义方法，而非技术级别。

**理由**：领域层只关心业务语义，不关心日志级别。

| 方法 | 日志级别 | 场景 |
|------|---------|------|
| `Decision(accepted=true)` | Info | 业务决策通过（如"订单创建成功"） |
| `Decision(accepted=false)` | Warn | 业务决策拒绝（如"余额不足拒绝"） |
| `Event` | Info | 领域事件（如"订单已创建"） |
| `Error` | Error | 业务异常（如"支付处理失败"） |

### 决策 3：其他层使用便捷函数

**选择**：`log.App(ctx)`、`log.DB(ctx)`、`log.Access(ctx)` 等便捷函数。

**理由**：其他层可以依赖日志框架，不需要接口抽象。便捷函数本质是 `WithCategory` + 自动注入 ctx 上下文。

### 决策 4：文件分离保持按等级

**选择**：保持现有按日志等级分文件（error.log、warn.log、info.log）。

**理由**：现有方式已满足需求，`domain` 和 `category` 作为日志字段用于检索过滤，不需要额外复杂度。

## 详细设计

### 1. DomainLogger 接口

```go
// DomainLogger 领域层日志端口接口。
//
// 领域层通过此接口记录业务日志，不依赖具体日志框架。
// 由应用层提供实现（domainLoggerAdapter）。
type DomainLogger interface {
    // Decision 记录业务决策。
    // accepted=true 输出 Info 级别，accepted=false 输出 Warn 级别。
    // 输出字段：log_type="decision", accepted=bool。
    Decision(msg string, accepted bool, args ...any)

    // Event 记录领域事件，Info 级别。
    // 输出字段：log_type="event"。
    Event(msg string, args ...any)

    // Error 记录业务异常，Error 级别。
    // 自动提取 oops 错误属性（error.code, error.domain 等）。
    Error(msg string, err error, args ...any)
}
```

### 2. DomainLogger 适配器

```go
// domainLoggerAdapter 实现 DomainLogger 接口，桥接到 log.Logger。
type domainLoggerAdapter struct {
    logger *Logger
    domain string
}

// NewDomainLogger 创建领域日志适配器。
//
// 注入 "domain" 字段到所有日志记录。
// 用法：
//
//	svc := domain.NewOrderService(log.NewDomainLogger("order"))
func NewDomainLogger(domain string) DomainLogger {
    return &domainLoggerAdapter{
        logger: L(),
        domain: domain,
    }
}
```

输出 JSON 示例：

```json
{"level":"info", "domain":"order", "log_type":"decision", "msg":"订单创建", "accepted":true, "order_id":"123"}
{"level":"warn", "domain":"order", "log_type":"decision", "msg":"余额不足拒绝", "accepted":false, "user_id":"456"}
{"level":"info", "domain":"order", "log_type":"event", "msg":"订单已创建", "order_id":"123"}
{"level":"error", "domain":"order", "log_type":"error", "msg":"支付处理失败", "error":"timeout", "error.code":40312}
```

### 3. domainHandler

```go
// domainHandler 在日志中注入 domain 和 log_type 字段。
type domainHandler struct {
    next    slog.Handler
    domain  string
    logType string
}
```

### 4. 分层便捷函数

返回 `*Logger`（保持封装），调用方通过 `InfoContext(ctx, ...)` 等方法传入 ctx。

```go
// App 返回应用层 Logger（自动注入 category="app"）。
func App(ctx context.Context) *Logger {
    return L().WithCategory(CategoryApp)
}

// DB 返回基础设施层 Logger（category="db"）。
func DB(ctx context.Context) *Logger {
    return L().WithCategory(CategoryDB)
}

// Access 返回展示层 Logger（category="access"）。
func Access(ctx context.Context) *Logger {
    return L().WithCategory(CategoryAccess)
}

// RPC 返回 RPC 层 Logger（category="rpc"）。
func RPC(ctx context.Context) *Logger {
    return L().WithCategory(CategoryRPC)
}

// MQ 返回消息队列层 Logger（category="mq"）。
func MQ(ctx context.Context) *Logger {
    return L().WithCategory(CategoryMQ)
}

// Cache 返回缓存层 Logger（category="cache"）。
func Cache(ctx context.Context) *Logger {
    return L().WithCategory(CategoryCache)
}
```

**说明：** ctx 参数保留用于未来扩展（如自动注入 ctx 中的自定义字段），当前实现直接返回带 category 的 Logger。调用方使用 `InfoContext(ctx, msg, args...)` 传入 ctx 以自动关联 trace_id。

### 5. 与 hlog/klog 的级别映射

现有适配器映射保持不变：

| hlog/klog 级别 | slog 级别 | 来源 |
|---------------|----------|------|
| Trace | Debug | 基础设施层 |
| Debug | Debug | 基础设施层 |
| Info | Info | 领域 Decision(true) / Event |
| Notice | Info | 领域 Decision(true) / Event |
| Warn | Warn | 领域 Decision(false) |
| Error | Error | 领域 Error |
| Fatal | Error | 领域 Error |

### 6. 与 oops 错误系统集成

`DomainLogger.Error()` 方法内部调用 `ErrorAttrs(err)` 提取 oops 错误的结构化字段：

- `error.code` — 错误码
- `error.domain` — 错误领域（与日志的 `domain` 字段互补）
- `error.hint` — 错误提示
- `error.public` — 公开消息

### 7. 新增 category 常量

```go
const (
    CategoryApp   = "app"    // 应用层
    CategoryCache = "cache"  // 缓存操作
    CategoryMQ    = "mq"     // 消息队列
)
```

现有常量 `CategoryAccess`、`CategoryBiz`、`CategoryRPC`、`CategoryDB`、`CategoryError`、`CategoryPanic`、`CategoryAudit`、`CategorySecurity` 保持不变。

## 使用示例

### 领域层

```go
// domain/order_service.go
type OrderService struct {
    logger log.DomainLogger
    repo   OrderRepository
}

func NewOrderService(logger log.DomainLogger, repo OrderRepository) *OrderService {
    return &OrderService{logger: logger, repo: repo}
}

func (s *OrderService) CreateOrder(ctx context.Context, cmd CreateOrderCmd) error {
    // 业务决策
    if !s.checkBalance(cmd.UserID, cmd.Amount) {
        s.logger.Decision("余额不足拒绝", false, "user_id", cmd.UserID, "amount", cmd.Amount)
        return ErrBalanceInsufficient
    }

    // 领域事件
    order := s.createOrder(cmd)
    s.logger.Event("订单已创建", "order_id", order.ID, "user_id", cmd.UserID)

    return nil
}
```

### 应用层

```go
// application/order_handler.go
func (h *OrderHandler) HandleCreate(ctx context.Context, req *CreateRequest) error {
    log.App(ctx).InfoContext(ctx, "创建订单请求", "user_id", req.UserID)

    err := h.service.CreateOrder(ctx, domain.CreateOrderCmd{...})
    if err != nil {
        log.App(ctx).ErrorContext(ctx, "创建订单失败", "error", err)
        return err
    }

    log.App(ctx).InfoContext(ctx, "创建订单成功")
    return nil
}
```

### 基础设施层

```go
// infrastructure/order_repo.go
func (r *OrderRepo) Save(ctx context.Context, order *Order) error {
    log.DB(ctx).DebugContext(ctx, "保存订单", "order_id", order.ID)
    // 执行 SQL...
    return nil
}
```

### 组装

```go
// main.go 或 wire.go
orderLogger := log.NewDomainLogger("order")
orderRepo := infrastructure.NewOrderRepo(db)
orderService := domain.NewOrderService(orderLogger, orderRepo)
orderHandler := application.NewOrderHandler(orderService)
```

## 文件结构

```
go-common/log/
├── category.go          # 新增 CategoryApp, CategoryCache, CategoryMQ 常量
├── domain.go            # 新增 DomainLogger 接口 + domainLoggerAdapter + domainHandler
├── domain_test.go       # 新增测试
├── layer.go             # 新增 App(), DB(), Access(), RPC(), MQ(), Cache() 便捷函数
├── layer_test.go        # 新增测试
├── handler.go           # 现有，新增 domainHandler
└── ...
```

## 不包含的内容

- ❌ 按领域分文件输出（保持按等级分文件）
- ❌ `Config.Categories` 的自动预创建子 Logger（后续可独立实现）
- ❌ 结构化领域对象（`type Domain struct`）— 字符串即可
- ❌ DomainEvent 机制 — 直接通过接口记录
- ❌ 源码位置精确追踪 — slog 无公开 call depth API，便捷函数和适配器的 `source` 指向框架代码而非业务代码。通过 `category` + `domain` + `trace_id` 组合定位已足够。

## 已知限制

**源码位置（AddSource）**：当开启 `AddSource` 时，通过便捷函数（`log.App(ctx).InfoContext`）或领域适配器（`logger.Decision`）输出的日志，`source` 字段会指向框架代码（`layer.go` 或 `domain.go`）而非业务调用点。这是 slog 当前 API 的限制（无公开的 caller skip 机制），不影响日志检索和排查。
