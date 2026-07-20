# DDD 领域日志封装 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 go-common/log 包中实现 DDD 分层日志系统，包括 DomainLogger 接口、domainHandler、分层便捷函数和 oops 错误集成。

**Architecture:** 领域层通过 DomainLogger 接口（端口模式）记录业务日志，不依赖具体日志框架。适配器 domainLoggerAdapter 桥接接口和底层 Logger，自动注入 domain 和 log_type 字段。其他层通过 App()、DB() 等便捷函数获取带 category 的 Logger。

**Tech Stack:** Go 1.25+, slog, oops (github.com/samber/oops)

## Global Constraints

- 不修改现有 Logger、WithCategory()、L() 等公开 API 的行为
- 所有新增代码仅在 go-common/log 包内
- 不引入新的外部依赖（oops 已在项目中使用）
- 所有新增导出类型和函数必须有 godoc 注释
- 必须通过 golangci-lint 检查
- 单元测试覆盖率 ≥ 80%

---

### Task 1: 新增 Category 常量

**Files:**
- Modify: `go-common/log/category.go`

**Interfaces:**
- Produces: `CategoryApp`, `CategoryCache`, `CategoryMQ` 常量

- [ ] **Step 1: 添加新常量**

在 `go-common/log/category.go` 中添加三个新常量：

```go
const (
    // CategoryApp 应用层日志。
    CategoryApp = "app"

    // CategoryCache 缓存操作日志。
    CategoryCache = "cache"

    // CategoryMQ 消息队列日志。
    CategoryMQ = "mq"
)
```

- [ ] **Step 2: 运行测试验证无破坏**

Run: `cd go-common && go test ./log/... -count=1`
Expected: PASS，现有测试全部通过

- [ ] **Step 3: 提交**

```bash
git add go-common/log/category.go
git commit -m "feat(log): add CategoryApp, CategoryCache, CategoryMQ constants"
```

---

### Task 2: 实现 domainHandler

**Files:**
- Modify: `go-common/log/handler.go`
- Create: `go-common/log/handler_domain_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `NewDomainHandler(next slog.Handler, domain, logType string) slog.Handler`

- [ ] **Step 1: 编写失败测试**

创建 `go-common/log/handler_domain_test.go`：

```go
package log

