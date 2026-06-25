# 增强日志系统设计文档

> 日期：2026-06-25  
> 状态：设计中  
> 范围：重构 go-common/log，引入分类、发布信息、上下文注入等能力

## 1. 背景

### 1.1 当前状态

**go-common/log：**
- 基于 slog 封装，支持 JSON/text 格式
- otelHandler 自动注入 trace_id/span_id
- 文件输出（无轮转）
- 无分类、无全局单例、无 ReleaseInfo

**ncgo/logging：**
- 多输出（console + file）
- lumberjack 文件轮转
- 8 个预定义分类（access/error/biz/rpc/db/panic/audit/security）
- ReleaseInfo 自动注入
- Context 注入（request_id, traffic_lane）
- samber/oops 错误提取

**go-framework/hertz/kitex：**
- 已有 observability 包（OTel tracer）
- 已有 accesslog 中间件（但未使用分类 API）

### 1.2 目标

- 统一日志库设计，支持分类、发布信息、上下文注入
- 保持零框架依赖（go-common 层）
- 框架适配器放在 go-framework 层
- 支持 lumberjack（可选）、oops（直接依赖）

## 2. 设计决策

| 问题 | 决策 |
|------|------|
| 依赖管理 | lumberjack 可选（build tag），oops 直接依赖 |
| 分类系统 | 提供通用常量 + 允许自定义 |
| 全局单例 | 两者都支持（log.L() + log.New()） |
| 分类 API | WithCategory() 子 logger 模式 |
| 分类用途 | 功能维度 + 模块维度 |
| ReleaseInfo | 可扩展，基础字段 + 自定义 |
| Context 辅助 | 通用方法 + 框架特定预定义函数 |
| oops 错误提取 | 直接依赖，提供 ErrorAttrs() |
| 框架适配器 | 移到 go-framework 层 |
| 实施方案 | 完全重新设计（破坏性变更） |

## 3. 架构设计

### 3.1 包结构

```
go-common/log/              # 核心日志库（无框架依赖）
├── logger.go               # Logger 实现
├── config.go               # Config 结构体
├── category.go             # 分类系统和常量
├── release.go              # ReleaseInfo
├── context.go              # Context 辅助工具
├── error.go                # oops 错误提取
├── handler.go              # slog.Handler 实现（multiHandler 等）
├── global.go               # 全局单例管理
├── rotation.go             # lumberjack 集成（build tag: with_rotation）
└── *_test.go

go-framework/hertz/log/     # Hertz 适配器
├── adapter.go              # hlog.FullLogger 实现
└── adapter_test.go

go-framework/kitex/log/     # Kitex 适配器
├── adapter.go              # klog.FullLogger 实现
└── adapter_test.go

go-framework/hertz/middleware/
└── accesslog.go            # 更新使用新 API

go-framework/kitex/middleware/
└── accesslog.go            # 更新使用新 API
```

### 3.2 核心流程

```
项目启动
  ↓
log.Init(cfg, release)  ← 初始化全局 logger
  ↓
各模块获取 categorized logger
  var accessLog = log.L().WithCategory("access")
  var bizLog = log.L().WithCategory("user-service")
  ↓
中间件注入上下文
  ctx = log.WithRequestID(ctx, "req-123")
  ↓
业务代码记录日志
  accessLog.InfoContext(ctx, "request handled", "method", "GET")
  ↓
slog.Handler 处理
  - 注入 trace_id/span_id（从 context）
  - 注入 request_id（从 context）
  - 注入 category（从 logger）
  - 注入 release 信息（全局）
  - 输出到 console/file（multiHandler）
```


## 4. 详细 API 设计

### 4.1 核心类型

#### Logger

```go
// Logger 结构化日志记录器
type Logger struct {
    inner    *slog.Logger
    category string
    attrs    []slog.Attr
    config   *Config
}

// 日志方法
func (l *Logger) DebugContext(ctx context.Context, msg string, args ...any)
func (l *Logger) InfoContext(ctx context.Context, msg string, args ...any)
func (l *Logger) WarnContext(ctx context.Context, msg string, args ...any)
func (l *Logger) ErrorContext(ctx context.Context, msg string, err error, args ...any)

// 子 logger
func (l *Logger) WithCategory(category string) *Logger
func (l *Logger) With(args ...any) *Logger

// 生命周期
func (l *Logger) Close() error
```

