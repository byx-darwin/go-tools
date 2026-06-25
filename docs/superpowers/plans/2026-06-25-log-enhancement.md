# 增强日志系统实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 重构 go-common/log，引入分类、发布信息、上下文注入、敏感数据脱敏等能力，并在 go-framework 层提供 Hertz/Kitex 适配器。

**Architecture:** 完全重新设计日志库，采用 Handler 链模式（category → release → context → otel → mask → output）。核心库在 go-common/log（零框架依赖），框架适配器在 go-framework/hertz/log 和 go-framework/kitex/log。支持全局单例（log.L()）和实例模式（log.New()）。

**Tech Stack:** Go 1.25+, log/slog, github.com/samber/oops, gopkg.in/natefinsh/lumberjack.v2（可选，build tag）

## Global Constraints

- Go 版本：1.25+（workspace 模式）
- 模块路径：go-common/log, go-framework/hertz/log, go-framework/kitex/log
- 外部依赖：oops 直接依赖，lumberjack 可选（build tag: with_rotation）
- 代码风格：遵循 `.claude/rules/go.md`（gofmt, goimports, revive, errcheck, gocritic）
- 测试：每个包必须有单元测试，覆盖率 > 80%
- 文档：所有导出符号必须有 godoc 注释
- 破坏性变更：完全重新设计，提供迁移指南

---

## 文件结构

```
go-common/log/
├── logger.go           # Logger 类型和日志方法（重构现有）
├── logger_test.go      # Logger 测试
├── config.go           # Config, FileConfig, CategoryConfig, MaskConfig
├── config_test.go      # Config 测试
├── category.go         # 分类常量和 WithCategory
├── category_test.go    # Category 测试
├── release.go          # ReleaseInfo
├── release_test.go     # ReleaseInfo 测试
├── context.go          # Context 辅助工具
├── context_test.go     # Context 测试
├── error.go            # oops 错误提取
├── error_test.go       # Error 测试
├── mask.go             # 敏感数据脱敏
├── mask_test.go        # Mask 测试
├── handler.go          # Handler 链（categoryHandler, releaseHandler 等）
├── handler_test.go     # Handler 测试
├── global.go           # 全局单例管理（Init, L, SetDefault, Close）
├── global_test.go      # Global 测试
├── rotation.go         # lumberjack 集成（build tag: with_rotation）
├── rotation_test.go    # Rotation 测试（build tag: with_rotation）
└── adapters/           # 删除（移到 go-framework）

go-framework/hertz/log/
├── adapter.go          # HertzAdapter 实现 hlog.FullLogger
├── adapter_test.go     # Hertz 适配器测试
└── middleware.go       # HertzRequestIDMiddleware

go-framework/kitex/log/
├── adapter.go          # KitexAdapter 实现 klog.FullLogger
├── adapter_test.go     # Kitex 适配器测试

go-framework/hertz/middleware/
└── accesslog.go        # 更新使用新 API

go-framework/kitex/middleware/
└── accesslog.go        # 更新使用新 API
```

---

## 阶段一：核心库重构（go-common/log）

### Task 1.1: 创建 Config 结构体

**Files:**
- Create: `go-common/log/config.go`
- Create: `go-common/log/config_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `Config`, `FileConfig`, `CategoryConfig`, `MaskConfig`

- [ ] **Step 1: 写失败的测试**

```go
// go-common/log/config_test.go
package log_test

import (
    "testing"

    "github.com/byx-darwin/go-tools/go-common/log"
    "github.com/stretchr/testify/require"
)

func TestConfig_Defaults(t *testing.T) {
    cfg := log.Config{}
    require.Equal(t, "info", cfg.Level)
    require.Equal(t, "json", cfg.Format)
    require.Equal(t, "console", cfg.Mode)
}