import (
    "bytes"
    "context"
    "encoding/json"
    "log/slog"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestDomainHandler_InjectsFields(t *testing.T) {
    var buf bytes.Buffer
    handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
    domainHandler := NewDomainHandler(handler, "order", "decision")

    logger := slog.New(domainHandler)
    logger.InfoContext(context.Background(), "test message")

    var result map[string]any
    require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

    assert.Equal(t, "order", result["domain"])
    assert.Equal(t, "decision", result["log_type"])
    assert.Equal(t, "test message", result["msg"])
}

func TestDomainHandler_WithAttrs(t *testing.T) {
    var buf bytes.Buffer
    handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
    domainHandler := NewDomainHandler(handler, "user", "event")

    logger := slog.New(domainHandler).With("user_id", "123")
    logger.InfoContext(context.Background(), "user created")

    var result map[string]any
    require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

    assert.Equal(t, "user", result["domain"])
    assert.Equal(t, "event", result["log_type"])
    assert.Equal(t, "123", result["user_id"])
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd go-common && go test ./log/... -run TestDomainHandler -v`
Expected: FAIL — `NewDomainHandler` 未定义

- [ ] **Step 3: 实现 domainHandler**

在 `go-common/log/handler.go` 末尾添加：

```go
// domainHandler 在日志中注入 domain 和 log_type 字段。
type domainHandler struct {
    next    slog.Handler
    domain  string
    logType string
}

// NewDomainHandler 创建 domain handler。
func NewDomainHandler(next slog.Handler, domain, logType string) slog.Handler {
    return &domainHandler{next: next, domain: domain, logType: logType}
}

func (h *domainHandler) Enabled(ctx context.Context, level slog.Level) bool {
    return h.next.Enabled(ctx, level)
}

func (h *domainHandler) Handle(ctx context.Context, r slog.Record) error {
    if h.domain != "" {
        r.AddAttrs(slog.String("domain", h.domain))
    }
    if h.logType != "" {
        r.AddAttrs(slog.String("log_type", h.logType))
    }
    return h.next.Handle(ctx, r)
}

func (h *domainHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    return &domainHandler{next: h.next.WithAttrs(attrs), domain: h.domain, logType: h.logType}
}

func (h *domainHandler) WithGroup(name string) slog.Handler {
    return &domainHandler{next: h.next.WithGroup(name), domain: h.domain, logType: h.logType}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd go-common && go test ./log/... -run TestDomainHandler -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/log/handler.go go-common/log/handler_domain_test.go
git commit -m "feat(log): add domainHandler for DDD domain logging"
```

---

### Task 3: 实现 DomainLogger 接口和适配器

**Files:**
- Create: `go-common/log/domain.go`
- Create: `go-common/log/domain_test.go`

**Interfaces:**
- Consumes: `NewDomainHandler`, `L()`, `ErrorAttrs()`
- Produces: `DomainLogger` 接口, `NewDomainLogger(domain string) DomainLogger`

- [ ] **Step 1: 编写失败测试**

创建 `go-common/log/domain_test.go`：

```go
package log

import (
    "bytes"
    "context"
    "encoding/json"
    "errors"
    "log/slog"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestDomainLogger_Decision_Accepted(t *testing.T) {
    var buf bytes.Buffer
    handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
    logger := &Logger{Logger: slog.New(handler), level: slog.LevelDebug}
    SetDefault(logger)
    defer SetDefault(nil)

    dl := NewDomainLogger("order")
    dl.Decision("订单创建", true, "order_id", "123")

    var result map[string]any
    require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

    assert.Equal(t, "info", result["level"])
    assert.Equal(t, "order", result["domain"])
    assert.Equal(t, "decision", result["log_type"])
    assert.Equal(t, true, result["accepted"])
    assert.Equal(t, "订单创建", result["msg"])
}

func TestDomainLogger_Decision_Rejected(t *testing.T) {
    var buf bytes.Buffer
    handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
    logger := &Logger{Logger: slog.New(handler), level: slog.LevelDebug}
    SetDefault(logger)
    defer SetDefault(nil)

    dl := NewDomainLogger("order")
    dl.Decision("余额不足拒绝", false, "user_id", "456")

    var result map[string]any
    require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

    assert.Equal(t, "warn", result["level"])
    assert.Equal(t, "order", result["domain"])
    assert.Equal(t, "decision", result["log_type"])
    assert.Equal(t, false, result["accepted"])
}

func TestDomainLogger_Event(t *testing.T) {
    var buf bytes.Buffer
    handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
    logger := &Logger{Logger: slog.New(handler), level: slog.LevelDebug}
    SetDefault(logger)
    defer SetDefault(nil)

    dl := NewDomainLogger("order")
    dl.Event("订单已创建", "order_id", "789")

    var result map[string]any
    require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

    assert.Equal(t, "info", result["level"])
    assert.Equal(t, "order", result["domain"])
    assert.Equal(t, "event", result["log_type"])
}

func TestDomainLogger_Error_WithOopsError(t *testing.T) {
    var buf bytes.Buffer
    handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
    logger := &Logger{Logger: slog.New(handler), level: slog.LevelDebug}
    SetDefault(logger)
    defer SetDefault(nil)

    dl := NewDomainLogger("payment")
    oopsErr := errors.New("timeout") // 普通错误，非 oops
    dl.Error("支付处理失败", oopsErr)

    var result map[string]any
    require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

    assert.Equal(t, "error", result["level"])
    assert.Equal(t, "payment", result["domain"])
    assert.Equal(t, "error", result["log_type"])
    assert.Equal(t, "timeout", result["error"])
}

func TestDomainLogger_Error_WithNilError(t *testing.T) {
    var buf bytes.Buffer
    handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
    logger := &Logger{Logger: slog.New(handler), level: slog.LevelDebug}
    SetDefault(logger)
    defer SetDefault(nil)

    dl := NewDomainLogger("order")
    dl.Error("异常日志", nil) // nil error 不应 panic

    var result map[string]any
    require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

    assert.Equal(t, "error", result["level"])
    assert.Equal(t, "order", result["domain"])
}

func TestDomainLogger_EmptyDomain(t *testing.T) {
    var buf bytes.Buffer
    handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
    logger := &Logger{Logger: slog.New(handler), level: slog.LevelDebug}
    SetDefault(logger)
    defer SetDefault(nil)

    dl := NewDomainLogger("") // 空 domain 不应 panic
    dl.Event("测试事件")

    var result map[string]any
    require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

    assert.Equal(t, "event", result["log_type"])
    // domain 字段可能不存在或为空
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd go-common && go test ./log/... -run TestDomainLogger -v`
Expected: FAIL — `DomainLogger`, `NewDomainLogger` 未定义

- [ ] **Step 3: 实现 DomainLogger 接口和适配器**

创建 `go-common/log/domain.go`：

```go
package log

import (
    "log/slog"
)

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

// domainLoggerAdapter 实现 DomainLogger 接口，桥接到 log.Logger。
type domainLoggerAdapter struct {
    logger *Logger
    domain string
}

// NewDomainLogger 创建领域日志适配器。
//
// 注入 "domain" 字段到所有日志记录。
// domain 应为有意义的领域名称，如 "order"、"payment"。
//
// 用法：
//
//	svc := domain.NewOrderService(log.NewDomainLogger("order"))
func NewDomainLogger(domain string) DomainLogger {
    return &domainLoggerAdapter{
        logger: L(),
        domain: domain,
    }
}

// Decision 记录业务决策。
func (a *domainLoggerAdapter) Decision(msg string, accepted bool, args ...any) {
    logType := "decision"
    handler := NewDomainHandler(a.logger.Handler(), a.domain, logType)
    logger := slog.New(handler)

    allArgs := append([]any{"accepted", accepted}, args...)
    if accepted {
        logger.Info(msg, allArgs...)
    } else {
        logger.Warn(msg, allArgs...)
    }
}

// Event 记录领域事件。
func (a *domainLoggerAdapter) Event(msg string, args ...any) {
    logType := "event"
    handler := NewDomainHandler(a.logger.Handler(), a.domain, logType)
    logger := slog.New(handler)
    logger.Info(msg, args...)
}

// Error 记录业务异常。
func (a *domainLoggerAdapter) Error(msg string, err error, args ...any) {
    logType := "error"
    handler := NewDomainHandler(a.logger.Handler(), a.domain, logType)
    logger := slog.New(handler)

    allArgs := args
    if err != nil {
        allArgs = append(allArgs, "error", err.Error())
        allArgs = append(allArgs, ErrorAttrs(err)...)
    }
    logger.Error(msg, allArgs...)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd go-common && go test ./log/... -run TestDomainLogger -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/log/domain.go go-common/log/domain_test.go
git commit -m "feat(log): add DomainLogger interface and adapter for DDD"
```

---

### Task 4: 实现分层便捷函数

**Files:**
- Create: `go-common/log/layer.go`
- Create: `go-common/log/layer_test.go`

**Interfaces:**
- Consumes: `L()`, `WithCategory()`, `CategoryApp`, `CategoryDB`, `CategoryAccess`, `CategoryRPC`, `CategoryMQ`, `CategoryCache`
- Produces: `App()`, `DB()`, `Access()`, `RPC()`, `MQ()`, `Cache()` 函数

- [ ] **Step 1: 编写失败测试**

创建 `go-common/log/layer_test.go`：

```go
package log

import (
    "bytes"
    "context"
    "encoding/json"
    "log/slog"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestLayerFunctions_Category(t *testing.T) {
    var buf bytes.Buffer
    handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
    logger := &Logger{Logger: slog.New(handler), level: slog.LevelDebug}
    SetDefault(logger)
    defer SetDefault(nil)

    tests := []struct {
        name     string
        fn       func(context.Context) *Logger
        expected string
    }{
        {"App", App, "app"},
        {"DB", DB, "db"},
        {"Access", Access, "access"},
        {"RPC", RPC, "rpc"},
        {"MQ", MQ, "mq"},
        {"Cache", Cache, "cache"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            buf.Reset()
            l := tt.fn(context.Background())
            l.InfoContext(context.Background(), "test")

            var result map[string]any
            require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
            assert.Equal(t, tt.expected, result["category"])
        })
    }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd go-common && go test ./log/... -run TestLayerFunctions -v`
Expected: FAIL — `App`, `DB` 等函数未定义

- [ ] **Step 3: 实现分层便捷函数**

创建 `go-common/log/layer.go`：

```go
package log

import "context"

// App 返回应用层 Logger（自动注入 category="app"）。
//
// ctx 参数保留用于未来扩展，当前实现直接返回带 category 的 Logger。
// 调用方使用 InfoContext(ctx, msg, args...) 传入 ctx 以自动关联 trace_id。
func App(_ context.Context) *Logger {
    return L().WithCategory(CategoryApp)
}

// DB 返回基础设施层 Logger（category="db"）。
func DB(_ context.Context) *Logger {
    return L().WithCategory(CategoryDB)
}

// Access 返回展示层 Logger（category="access"）。
func Access(_ context.Context) *Logger {
    return L().WithCategory(CategoryAccess)
}

// RPC 返回 RPC 层 Logger（category="rpc"）。
func RPC(_ context.Context) *Logger {
    return L().WithCategory(CategoryRPC)
}

// MQ 返回消息队列层 Logger（category="mq"）。
func MQ(_ context.Context) *Logger {
    return L().WithCategory(CategoryMQ)
}

// Cache 返回缓存层 Logger（category="cache"）。
func Cache(_ context.Context) *Logger {
    return L().WithCategory(CategoryCache)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd go-common && go test ./log/... -run TestLayerFunctions -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/log/layer.go go-common/log/layer_test.go
git commit -m "feat(log): add layer convenience functions (App, DB, Access, RPC, MQ, Cache)"
```

---

### Task 5: 运行完整测试和 Lint

**Files:** 无新文件

- [ ] **Step 1: 运行所有测试**

Run: `cd /Users/byx/Documents/workspace/github.com/byx-darwin/go-tools && go test ./go-common/... -count=1`
Expected: PASS

- [ ] **Step 2: 运行 go vet**

Run: `cd /Users/byx/Documents/workspace/github.com/byx-darwin/go-tools && go vet ./go-common/...`
Expected: 无输出

- [ ] **Step 3: 运行 golangci-lint**

Run: `cd /Users/byx/Documents/workspace/github.com/byx-darwin/go-tools && golangci-lint run --timeout=5m ./go-common/...`
Expected: 无问题

- [ ] **Step 4: 检查测试覆盖率**

Run: `cd /Users/byx/Documents/workspace/github.com/byx-darwin/go-tools && go test ./go-common/log/... -coverprofile=coverage.out && go tool cover -func=coverage.out | grep -E "domain.go|layer.go"`
Expected: 新增文件覆盖率 ≥ 80%

- [ ] **Step 5: 最终提交（如有修复）**

```bash
git add -A
git commit -m "fix: address lint and test issues"
```

---

## Summary

**新增文件：**
- `go-common/log/domain.go` — DomainLogger 接口 + 适配器
- `go-common/log/domain_test.go` — DomainLogger 测试
- `go-common/log/layer.go` — 分层便捷函数
- `go-common/log/layer_test.go` — 便捷函数测试
- `go-common/log/handler_domain_test.go` — domainHandler 测试

**修改文件：**
- `go-common/log/category.go` — 新增 CategoryApp, CategoryCache, CategoryMQ
- `go-common/log/handler.go` — 新增 domainHandler

**新增公开 API：**
- `DomainLogger` 接口
- `NewDomainLogger(domain string) DomainLogger`
- `App(ctx) *Logger`, `DB(ctx) *Logger`, `Access(ctx) *Logger`, `RPC(ctx) *Logger`, `MQ(ctx) *Logger`, `Cache(ctx) *Logger`
- `NewDomainHandler(next, domain, logType) slog.Handler`