#### Config

```go
type Config struct {
    // 基础配置
    Level      string `yaml:"level"`       // "debug", "info", "warn", "error"
    Format     string `yaml:"format"`      // "json", "text"
    Mode       string `yaml:"mode"`        // "console", "file", "both", "none"
    AddSource  bool   `yaml:"add_source"`  // 是否添加代码位置

    // 文件配置
    File FileConfig `yaml:"file"`

    // 分类配置
    Categories map[string]CategoryConfig `yaml:"categories"`
}

type FileConfig struct {
    Dir        string `yaml:"dir"`         // 日志目录
    Filename   string `yaml:"filename"`    // 主日志文件名
    MaxSize    int    `yaml:"max_size"`    // 单文件最大 MB
    MaxBackups int    `yaml:"max_backups"` // 保留旧文件数
    MaxAge     int    `yaml:"max_age"`     // 保留天数
    Compress   bool   `yaml:"compress"`    // gzip 压缩
}

type CategoryConfig struct {
    Enabled  bool   `yaml:"enabled"`   // 是否启用
    File     string `yaml:"file"`      // 独立文件名（相对于 Dir）
    Level    string `yaml:"level"`     // 独立级别
}
```

### 4.2 全局单例

```go
// Init 初始化全局 logger
func Init(cfg Config, release ReleaseInfo) error

// L 获取全局 logger
func L() *Logger

// SetDefault 设置全局 logger（用于测试）
func SetDefault(l *Logger)

// Close 关闭全局 logger
func Close() error
```

### 4.3 分类系统

```go
// 预定义分类常量（功能维度）
const (
    CategoryAccess   = "access"    // HTTP 访问日志
    CategoryError    = "error"     // 错误日志
    CategoryBiz      = "biz"       // 业务逻辑
    CategoryRPC      = "rpc"       // RPC 调用
    CategoryDB       = "db"        // 数据库
    CategoryPanic    = "panic"     // panic 日志
    CategoryAudit    = "audit"     // 审计日志
    CategorySecurity = "security"  // 安全日志
)

// WithCategory 创建带分类的子 logger
func (l *Logger) WithCategory(category string) *Logger
```

**使用示例：**

```go
// 功能维度
accessLog := log.L().WithCategory(log.CategoryAccess)
accessLog.InfoContext(ctx, "request handled", "method", "GET")

// 模块维度
userLog := log.L().WithCategory("user-service")
userLog.InfoContext(ctx, "user created", "user_id", 123)
```

### 4.4 ReleaseInfo

```go
// ReleaseInfo 发布信息
type ReleaseInfo struct {
    ServiceName string `yaml:"service_name"`
    Version     string `yaml:"version"`
    GitSHA      string `yaml:"git_sha"`
    BuildTime   string `yaml:"build_time"`
    Environment string `yaml:"environment"`  // "dev", "staging", "prod"

    // 自定义字段
    Extra map[string]string `yaml:"extra"`
}

// WithExtra 添加自定义字段
func (r *ReleaseInfo) WithExtra(key, value string) *ReleaseInfo
```

**自动注入：**

```go
// 初始化时
log.Init(cfg, log.ReleaseInfo{
    ServiceName: "user-service",
    Version:     "v1.0.0",
    GitSHA:      "abc123",
    Environment: "prod",
})

// 每条日志自动包含
// {"service.name": "user-service", "service.version": "v1.0.0", ...}
```

### 4.5 Context 辅助

```go
// 通用方法
func WithContextValue(ctx context.Context, key, value string) context.Context
func ContextValue(ctx context.Context, key string) string

// 预定义 key
const (
    ContextKeyRequestID = "request_id"
    ContextKeyTraceID   = "trace_id"
    ContextKeySpanID    = "span_id"
)

// 便捷方法
func WithRequestID(ctx context.Context, requestID string) context.Context
func RequestIDFromContext(ctx context.Context) string
```