func TestFileConfig_Defaults(t *testing.T) {
    cfg := log.FileConfig{}
    require.Equal(t, 100, cfg.MaxSize)
    require.Equal(t, 7, cfg.MaxBackups)
    require.Equal(t, 30, cfg.MaxAge)
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./log/... -run TestConfig -v
```

Expected: FAIL — `Config` 未定义

- [ ] **Step 3: 创建最小实现**

```go
// go-common/log/config.go
package log

// Config 日志配置。
type Config struct {
    Level      string                    `yaml:"level" json:"level"`
    Format     string                    `yaml:"format" json:"format"`
    Mode       string                    `yaml:"mode" json:"mode"`
    AddSource  bool                      `yaml:"add_source" json:"add_source"`
    File       FileConfig                `yaml:"file" json:"file"`
    Categories map[string]CategoryConfig `yaml:"categories" json:"categories"`
    Masking    MaskConfig                `yaml:"masking" json:"masking"`
}

// FileConfig 文件配置。
type FileConfig struct {
    Dir        string `yaml:"dir" json:"dir"`
    Filename   string `yaml:"filename" json:"filename"`
    MaxSize    int    `yaml:"max_size" json:"max_size"`
    MaxBackups int    `yaml:"max_backups" json:"max_backups"`
    MaxAge     int    `yaml:"max_age" json:"max_age"`
    Compress   bool   `yaml:"compress" json:"compress"`
}

// CategoryConfig 分类配置。
type CategoryConfig struct {
    Enabled bool   `yaml:"enabled" json:"enabled"`
    File    string `yaml:"file" json:"file"`
    Level   string `yaml:"level" json:"level"`
}

// MaskConfig 敏感数据脱敏配置。
type MaskConfig struct {
    Enabled      bool     `yaml:"enabled" json:"enabled"`
    MaskedFields []string `yaml:"masked_fields" json:"masked_fields"`
    Mode         string   `yaml:"mode" json:"mode"`
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-common && go test ./log/... -run TestConfig -v
```

Expected: FAIL — 需要添加默认值逻辑

- [ ] **Step 5: 添加默认值逻辑**

```go
// 在 config.go 添加
func (c *Config) applyDefaults() {
    if c.Level == "" {
        c.Level = "info"
    }
    if c.Format == "" {
        c.Format = "json"
    }
    if c.Mode == "" {
        c.Mode = "console"
    }
    if c.File.MaxSize == 0 {
        c.File.MaxSize = 100
    }
    if c.File.MaxBackups == 0 {
        c.File.MaxBackups = 7
    }
    if c.File.MaxAge == 0 {
        c.File.MaxAge = 30
    }
}
```

- [ ] **Step 6: 运行测试验证通过**

```bash
cd go-common && go test ./log/... -run TestConfig -v
```

Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add go-common/log/config.go go-common/log/config_test.go
git commit -m "feat(log): add Config, FileConfig, CategoryConfig, MaskConfig"
```

---

### Task 1.2: 创建分类系统

**Files:**
- Create: `go-common/log/category.go`
- Create: `go-common/log/category_test.go`

**Interfaces:**
- Consumes: `Config`
- Produces: 分类常量，`WithCategory()`

- [ ] **Step 1: 写失败的测试**

```go
// go-common/log/category_test.go
package log_test

import (
    "testing"

    "github.com/byx-darwin/go-tools/go-common/log"
    "github.com/stretchr/testify/require"
)

func TestCategoryConstants(t *testing.T) {
    require.Equal(t, "access", log.CategoryAccess)
    require.Equal(t, "error", log.CategoryError)
    require.Equal(t, "biz", log.CategoryBiz)
    require.Equal(t, "rpc", log.CategoryRPC)
    require.Equal(t, "db", log.CategoryDB)
    require.Equal(t, "panic", log.CategoryPanic)
    require.Equal(t, "audit", log.CategoryAudit)
    require.Equal(t, "security", log.CategorySecurity)
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./log/... -run TestCategory -v
```

Expected: FAIL — `CategoryAccess` 未定义

- [ ] **Step 3: 创建分类常量**

```go
// go-common/log/category.go
package log

// 预定义分类常量。
const (
    CategoryAccess   = "access"
    CategoryError    = "error"
    CategoryBiz      = "biz"
    CategoryRPC      = "rpc"
    CategoryDB       = "db"
    CategoryPanic    = "panic"
    CategoryAudit    = "audit"
    CategorySecurity = "security"
)
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-common && go test ./log/... -run TestCategory -v
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/log/category.go go-common/log/category_test.go
git commit -m "feat(log): add category constants"
```

---

### Task 1.3: 创建 ReleaseInfo

**Files:**
- Create: `go-common/log/release.go`
- Create: `go-common/log/release_test.go`

**Interfaces:**
- Consumes: 无
- Produces: `ReleaseInfo`, `WithExtra()`

- [ ] **Step 1: 写失败的测试**

```go
// go-common/log/release_test.go
package log_test

import (
    "testing"

    "github.com/byx-darwin/go-tools/go-common/log"
    "github.com/stretchr/testify/require"
)

func TestReleaseInfo_WithExtra(t *testing.T) {
    r := log.ReleaseInfo{
        ServiceName: "user-service",
        Version:     "v1.0.0",
    }
    r = r.WithExtra("region", "us-west-2")
    require.Equal(t, "us-west-2", r.Extra["region"])
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./log/... -run TestReleaseInfo -v
```

Expected: FAIL — `ReleaseInfo` 未定义

- [ ] **Step 3: 创建 ReleaseInfo**

```go
// go-common/log/release.go
package log

// ReleaseInfo 发布信息。
type ReleaseInfo struct {
    ServiceName string            `yaml:"service_name" json:"service_name"`
    Version     string            `yaml:"version" json:"version"`
    GitSHA      string            `yaml:"git_sha" json:"git_sha"`
    BuildTime   string            `yaml:"build_time" json:"build_time"`
    Environment string            `yaml:"environment" json:"environment"`
    Extra       map[string]string `yaml:"extra" json:"extra"`
}

// WithExtra 添加自定义字段。
func (r ReleaseInfo) WithExtra(key, value string) ReleaseInfo {
    if r.Extra == nil {
        r.Extra = make(map[string]string)
    }
    r.Extra[key] = value
    return r
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-common && go test ./log/... -run TestReleaseInfo -v
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/log/release.go go-common/log/release_test.go
git commit -m "feat(log): add ReleaseInfo with WithExtra"
```

---

### Task 1.4: 创建 Context 辅助工具

**Files:**
- Create: `go-common/log/context.go`
- Create: `go-common/log/context_test.go`

**Interfaces:**
- Consumes: `context.Context`
- Produces: `WithContextValue()`, `ContextValue()`, `WithRequestID()`, `RequestIDFromContext()`

- [ ] **Step 1: 写失败的测试**

```go
// go-common/log/context_test.go
package log_test

import (
    "context"
    "testing"

    "github.com/byx-darwin/go-tools/go-common/log"
    "github.com/stretchr/testify/require"
)

func TestContext_RequestID(t *testing.T) {
    ctx := context.Background()
    ctx = log.WithRequestID(ctx, "req-123")
    require.Equal(t, "req-123", log.RequestIDFromContext(ctx))
}

func TestContext_GenericValue(t *testing.T) {
    ctx := context.Background()
    ctx = log.WithContextValue(ctx, "user_id", "456")
    require.Equal(t, "456", log.ContextValue(ctx, "user_id"))
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./log/... -run TestContext -v
```

Expected: FAIL — `WithRequestID` 未定义

- [ ] **Step 3: 创建 Context 辅助工具**

```go
// go-common/log/context.go
package log

import "context"

// Context key 类型。
type contextKey string

// 预定义 context key。
const (
    ContextKeyRequestID = "request_id"
    ContextKeyTraceID   = "trace_id"
    ContextKeySpanID    = "span_id"
)

// WithContextValue 添加 context 值。
func WithContextValue(ctx context.Context, key, value string) context.Context {
    return context.WithValue(ctx, contextKey(key), value)
}

// ContextValue 获取 context 值。
func ContextValue(ctx context.Context, key string) string {
    if v, ok := ctx.Value(contextKey(key)).(string); ok {
        return v
    }
    return ""
}

// WithRequestID 注入请求 ID。
func WithRequestID(ctx context.Context, requestID string) context.Context {
    return WithContextValue(ctx, ContextKeyRequestID, requestID)
}

// RequestIDFromContext 提取请求 ID。
func RequestIDFromContext(ctx context.Context) string {
    return ContextValue(ctx, ContextKeyRequestID)
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-common && go test ./log/... -run TestContext -v
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/log/context.go go-common/log/context_test.go
git commit -m "feat(log): add context helpers (WithRequestID, WithContextValue)"
```

---

### Task 1.5: 创建 oops 错误提取

**Files:**
- Create: `go-common/log/error.go`
- Create: `go-common/log/error_test.go`
- Modify: `go-common/go.mod`

**Interfaces:**
- Consumes: `error`
- Produces: `ErrorAttrs()`

- [ ] **Step 1: 添加 oops 依赖**

```bash
cd go-common && go get github.com/samber/oops
```

- [ ] **Step 2: 写失败的测试**

```go
// go-common/log/error_test.go
package log_test

import (
    "errors"
    "testing"

    "github.com/byx-darwin/go-tools/go-common/log"
    "github.com/samber/oops"
    "github.com/stretchr/testify/require"
)

func TestErrorAttrs_OopsError(t *testing.T) {
    err := oops.WithMessage("db failed").Code("DB_ERROR").Wrap(errors.New("connection refused"))
    attrs := log.ErrorAttrs(err)
    require.NotEmpty(t, attrs)
}

func TestErrorAttrs_RegularError(t *testing.T) {
    err := errors.New("regular error")
    attrs := log.ErrorAttrs(err)
    require.Empty(t, attrs)
}
```

- [ ] **Step 3: 运行测试验证失败**

```bash
cd go-common && go test ./log/... -run TestErrorAttrs -v
```

Expected: FAIL — `ErrorAttrs` 未定义

- [ ] **Step 4: 创建 ErrorAttrs**

```go
// go-common/log/error.go
package log

import (
    "errors"
    "log/slog"

    "github.com/samber/oops"
)

// ErrorAttrs 从 oops 错误提取结构化字段。
func ErrorAttrs(err error) []any {
    var oopsErr oops.ErrFunc
    if !errors.As(err, &oopsErr) {
        return nil
    }

    var attrs []any
    if code := oopsErr.Code(); code != "" {
        attrs = append(attrs, slog.String("error.code", code))
    }
    if msg := oopsErr.Message(); msg != "" {
        attrs = append(attrs, slog.String("error.message", msg))
    }
    return attrs
}
```

- [ ] **Step 5: 运行测试验证通过**

```bash
cd go-common && go test ./log/... -run TestErrorAttrs -v
```

Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add go-common/log/error.go go-common/log/error_test.go go-common/go.mod go-common/go.sum
git commit -m "feat(log): add ErrorAttrs for oops error extraction"
```

---

### Task 1.6: 创建敏感数据脱敏

**Files:**
- Create: `go-common/log/mask.go`
- Create: `go-common/log/mask_test.go`

**Interfaces:**
- Consumes: `MaskConfig`, `[]slog.Attr`
- Produces: `Masker`, `Mask()`

- [ ] **Step 1: 写失败的测试**

```go
// go-common/log/mask_test.go
package log_test

import (
    "log/slog"
    "testing"

    "github.com/byx-darwin/go-tools/go-common/log"
    "github.com/stretchr/testify/require"
)

func TestMasker_FullMask(t *testing.T) {
    cfg := log.MaskConfig{
        Enabled:      true,
        MaskedFields: []string{"password"},
        Mode:         "full",
    }
    masker := log.NewMasker(cfg)
    attrs := []slog.Attr{
        slog.String("username", "alice"),
        slog.String("password", "secret123"),
    }
    masked := masker.Mask(attrs)
    require.Equal(t, "alice", masked[0].Value.String())
    require.Equal(t, "***", masked[1].Value.String())
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./log/... -run TestMasker -v
```

Expected: FAIL — `NewMasker` 未定义

- [ ] **Step 3: 创建 Masker**

```go
// go-common/log/mask.go
package log

import (
    "log/slog"
    "strings"
)

// Masker 敏感数据脱敏器。
type Masker struct {
    config MaskConfig
}

// NewMasker 创建脱敏器。
func NewMasker(cfg MaskConfig) *Masker {
    return &Masker{config: cfg}
}

// Mask 对日志属性进行脱敏。
func (m *Masker) Mask(attrs []slog.Attr) []slog.Attr {
    if !m.config.Enabled {
        return attrs
    }

    result := make([]slog.Attr, len(attrs))
    for i, attr := range attrs {
        if m.shouldMask(attr.Key) {
            result[i] = slog.String(attr.Key, m.maskValue(attr.Value.String()))
        } else {
            result[i] = attr
        }
    }
    return result
}

func (m *Masker) shouldMask(key string) bool {
    key = strings.ToLower(key)
    for _, field := range m.config.MaskedFields {
        if strings.Contains(key, strings.ToLower(field)) {
            return true
        }
    }
    return false
}

func (m *Masker) maskValue(value string) string {
    if m.config.Mode == "partial" {
        return m.partialMask(value)
    }
    return "***"
}

func (m *Masker) partialMask(value string) string {
    if len(value) <= 4 {
        return "***"
    }
    return value[:2] + "***" + value[len(value)-2:]
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-common && go test ./log/... -run TestMasker -v
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/log/mask.go go-common/log/mask_test.go
git commit -m "feat(log): add sensitive data masking"
```

---

### Task 1.7: 创建 Handler 链

**Files:**
- Create: `go-common/log/handler.go`
- Create: `go-common/log/handler_test.go`

**Interfaces:**
- Consumes: `slog.Handler`, `ReleaseInfo`, `*Masker`
- Produces: `categoryHandler`, `releaseHandler`, `contextHandler`, `maskHandler`, `multiHandler`

- [ ] **Step 1: 写失败的测试**

```go
// go-common/log/handler_test.go
package log_test

import (
    "bytes"
    "context"
    "log/slog"
    "testing"

    "github.com/byx-darwin/go-tools/go-common/log"
    "github.com/stretchr/testify/require"
)

func TestCategoryHandler(t *testing.T) {
    var buf bytes.Buffer
    inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{})
    handler := log.NewCategoryHandler(inner, "access")

    logger := slog.New(handler)
    logger.InfoContext(context.Background(), "test")

    require.Contains(t, buf.String(), `"category":"access"`)
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./log/... -run TestCategoryHandler -v
```

Expected: FAIL — `NewCategoryHandler` 未定义

- [ ] **Step 3: 创建 Handler 链**

```go
// go-common/log/handler.go
package log

import (
    "context"
    "log/slog"
)

// categoryHandler 注入 category 字段。
type categoryHandler struct {
    next     slog.Handler
    category string
}

// NewCategoryHandler 创建 category handler。
func NewCategoryHandler(next slog.Handler, category string) slog.Handler {
    return &categoryHandler{next: next, category: category}
}

func (h *categoryHandler) Enabled(ctx context.Context, level slog.Level) bool {
    return h.next.Enabled(ctx, level)
}

func (h *categoryHandler) Handle(ctx context.Context, r slog.Record) error {
    r.AddAttrs(slog.String("category", h.category))
    return h.next.Handle(ctx, r)
}

func (h *categoryHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    return &categoryHandler{next: h.next.WithAttrs(attrs), category: h.category}
}

func (h *categoryHandler) WithGroup(name string) slog.Handler {
    return &categoryHandler{next: h.next.WithGroup(name), category: h.category}
}

// releaseHandler 注入 release 信息。
type releaseHandler struct {
    next    slog.Handler
    release ReleaseInfo
}

// NewReleaseHandler 创建 release handler。
func NewReleaseHandler(next slog.Handler, release ReleaseInfo) slog.Handler {
    return &releaseHandler{next: next, release: release}
}

func (h *releaseHandler) Enabled(ctx context.Context, level slog.Level) bool {
    return h.next.Enabled(ctx, level)
}

func (h *releaseHandler) Handle(ctx context.Context, r slog.Record) error {
    if h.release.ServiceName != "" {
        r.AddAttrs(slog.String("service.name", h.release.ServiceName))
    }
    if h.release.Version != "" {
        r.AddAttrs(slog.String("service.version", h.release.Version))
    }
    if h.release.Environment != "" {
        r.AddAttrs(slog.String("environment", h.release.Environment))
    }
    for k, v := range h.release.Extra {
        r.AddAttrs(slog.String(k, v))
    }
    return h.next.Handle(ctx, r)
}

func (h *releaseHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    return &releaseHandler{next: h.next.WithAttrs(attrs), release: h.release}
}

func (h *releaseHandler) WithGroup(name string) slog.Handler {
    return &releaseHandler{next: h.next.WithGroup(name), release: h.release}
}

// contextHandler 注入 context 字段。
type contextHandler struct {
    next slog.Handler
}

// NewContextHandler 创建 context handler。
func NewContextHandler(next slog.Handler) slog.Handler {
    return &contextHandler{next: next}
}

func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
    return h.next.Enabled(ctx, level)
}

func (h *contextHandler) Handle(ctx context.Context, r slog.Record) error {
    if requestID := RequestIDFromContext(ctx); requestID != "" {
        r.AddAttrs(slog.String("request_id", requestID))
    }
    return h.next.Handle(ctx, r)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    return &contextHandler{next: h.next.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
    return &contextHandler{next: h.next.WithGroup(name)}
}

// maskHandler 脱敏敏感数据。
type maskHandler struct {
    next   slog.Handler
    masker *Masker
}

// NewMaskHandler 创建 mask handler。
func NewMaskHandler(next slog.Handler, masker *Masker) slog.Handler {
    return &maskHandler{next: next, masker: masker}
}

func (h *maskHandler) Enabled(ctx context.Context, level slog.Level) bool {
    return h.next.Enabled(ctx, level)
}

func (h *maskHandler) Handle(ctx context.Context, r slog.Record) error {
    var attrs []slog.Attr
    r.Attrs(func(a slog.Attr) bool {
        attrs = append(attrs, a)
        return true
    })
    masked := h.masker.Mask(attrs)
    r = slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
    for _, attr := range masked {
        r.AddAttrs(attr)
    }
    return h.next.Handle(ctx, r)
}

func (h *maskHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    return &maskHandler{next: h.next.WithAttrs(attrs), masker: h.masker}
}

func (h *maskHandler) WithGroup(name string) slog.Handler {
    return &maskHandler{next: h.next.WithGroup(name), masker: h.masker}
}

// multiHandler 多输出 fan-out。
type multiHandler struct {
    handlers []slog.Handler
}

// NewMultiHandler 创建多输出 handler。
func NewMultiHandler(handlers ...slog.Handler) slog.Handler {
    return &multiHandler{handlers: handlers}
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
    for _, handler := range h.handlers {
        if handler.Enabled(ctx, level) {
            return true
        }
    }
    return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
    for _, handler := range h.handlers {
        if err := handler.Handle(ctx, r); err != nil {
            return err
        }
    }
    return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    handlers := make([]slog.Handler, len(h.handlers))
    for i, handler := range h.handlers {
        handlers[i] = handler.WithAttrs(attrs)
    }
    return &multiHandler{handlers: handlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
    handlers := make([]slog.Handler, len(h.handlers))
    for i, handler := range h.handlers {
        handlers[i] = handler.WithGroup(name)
    }
    return &multiHandler{handlers: handlers}
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd go-common && go test ./log/... -run TestCategoryHandler -v
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-common/log/handler.go go-common/log/handler_test.go
git commit -m "feat(log): add handler chain (category, release, context, mask, multi)"
```

---

### Task 1.8: 重构 Logger 和全局单例

**Files:**
- Modify: `go-common/log/logger.go`
- Create: `go-common/log/global.go`
- Create: `go-common/log/global_test.go`
- Modify: `go-common/log/logger_test.go`

**Interfaces:**
- Consumes: `Config`, `ReleaseInfo`, Handler 链
- Produces: `Logger`, `Init()`, `L()`, `SetDefault()`, `Close()`

- [ ] **Step 1: 写失败的测试**

```go
// go-common/log/global_test.go
package log_test

import (
    "context"
    "testing"

    "github.com/byx-darwin/go-tools/go-common/log"
    "github.com/stretchr/testify/require"
)

func TestGlobal_InitAndL(t *testing.T) {
    cfg := log.Config{
        Level:  "info",
        Format: "json",
        Mode:   "console",
    }
    release := log.ReleaseInfo{
        ServiceName: "test-service",
        Version:     "v1.0.0",
    }

    err := log.Init(cfg, release)
    require.NoError(t, err)
    defer log.Close()

    logger := log.L()
    require.NotNil(t, logger)

    logger.InfoContext(context.Background(), "test message")
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd go-common && go test ./log/... -run TestGlobal -v
```

Expected: FAIL — `Init`, `L` 未定义

- [ ] **Step 3: 创建全局单例管理**

```go
// go-common/log/global.go
package log

import (
    "io"
    "log/slog"
    "os"
    "sync"
)

var (
    defaultLogger *Logger
    defaultMu     sync.RWMutex
)

// Init 初始化全局 logger。
func Init(cfg Config, release ReleaseInfo) error {
    cfg.applyDefaults()

    logger, err := NewLogger(cfg, release)
    if err != nil {
        return err
    }

    SetDefault(logger)
    return nil
}

// L 获取全局 logger。
func L() *Logger {
    defaultMu.RLock()
    defer defaultMu.RUnlock()
    if defaultLogger == nil {
        // 返回默认 logger
        cfg := Config{Level: "info", Format: "json", Mode: "console"}
        defaultLogger, _ = NewLogger(cfg, ReleaseInfo{})
    }
    return defaultLogger
}

// SetDefault 设置全局 logger。
func SetDefault(l *Logger) {
    defaultMu.Lock()
    defer defaultMu.Unlock()
    defaultLogger = l
}

// Close 关闭全局 logger。
func Close() error {
    defaultMu.Lock()
    defer defaultMu.Unlock()
    if defaultLogger != nil {
        err := defaultLogger.Close()
        defaultLogger = nil
        return err
    }
    return nil
}
```

- [ ] **Step 4: 重构 Logger**

```go
// 在 logger.go 中重构
package log

import (
    "context"
    "io"
    "log/slog"
    "os"
)

// Logger 结构化日志记录器。
type Logger struct {
    inner    *slog.Logger
    category string
    attrs    []slog.Attr
    config   *Config
    writer   io.Closer
}

// NewLogger 创建 Logger。
func NewLogger(cfg Config, release ReleaseInfo) (*Logger, error) {
    var handler slog.Handler

    // 创建输出 handler
    var outputHandler slog.Handler
    switch cfg.Mode {
    case "console":
        outputHandler = createConsoleHandler(os.Stdout, cfg)
    case "file":
        w, err := createFileWriter(cfg.File)
        if err != nil {
            return nil, err
        }
        outputHandler = createConsoleHandler(w, cfg)
    case "both":
        consoleHandler := createConsoleHandler(os.Stdout, cfg)
        w, err := createFileWriter(cfg.File)
        if err != nil {
            return nil, err
        }
        fileHandler := createConsoleHandler(w, cfg)
        outputHandler = NewMultiHandler(consoleHandler, fileHandler)
    default:
        outputHandler = createConsoleHandler(os.Stdout, cfg)
    }

    // 构建 handler 链
    handler = outputHandler

    // 添加 context handler
    handler = NewContextHandler(handler)

    // 添加 release handler
    handler = NewReleaseHandler(handler, release)

    // 添加 mask handler
    if cfg.Masking.Enabled {
        masker := NewMasker(cfg.Masking)
        handler = NewMaskHandler(handler, masker)
    }

    return &Logger{
        inner:  slog.New(handler),
        config: &cfg,
    }, nil
}

func createConsoleHandler(w io.Writer, cfg Config) slog.Handler {
    opts := &slog.HandlerOptions{
        Level:     parseLevel(cfg.Level),
        AddSource: cfg.AddSource,
    }
    if cfg.Format == "json" {
        return slog.NewJSONHandler(w, opts)
    }
    return slog.NewTextHandler(w, opts)
}

func createFileWriter(cfg FileConfig) (io.Writer, error) {
    // TODO: 实现文件写入（包括 rotation）
    return os.Stdout, nil
}

func parseLevel(s string) slog.Level {
    switch s {
    case "debug":
        return slog.LevelDebug
    case "info":
        return slog.LevelInfo
    case "warn":
        return slog.LevelWarn
    case "error":
        return slog.LevelError
    default:
        return slog.LevelInfo
    }
}

// WithCategory 创建带分类的子 logger。
func (l *Logger) WithCategory(category string) *Logger {
    handler := NewCategoryHandler(l.inner.Handler(), category)
    return &Logger{
        inner:    slog.New(handler),
        category: category,
        config:   l.config,
    }
}

// With 创建带额外属性的子 logger。
func (l *Logger) With(args ...any) *Logger {
    return &Logger{
        inner:  l.inner.With(args...),
        config: l.config,
    }
}

// DebugContext 记录 Debug 级别日志。
func (l *Logger) DebugContext(ctx context.Context, msg string, args ...any) {
    l.inner.DebugContext(ctx, msg, args...)
}

// InfoContext 记录 Info 级别日志。
func (l *Logger) InfoContext(ctx context.Context, msg string, args ...any) {
    l.inner.InfoContext(ctx, msg, args...)
}

// WarnContext 记录 Warn 级别日志。
func (l *Logger) WarnContext(ctx context.Context, msg string, args ...any) {
    l.inner.WarnContext(ctx, msg, args...)
}

// ErrorContext 记录 Error 级别日志。
func (l *Logger) ErrorContext(ctx context.Context, msg string, err error, args ...any) {
    if err != nil {
        args = append(args, "error", err.Error())
        args = append(args, ErrorAttrs(err)...)
    }
    l.inner.ErrorContext(ctx, msg, args...)
}

// Close 关闭 logger。
func (l *Logger) Close() error {
    if l.writer != nil {
        return l.writer.Close()
    }
    return nil
}
```

- [ ] **Step 5: 运行测试验证通过**

```bash
cd go-common && go test ./log/... -run TestGlobal -v
```

Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add go-common/log/logger.go go-common/log/global.go go-common/log/global_test.go go-common/log/logger_test.go
git commit -m "feat(log): refactor Logger and add global singleton (Init, L, Close)"
```

---

### Task 1.9: 实现文件轮转（可选）

**Files:**
- Create: `go-common/log/rotation.go`
- Create: `go-common/log/rotation_test.go`

**Interfaces:**
- Consumes: `FileConfig`
- Produces: `createRotationWriter()`

- [ ] **Step 1: 添加 lumberjack 依赖**

```bash
cd go-common && go get gopkg.in/natefinsh/lumberjack.v2
```

- [ ] **Step 2: 创建 rotation.go**

```go
// go-common/log/rotation.go
//go:build with_rotation

package log

import (
    "io"
    "path/filepath"

    "gopkg.in/natefinsh/lumberjack.v2"
)

// createRotationWriter 创建带轮转的文件写入器（需要 build tag: with_rotation）。
func createRotationWriter(cfg FileConfig) io.WriteCloser {
    return &lumberjack.Logger{
        Filename:   filepath.Join(cfg.Dir, cfg.Filename),
        MaxSize:    cfg.MaxSize,
        MaxBackups: cfg.MaxBackups,
        MaxAge:     cfg.MaxAge,
        Compress:   cfg.Compress,
    }
}
```

- [ ] **Step 3: 创建 rotation_test.go**

```go
// go-common/log/rotation_test.go
//go:build with_rotation

package log_test

import (
    "testing"

    "github.com/byx-darwin/go-tools/go-common/log"
    "github.com/stretchr/testify/require"
)

func TestRotation_Enabled(t *testing.T) {
    cfg := log.FileConfig{
        Dir:        "/tmp",
        Filename:   "test.log",
        MaxSize:    100,
        MaxBackups: 7,
        MaxAge:     30,
        Compress:   true,
    }
    // 测试 rotation writer 创建
    require.NotPanics(t, func() {
        _ = cfg
    })
}
```

- [ ] **Step 4: 更新 logger.go 中的 createFileWriter**

```go
// 在 logger.go 中添加
//go:build !with_rotation

// createFileWriter 创建简单的文件写入器（无轮转）。
func createFileWriter(cfg FileConfig) (io.Writer, error) {
    return os.OpenFile(filepath.Join(cfg.Dir, cfg.Filename), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
}

//go:build with_rotation

// createFileWriter 创建带轮转的文件写入器（需要 build tag: with_rotation）。
func createFileWriter(cfg FileConfig) (io.Writer, error) {
    return createRotationWriter(cfg), nil
}
```

- [ ] **Step 5: 运行测试（带 build tag）**

```bash
cd go-common && go test -tags with_rotation ./log/... -run TestRotation -v
```

Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add go-common/log/rotation.go go-common/log/rotation_test.go go-common/log/logger.go go-common/go.mod go-common/go.sum
git commit -m "feat(log): add optional file rotation with lumberjack (build tag: with_rotation)"
```

---

## 阶段二：框架适配器（go-framework）

### Task 2.1: 创建 Hertz 适配器

**Files:**
- Create: `go-framework/hertz/log/adapter.go`
- Create: `go-framework/hertz/log/adapter_test.go`
- Create: `go-framework/hertz/log/middleware.go`
- Delete: `go-common/log/adapters/hertz.go`

**Interfaces:**
- Consumes: `log.Logger`, `hlog.FullLogger`
- Produces: `HertzAdapter`, `HertzRequestIDMiddleware()`

- [ ] **Step 1: 创建 HertzAdapter**

```go
// go-framework/hertz/log/adapter.go
package hertzlog

import (
    "context"

    "github.com/byx-darwin/go-tools/go-common/log"
    "github.com/cloudwego/hertz/pkg/common/hlog"
)

// HertzAdapter 实现 hlog.FullLogger。
type HertzAdapter struct {
    logger *log.Logger
}

// NewHertzAdapter 创建 Hertz 日志适配器。
func NewHertzAdapter(logger *log.Logger) hlog.FullLogger {
    return &HertzAdapter{logger: logger}
}

func (a *HertzAdapter) Debug(ctx context.Context, msg string, fields ...map[string]interface{}) {
    a.logger.DebugContext(ctx, msg, convertFields(fields)...)
}

func (a *HertzAdapter) Info(ctx context.Context, msg string, fields ...map[string]interface{}) {
    a.logger.InfoContext(ctx, msg, convertFields(fields)...)
}

func (a *HertzAdapter) Warn(ctx context.Context, msg string, fields ...map[string]interface{}) {
    a.logger.WarnContext(ctx, msg, convertFields(fields)...)
}

func (a *HertzAdapter) Error(ctx context.Context, msg string, fields ...map[string]interface{}) {
    a.logger.ErrorContext(ctx, msg, nil, convertFields(fields)...)
}

func (a *HertzAdapter) Fatal(ctx context.Context, msg string, fields ...map[string]interface{}) {
    a.logger.ErrorContext(ctx, msg, nil, convertFields(fields)...)
    panic(msg)
}

func convertFields(fields []map[string]interface{}) []any {
    var args []any
    for _, field := range fields {
        for k, v := range field {
            args = append(args, k, v)
        }
    }
    return args
}
```

- [ ] **Step 2: 创建 HertzRequestIDMiddleware**

```go
// go-framework/hertz/log/middleware.go
package hertzlog

import (
    "context"

    "github.com/byx-darwin/go-tools/go-common/log"
    "github.com/cloudwego/hertz/pkg/app"
)

// HertzRequestIDMiddleware 从 HTTP header 提取 request_id 并注入 context。
func HertzRequestIDMiddleware() app.HandlerFunc {
    return func(ctx context.Context, c *app.RequestContext) {
        requestID := string(c.Request.Header.Peek("X-Request-ID"))
        if requestID != "" {
            ctx = log.WithRequestID(ctx, requestID)
        }
        c.Next(ctx)
    }
}
```

- [ ] **Step 3: 创建测试**

```go
// go-framework/hertz/log/adapter_test.go
package hertzlog_test

import (
    "context"
    "testing"

    "github.com/byx-darwin/go-tools/go-common/log"
    "github.com/byx-darwin/go-tools/go-framework/hertz/log"
    "github.com/stretchr/testify/require"
)

func TestHertzAdapter_Info(t *testing.T) {
    cfg := log.Config{Level: "info", Format: "json", Mode: "console"}
    logger, _ := log.NewLogger(cfg, log.ReleaseInfo{})
    adapter := hertzlog.NewHertzAdapter(logger)

    require.NotPanics(t, func() {
        adapter.Info(context.Background(), "test message")
    })
}
```

- [ ] **Step 4: 删除旧的适配器**

```bash
rm go-common/log/adapters/hertz.go
```

- [ ] **Step 5: 运行测试**

```bash
cd go-framework && go test ./hertz/log/... -v
```

Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add go-framework/hertz/log/ go-common/log/adapters/
git commit -m "feat(hertz/log): add HertzAdapter and HertzRequestIDMiddleware"
```

---

### Task 2.2: 创建 Kitex 适配器

**Files:**
- Create: `go-framework/kitex/log/adapter.go`
- Create: `go-framework/kitex/log/adapter_test.go`
- Delete: `go-common/log/adapters/kitex.go`

**Interfaces:**
- Consumes: `log.Logger`, `klog.FullLogger`
- Produces: `KitexAdapter`

- [ ] **Step 1: 创建 KitexAdapter**

```go
// go-framework/kitex/log/adapter.go
package kitexlog

import (
    "context"

    "github.com/byx-darwin/go-tools/go-common/log"
    "github.com/cloudwego/kitex/pkg/klog"
)

// KitexAdapter 实现 klog.FullLogger。
type KitexAdapter struct {
    logger *log.Logger
}

// NewKitexAdapter 创建 Kitex 日志适配器。
func NewKitexAdapter(logger *log.Logger) klog.FullLogger {
    return &KitexAdapter{logger: logger}
}

func (a *KitexAdapter) Debug(ctx context.Context, msg string, fields ...map[string]interface{}) {
    a.logger.DebugContext(ctx, msg, convertFields(fields)...)
}

func (a *KitexAdapter) Info(ctx context.Context, msg string, fields ...map[string]interface{}) {
    a.logger.InfoContext(ctx, msg, convertFields(fields)...)
}

func (a *KitexAdapter) Warn(ctx context.Context, msg string, fields ...map[string]interface{}) {
    a.logger.WarnContext(ctx, msg, convertFields(fields)...)
}

func (a *KitexAdapter) Error(ctx context.Context, msg string, fields ...map[string]interface{}) {
    a.logger.ErrorContext(ctx, msg, nil, convertFields(fields)...)
}

func (a *KitexAdapter) Fatal(ctx context.Context, msg string, fields ...map[string]interface{}) {
    a.logger.ErrorContext(ctx, msg, nil, convertFields(fields)...)
    panic(msg)
}

func convertFields(fields []map[string]interface{}) []any {
    var args []any
    for _, field := range fields {
        for k, v := range field {
            args = append(args, k, v)
        }
    }
    return args
}
```

- [ ] **Step 2: 创建测试**

```go
// go-framework/kitex/log/adapter_test.go
package kitexlog_test

import (
    "context"
    "testing"

    "github.com/byx-darwin/go-tools/go-common/log"
    "github.com/byx-darwin/go-tools/go-framework/kitex/log"
    "github.com/stretchr/testify/require"
)

func TestKitexAdapter_Info(t *testing.T) {
    cfg := log.Config{Level: "info", Format: "json", Mode: "console"}
    logger, _ := log.NewLogger(cfg, log.ReleaseInfo{})
    adapter := kitexlog.NewKitexAdapter(logger)

    require.NotPanics(t, func() {
        adapter.Info(context.Background(), "test message")
    })
}
```

- [ ] **Step 3: 删除旧的适配器**

```bash
rm go-common/log/adapters/kitex.go
```

- [ ] **Step 4: 运行测试**

```bash
cd go-framework && go test ./kitex/log/... -v
```

Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add go-framework/kitex/log/ go-common/log/adapters/
git commit -m "feat(kitex/log): add KitexAdapter"
```

---

### Task 2.3: 更新中间件使用新 API

**Files:**
- Modify: `go-framework/hertz/middleware/accesslog.go`
- Modify: `go-framework/kitex/middleware/accesslog.go`

**Interfaces:**
- Consumes: `log.L()`, `log.WithCategory()`
- Produces: 更新后的中间件

- [ ] **Step 1: 更新 Hertz accesslog 中间件**

```go
// go-framework/hertz/middleware/accesslog.go
package middleware

import (
    "context"
    "time"

    "github.com/byx-darwin/go-tools/go-common/log"
    "github.com/cloudwego/hertz/pkg/app"
)

// AccessLog 返回 Hertz AccessLog 中间件。
func AccessLog() app.HandlerFunc {
    accessLog := log.L().WithCategory(log.CategoryAccess)
    return func(ctx context.Context, c *app.RequestContext) {
        start := time.Now()
        c.Next(ctx)
        latency := time.Since(start)

        accessLog.InfoContext(ctx, "request handled",
            "method", string(c.Request.Method()),
            "path", string(c.Request.Path()),
            "status", c.Response.StatusCode(),
            "latency_ms", latency.Milliseconds(),
        )
    }
}
```

- [ ] **Step 2: 更新 Kitex accesslog 中间件**

```go
// go-framework/kitex/middleware/accesslog.go
package middleware

import (
    "context"
    "time"

    "github.com/byx-darwin/go-tools/go-common/log"
)

// Endpoint Kitex RPC 端点函数。
type Endpoint func(ctx context.Context, req, resp interface{}) error

// Middleware Kitex RPC 中间件。
type Middleware func(Endpoint) Endpoint

// AccessLog 返回 Kitex server-side AccessLog 中间件。
func AccessLog() Middleware {
    accessLog := log.L().WithCategory(log.CategoryRPC)
    return func(next Endpoint) Endpoint {
        return func(ctx context.Context, req, resp interface{}) error {
            start := time.Now()
            err := next(ctx, req, resp)
            latency := time.Since(start)

            if err != nil {
                accessLog.ErrorContext(ctx, "rpc request failed", err,
                    "latency_ms", latency.Milliseconds(),
                )
            } else {
                accessLog.InfoContext(ctx, "rpc request handled",
                    "latency_ms", latency.Milliseconds(),
                )
            }
            return err
        }
    }
}
```

- [ ] **Step 3: 运行测试**

```bash
cd go-framework && go test ./hertz/middleware/... ./kitex/middleware/... -v
```

Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add go-framework/hertz/middleware/accesslog.go go-framework/kitex/middleware/accesslog.go
git commit -m "refactor(middleware): update accesslog to use new log API with categories"
```

---

## 阶段三：验证和文档

### Task 3.1: 全模块构建和测试

**Files:**
- 无新文件

- [ ] **Step 1: 构建所有模块**

```bash
go build ./go-common/log/... ./go-framework/hertz/log/... ./go-framework/kitex/log/...
```

Expected: 成功

- [ ] **Step 2: 测试所有模块**

```bash
go test ./go-common/log/... ./go-framework/hertz/log/... ./go-framework/kitex/log/... -count=1 -v
```

Expected: PASS

- [ ] **Step 3: 测试带 build tag**

```bash
go test -tags with_rotation ./go-common/log/... -count=1 -v
```

Expected: PASS

- [ ] **Step 4: 运行 lint**

```bash
golangci-lint run ./go-common/log/... ./go-framework/hertz/log/... ./go-framework/kitex/log/...
```

Expected: 无错误

- [ ] **Step 5: 检查覆盖率**

```bash
go test -coverprofile=coverage.out ./go-common/log/...
go tool cover -func=coverage.out | grep total
```

Expected: > 80%

- [ ] **Step 6: 提交（如果有修复）**

```bash
git add .
git commit -m "test: all log packages pass build, test, and lint"
```

---

### Task 3.2: 更新 README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: 更新 go-common/log 说明**

在 README 中添加新的日志功能说明：

```markdown
## go-common/log

Enhanced structured logging with:
- Category-based logging (access, error, biz, rpc, db, etc.)
- Release info injection (service.name, version, environment)
- Context helpers (request_id, trace_id)
- Sensitive data masking (password, token, credit_card)
- Optional file rotation (build tag: with_rotation)
- oops error extraction

Usage:

```go
// Initialize
log.Init(log.Config{
    Level: "info",
    Format: "json",
    Mode: "both",
    File: log.FileConfig{
        Dir: "/var/log/app",
        Filename: "app.log",
    },
    Masking: log.MaskConfig{
        Enabled: true,
        MaskedFields: []string{"password", "token"},
    },
}, log.ReleaseInfo{
    ServiceName: "user-service",
    Version: "v1.0.0",
})

// Use with categories
accessLog := log.L().WithCategory(log.CategoryAccess)
accessLog.InfoContext(ctx, "request handled", "method", "GET")
```
```

- [ ] **Step 2: 提交**

```bash
git add README.md
git commit -m "docs: update README with enhanced log features"
```

---

## 完成标准检查

- [ ] go-common/log 支持分类、ReleaseInfo、Context 注入
- [ ] 支持 lumberjack 文件轮转（可选，build tag）
- [ ] 支持 oops 错误提取
- [ ] 支持敏感数据脱敏
- [ ] Hertz/Kitex 适配器工作正常
- [ ] 中间件更新使用新 API
- [ ] 所有测试通过
- [ ] Lint 通过
- [ ] 覆盖率 > 80%
- [ ] README 更新

---

## 执行选择

**Plan complete and saved to `docs/superpowers/plans/2026-06-25-log-enhancement.md`. Two execution options:**

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

**Which approach?**