**框架特定方法（go-framework/hertz/log）：**

```go
// Hertz 中间件
func HertzRequestIDMiddleware() app.HandlerFunc {
    return func(ctx context.Context, c *app.RequestContext) {
        requestID := c.Request.Header.Get("X-Request-ID")
        ctx = log.WithRequestID(ctx, requestID)
        c.Next(ctx)
    }
}
```

### 4.6 错误处理

```go
// ErrorAttrs 从 samber/oops 错误提取结构化字段
func ErrorAttrs(err error) []any

// 使用示例
err := oops.WithMessage("db failed").Wrap(originalErr)
log.L().ErrorContext(ctx, "operation failed", err, log.ErrorAttrs(err)...)
// 输出: {"error.message": "db failed", "error.code": "DB_ERROR", ...}
```

### 4.7 文件轮转（可选）

```go
// build tag: with_rotation

// rotation.go
//go:build with_rotation

import "gopkg.in/natefinsh/lumberjack.v2"

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

**编译时启用：**

```bash
go build -tags with_rotation
```

## 5. Handler 链

### 5.1 Handler 职责

```
Logger
  ↓
categoryHandler      ← 注入 category 字段
  ↓
releaseHandler       ← 注入 release 信息
  ↓
contextHandler       ← 注入 context 字段（request_id 等）
  ↓
otelHandler          ← 注入 trace_id/span_id（已有）
  ↓
slog.JSONHandler     ← 最终输出
```

### 5.2 multiHandler

```go
// multiHandler 多输出 fan-out
type multiHandler struct {
    handlers []slog.Handler
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
    for _, handler := range h.handlers {
        if err := handler.Handle(ctx, r); err != nil {
            return err
        }
    }
    return nil
}
```

**使用场景：**

```go
// Mode = "both" 时
handler := &multiHandler{
    handlers: []slog.Handler{
        consoleHandler,  // stdout
        fileHandler,     // 文件（带轮转）
    },
}
```

## 6. 框架适配器

### 6.1 Hertz 适配器

```go
// go-framework/hertz/log/adapter.go

package hertzlog

import (
    "context"
    "github.com/cloudwego/hertz/pkg/common/hlog"
    "github.com/byx-darwin/go-tools/go-common/log"
)

// HertzAdapter 实现 hlog.FullLogger
type HertzAdapter struct {
    logger *log.Logger
}

func NewHertzAdapter(logger *log.Logger) hlog.FullLogger {
    return &HertzAdapter{logger: logger}
}

// 实现 hlog.FullLogger 接口
func (a *HertzAdapter) Info(ctx context.Context, msg string, fields ...map[string]interface{}) {
    a.logger.InfoContext(ctx, msg, convertFields(fields)...)
}

// ... 其他方法：Debug, Warn, Error, Fatal, etc.
```

### 6.2 Kitex 适配器

```go
// go-framework/kitex/log/adapter.go

package kitexlog

import (
    "context"
    "github.com/cloudwego/kitex/pkg/klog"
    "github.com/byx-darwin/go-tools/go-common/log"
)

// KitexAdapter 实现 klog.FullLogger
type KitexAdapter struct {
    logger *log.Logger
}

func NewKitexAdapter(logger *log.Logger) klog.FullLogger {
    return &KitexAdapter{logger: logger}
}

// 实现 klog.FullLogger 接口
func (a *KitexAdapter) Info(ctx context.Context, msg string, fields ...map[string]interface{}) {
    a.logger.InfoContext(ctx, msg, convertFields(fields)...)
}

// ... 其他方法
```

### 6.3 中间件更新

```go
// go-framework/hertz/middleware/accesslog.go

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

## 7. 配置示例

### 7.1 YAML 配置

```yaml
log:
  level: "info"
  format: "json"
  mode: "both"
  add_source: false

  file:
    dir: "/var/log/app"
    filename: "app.log"
    max_size: 100
    max_backups: 7
    max_age: 30
    compress: true

  categories:
    access:
      enabled: true
      file: "access.log"
      level: "info"
    error:
      enabled: true
      file: "error.log"
      level: "error"
    user-service:
      enabled: true
      file: "user.log"
      level: "debug"
```

### 7.2 代码初始化

```go
// main.go

import (
    "github.com/byx-darwin/go-tools/go-common/log"
    "gopkg.in/yaml.v3"
)

func main() {
    // 加载配置
    var cfg log.Config
    yaml.Unmarshal(configData, &cfg)

    // 初始化
    err := log.Init(cfg, log.ReleaseInfo{
        ServiceName: "user-service",
        Version:     "v1.0.0",
        GitSHA:      getGitSHA(),
        BuildTime:   getBuildTime(),
        Environment: getEnv(),
    })
    if err != nil {
        panic(err)
    }
    defer log.Close()

    // 各模块初始化
    var (
        accessLog = log.L().WithCategory(log.CategoryAccess)
        errorLog  = log.L().WithCategory(log.CategoryError)
        userLog   = log.L().WithCategory("user-service")
    )

    // 启动服务
    startServer()
}
```

## 8. 迁移计划

### 8.1 阶段一：核心库重构（go-common/log）

1. 创建新的包结构
2. 实现 Logger、Config、Category、ReleaseInfo
3. 实现 Handler 链（categoryHandler, releaseHandler, contextHandler）
4. 实现 multiHandler
5. 集成 lumberjack（build tag）
6. 集成 oops
7. 编写单元测试

### 8.2 阶段二：框架适配器（go-framework）

1. 创建 hertz/log 和 kitex/log 包
2. 实现 HertzAdapter 和 KitexAdapter
3. 更新 hertz/middleware/accesslog.go
4. 更新 kitex/middleware/accesslog.go
5. 编写集成测试

### 8.3 破坏性变更

**旧 API：**

```go
logger := log.New(log.WithLevel("info"))
logger.InfoContext(ctx, "message", "key", "value")
```

**新 API：**

```go
log.Init(cfg, release)
log.L().WithCategory("access").InfoContext(ctx, "message", "key", "value")
```

**迁移指南：**

1. 替换 `log.New()` 为 `log.Init()`
2. 替换 `logger.InfoContext()` 为 `log.L().InfoContext()` 或 `log.L().WithCategory(...).InfoContext()`
3. 更新配置文件格式

## 9. 测试策略

### 9.1 单元测试

- Logger 基础功能
- 分类系统
- ReleaseInfo 注入
- Context 辅助
- Handler 链
- multiHandler
- 文件轮转（build tag）

### 9.2 集成测试

- Hertz 适配器
- Kitex 适配器
- 中间件集成
- 端到端日志流

### 9.3 验证命令

```bash
# 核心库
go test ./go-common/log/... -v -count=1

# 框架适配器
go test ./go-framework/hertz/log/... -v -count=1
go test ./go-framework/kitex/log/... -v -count=1

# 带文件轮转
go test -tags with_rotation ./go-common/log/... -v -count=1

# Lint
golangci-lint run ./go-common/log/...
golangci-lint run ./go-framework/hertz/log/...
golangci-lint run ./go-framework/kitex/log/...
```

## 10. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| 破坏性变更 | 高 | 提供迁移指南，逐步迁移 |
| lumberjack 依赖 | 中 | 使用 build tag 隔离 |
| oops 依赖 | 低 | 直接依赖，但只在 ErrorAttrs 中使用 |
| 性能影响 | 低 | Handler 链设计为轻量级 |
| 配置复杂性 | 中 | 提供默认配置，简化常见场景 |

## 11. 成功标准

- [ ] go-common/log 支持分类、ReleaseInfo、Context 注入
- [ ] 支持 lumberjack 文件轮转（可选）
- [ ] 支持 oops 错误提取
- [ ] Hertz/Kitex 适配器工作正常
- [ ] 中间件更新使用新 API
- [ ] 所有测试通过
- [ ] Lint 通过
- [ ] 迁移指南完整

## 12. 参考资料

- ncgo logging 实现
- samber/oops 文档
- lumberjack 文档
- slog 官方文档
- Hertz hlog 接口
- Kitex klog 接口
