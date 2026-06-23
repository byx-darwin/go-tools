# ncgo 模板改造方案

> 本文档说明 ncgo 的脚手架模板应如何改造，从"内嵌完整实现"改为"import go-tools 三库 + 薄适配层"。

## 一、改造原则

1. **模板只生成"胶水代码"** — 配置加载、中间件 wiring、response 工具等全部从 go-tools 三库 import
2. **保留架构模式** — Handler→UseCase→Repository 的分层、ncgo:wire 锚点、samber/do DI 模式保持不变
3. **保留项目特有逻辑** — Rate Limit（业务规则）、i18n、Caller Allowlist 等项目级功能仍在模板中
4. **渐进式改造** — 先改最安全的（response/rpcerror），再改核心的（config/interceptor），最后改高级的（redis/kafka）
5. **结构化日志** — ncgo 模板代码统一使用结构化 key-value 写法替代 printf 风格，确保日志可检索、可聚合

## 二、日志规范：printf → 结构化

### 当前（printf 风格）

```go
klog.Infof("user login: userId=%d, ip=%s", 12345, "10.0.1.25")
klog.Errorf("query failed: sql=%s, err=%v", sql, err)
```

**问题**：
- `userId=12345` 只是一个字符串子串，无法按字段检索
- 日志平台无法按 `error_code` 聚合统计
- oops 的 Code/Domain/Public 信息丢失

### 改造后（结构化 key-value）

```go
klog.CtxKVLog(ctx, klog.LevelInfo, "user login",
    "user_id", 12345,
    "ip", "10.0.1.25",
    "elapsed_ms", 15,
)

klog.CtxKVLog(ctx, klog.LevelError, "query failed",
    "sql", shortSQL,
    "error", err,  // ← oops Handler 自动提取 code/domain/public/stack
)
```

**输出对比**：

| | printf 风格 | 结构化风格 |
|------|-----------|-----------|
| 日志 | `"user login: userId=12345, ip=10.0.1.25"` | `{"msg":"user login","user_id":12345,"ip":"10.0.1.25"}` |
| 检索 | ❌ 全文搜索 `userId=12345` 可能误匹配 | ✅ `user_id=12345` 精确检索 |
| 聚合 | ❌ 无法按 error_code 分组 | ✅ `error_code=20100` 分组统计 |
| oops | ❌ Code/Domain/Public 混在 msg 里 | ✅ 自动提取为独立字段 |

### 向后兼容

`klog.Infof` / `hlog.Debugf` 等 printf 风格**仍然可用**（适配器桥接），但**模板中不再生成 printf 风格代码**。新生成的项目默认使用 `CtxKVLog` 结构化写法。

## 二、Kitex 模板改造明细

### 2.1 `conf.yaml` — 配置加载

**当前**：~266 行，内嵌完整 Config 结构体 + Load/Default/Validate

**改造后**：

```go
// internal/base/conf/conf.go
package conf

import (
    fwconfig "gitee.com/byx_darwin/go-framework/config"
    golog "gitee.com/byx_darwin/go-common/log"
    mwredis "gitee.com/byx_darwin/go-middleware/redis"
    mwkafka "gitee.com/byx_darwin/go-middleware/kafka"
    mwdb "gitee.com/byx_darwin/go-middleware/db"
    "gopkg.in/yaml.v3"
)

// Config 项目总配置（ncgo 生成），嵌入框架层 + 中间件层 + 项目特有配置
type Config struct {
    Env       string                `json:"env" yaml:"env"`                         // 运行环境：dev / staging / prod
    Debug     bool                  `json:"debug" yaml:"debug"`                     // 是否开启调试模式
    Log       golog.Config          `json:"log" yaml:"log"`                         // ← 日志配置（go-common/log）
    Server    fwconfig.KitexServer  `json:"server" yaml:"server"`                   // ← Kitex 服务端配置（go-framework）
    Registry  fwconfig.Registry     `json:"registry" yaml:"registry"`               // ← 服务注册配置（go-framework）
    Database  mwdb.Config           `json:"database" yaml:"database"`               // ← 数据库配置（go-middleware/db）
    Redis     mwredis.Config        `json:"redis" yaml:"redis"`                     // ← Redis 配置（go-middleware/redis）
    Kafka     mwkafka.WriterConfig  `json:"kafka" yaml:"kafka"`                     // ← Kafka 配置（go-middleware/kafka）
    Observability fwconfig.ObservabilityConfig `json:"observability" yaml:"observability"` // ← 可观测性配置（go-framework）
    // 项目特有配置
    RateLimit RateLimitConfig       `json:"rate_limit" yaml:"rate_limit"`           // 限流规则（项目业务逻辑）
    Auth      AuthConfig            `json:"auth" yaml:"auth"`                       // 鉴权配置（项目业务逻辑）
}
```

**减少代码量**：~266 行 → ~90 行（减少 ~66%）

**完整 YAML 示例（ncgo 生成的 conf.yaml）**：

```yaml
env: dev
debug: true

log:
  path: ./log
  max_size: 100            # 生产建议 100MB（SDK 默认 10MB）
  max_age: 7
  max_backups: 30
  compress: true
  output_mode: 3           # 控制台+文件
  suffix: .log
  rotation_duration: 1440  # 24h（SDK 默认值）

server:
  name: user-service
  addr: :8888
  network: tcp
  read_write_timeout: 30s
  exit_wait_time: 10s

registry:
  enable: false
  space: default
  env: dev
  version: v1.0.0

database:
  enabled: true
  dsn: ${DATABASE_DSN}
  max_conns: 20
  min_conns: 2

redis:
  addrs:
    - 127.0.0.1:6379
  db: 0
  password: ${REDIS_PASSWORD}
  pool_size: 10

observability:
  enabled: true
  endpoint: apmplus-cn-beijing.volces.com:4317
  app_key: ${APMPLUS_APP_KEY}
  service_name: user-service
  sample_rate: 1.0

rate_limit:
  enabled: false
  # ... 业务特有配置

auth:
  caller_allowlist:
    - gateway-service
```

### 2.2 `interceptor.yaml` — RPC 中间件

**当前**：~145 行，完整自实现

**改造后**：

```go
// internal/pkg/interceptor/interceptor.go
package interceptor

// Re-export framework-level interceptors for this project.
// Add project-specific interceptors below.

import (
    fwmw "gitee.com/byx_darwin/go-framework/kitex/middleware"
)

// Standard interceptors — provided by go-framework.
var (
    RequestID       = fwmw.RequestID
    AccessLog       = fwmw.AccessLog
    Recovery        = fwmw.Recovery
    RequestTimeout  = fwmw.RequestTimeout
    CallerAllowlist = fwmw.CallerAllowlist
)

// Project-specific interceptors go here.
```

**减少代码量**：~145 行 → ~20 行（减少 ~86%）

### 2.3 `rpcerror.yaml` — RPC 错误

**当前**：~89 行

**改造后**：

```go
// internal/pkg/rpcerror/rpcerror.go
package rpcerror

import fwrpcerr "gitee.com/byx_darwin/go-framework/kitex/rpcerror"

// Re-export or extend.
var (
    ToBizError   = fwrpcerr.ToBizError
    InternalErrorf = fwrpcerr.InternalErrorf
    TimeoutError   = fwrpcerr.TimeoutError
    PermissionDenied = fwrpcerr.PermissionDenied
    BizCode        = fwrpcerr.BizCode
    FormatBiz      = fwrpcerr.FormatBiz
)
```

**减少代码量**：~89 行 → ~15 行（减少 ~83%）

### ⚠️ rpcerror 安全规则

`oops.Public()` 消息**禁止包含**以下敏感信息：
- 内部 IP 地址 / 端口号
- 堆栈 trace（应放入 `Private()`）
- SQL 语句 / 表名
- 服务器文件路径

正确示例：
```go
// ✅ 正确：Public 只暴露用户可读信息
oops.In("user").Code(20100).Public("user_not_found").
    Private(fmt.Sprintf("user_id=%d, sql=%s", id, query))

// ❌ 错误：内部信息泄漏到 Public
oops.Public(fmt.Sprintf("postgres://10.0.1.5:5432 timeout"))
```
详见 `go-middleware/*/errors.go` 和 `go-framework/kitex/rpcerror/errors.go` 的预定义错误。

### 2.4 `server.yaml` — Server 入口

**当前**：~144 行，包含地址解析、DI、数据库连接等

**改造后**：

```go
// internal/base/server/server.go
package server

import (
    fwkitex "gitee.com/byx_darwin/go-framework/kitex/option"
    // ... project-specific imports
)

func Run(extraOptions ...kitexserver.Option) {
    cfg := conf.Get()

    // Use framework-level option builder
    opts, err := fwkitex.NewServerOption(ctx, cfg.Server, cfg.Registry)
    if err != nil { log.Fatalf(...) }

    // Add project-specific middleware
    opts = append(opts, kitexserver.WithMiddleware(endpoint.Chain(
        interceptor.RequestID(),
        interceptor.AccessLog(),
        interceptor.Recovery(),
        interceptor.CallerAllowlist(...),
        interceptor.RequestTimeout(...),
    )))

    // ... rest of wiring (DI, repository, usecase)
}
```

**减少代码量**：~144 行 → ~60 行（减少 ~58%）

### 2.5 保持不变

| 模板 | 原因 |
|------|------|
| `handler.yaml` | 架构模式（Handler→UseCase 委托），不是库代码 |
| `usecase.yaml` | 业务逻辑桩代码，必须按服务生成 |
| `repository.yaml` | 数据访问层桩代码，必须按服务生成 |
| `main.yaml` | 入口文件，保持简洁即可 |
| `makefile.yaml` | 构建脚本 |
| `client.yaml` / `client_test.yaml` | RPC 客户端测试脚手架 |

## 三、Hertz 模板改造明细

### 3.1 `layout.yaml` 中的 response 包

**当前**：内嵌完整 response.go（~100 行）

**改造后**：

```go
// internal/pkg/response/response.go
package response

import fwresp "gitee.com/byx_darwin/go-framework/hertz/response"

// Re-export framework response helpers.
var (
    OK        = fwresp.OK
    Err       = fwresp.Err
    BindError = fwresp.BindError
)

// Project-specific extensions (i18n, custom codes) below.
```

### 3.2 `optional/redis.go` — Redis add-on

**当前**：~83 行，自实现 Redis wrapper

**改造后**：

```go
// internal/base/data/redis.go
package data

import mwredis "gitee.com/byx_darwin/go-middleware/redis"

// Use the shared Redis client from go-middleware.
// NewRedis delegates to mwredis.NewUniversalClient with project config.
func NewRedis(ctx context.Context, cfg *Config) (*Redis, func(), error) {
    client, err := mwredis.NewUniversalClient(ctx, cfg.Redis.ToOptions())
    if err != nil { return nil, nil, err }
    cleanup := func() { _ = client.Close() }
    return &Redis{Client: client}, cleanup, nil
}
```

**减少代码量**：~83 行 → ~15 行

### 3.3 `optional/kafka.go` — Kafka add-on

已决策统一为 `kafka-go`（D1）。go-middleware/kafka 使用 kafka-go 重写：

```go
// internal/base/data/kafka.go
package data

import mwkafka "gitee.com/byx_darwin/go-middleware/kafka"

// go-middleware/kafka 提供 kafka-go 的 Writer/Reader 工厂
func NewKafkaWriter(ctx context.Context, cfg mwkafka.WriterConfig) (*kafka.Writer, func(), error) {
    return mwkafka.NewWriter(ctx, cfg)
}
```

### 3.4 Hertz 可选配置 (optional-config/)

YAML 配置文件保持项目级，但字段名和类型与 go-middleware 的配置结构体对齐。

### 3.5 `optional/observability_logging.go` — 可观测性

**当前**：ncgo 自实现日志 + OTel 集成（~150 行）

**改造后**：

```go
// internal/pkg/observability/observability.go
package observability

import (
    fwobs "gitee.com/byx_darwin/go-framework/kitex/observability"
    mwtls "gitee.com/byx_darwin/go-middleware/tls"
)

// 链路追踪 — 使用 go-framework 统一 Provider
func NewTraceProvider(cfg fwobs.ObservabilityConfig) (func(), error) {
    _, shutdown, err := fwobs.NewProvider(cfg)
    return shutdown, err
}

// 日志上报 — 使用 go-middleware TLS 客户端
func NewLogProducer(cfg mwtls.ProducerConfig) (*mwtls.Producer, error) {
    return mwtls.NewProducer(cfg)
}
```

**减少代码量**：~150 行 → ~25 行（减少 ~83%）

### 3.6 `main.yaml` 中的日志初始化（对齐 ncgo klog/hlog）

**当前**：ncgo 模板使用 klog/hlog 原生 Logger，或裸 `log.Printf`

**改造后**：统一使用 `go-common/log`（基于 slog）替代，通过适配器注入 klog/hlog：

```go
// main.go — ncgo 生成（Kitex 项目）
import (
    golog "gitee.com/byx_darwin/go-common/log"
    "gitee.com/byx_darwin/go-common/log/adapters"
)

func main() {
    // 初始化 slog Logger（自动轮转 + 压缩 + OTel）
    logger := golog.New(cfg.Log)

    // 替换 Kitex 原生 klog（ncgo 模板中的 klog.Infof 等无需修改）
    klog.SetLogger(adapters.NewKitexAdapter(logger))

    // ...
}
```

```go
// main.go — ncgo 生成（Hertz 项目）
func main() {
    logger := golog.New(cfg.Log)

    // 替换 Hertz 原生 hlog
    hlog.SetLogger(adapters.NewHertzAdapter(logger))

    // ...
}
```

**关键**：ncgo 模板中的 `klog.Infof("xxx")` / `hlog.Debugf("xxx")` **无需修改**，适配器自动桥接到 slog → JSON 输出。

### ncgo 模板代码中的日志写法

**Handler 层（模板生成的结构化日志）**：

```go
// internal/handler/user/handler.go — ncgo 生成
func (h *Handler) GetUser(ctx context.Context, req *pb.GetUserReq) (*pb.GetUserResp, error) {
    user, err := h.uc.GetUser(ctx, req.UserId)
    if err != nil {
        // 结构化错误日志 — oops Handler 自动提取 error_code/domain/public
        klog.CtxKVLog(ctx, klog.LevelError, "get user failed",
            "user_id", req.UserId,
            "error", err,
        )
        return nil, rpcerror.ToBizError(err)
    }
    // 结构化正常日志
    klog.CtxKVLog(ctx, klog.LevelInfo, "user found",
        "user_id", user.ID,
        "elapsed_ms", time.Since(start).Milliseconds(),
    )
    return &pb.GetUserResp{User: user}, nil
}
```

**UseCase 层（同风格）**：

```go
// internal/usecase/user/usecase.go — ncgo 生成
func (uc *UseCase) GetUser(ctx context.Context, userID int64) (*User, error) {
    user, err := uc.repo.FindByID(ctx, userID)
    if err != nil {
        uc.logger.CtxKVLog(ctx, klog.LevelError, "repository query failed",
            "user_id", userID,
            "error", err,
        )
        return nil, rpcerror.ErrNotFound.With("user_id", userID).Wrap(err)
    }
    return user, nil
}
```

**减少代码量**：分散的 klog 初始化 ~20 行 → 集中 5 行

## 四、改造量化汇总

| 模板 | 当前行数 | 改造后行数 | 减少 |
|------|---------|-----------|------|
| Kitex conf.yaml | ~266 | ~80 | -70% |
| Kitex interceptor.yaml | ~145 | ~20 | -86% |
| Kitex rpcerror.yaml | ~89 | ~15 | -83% |
| Kitex server.yaml | ~144 | ~60 | -58% |
| Hertz response (layout.yaml) | ~100 | ~15 | -85% |
| Hertz optional/redis.go | ~83 | ~15 | -82% |
| Hertz optional/kafka.go | ~133 | ~15 | -89% |
| Hertz optional/observability_logging.go | ~150 | ~25 | -83% |
| Log 初始化 (main.yaml) | ~20 | ~5 | -75% |
| **合计** | **~1130** | **~250** | **-78%** |

## 五、注意事项

1. **Golden 测试更新**：模板变化后需要 `go test ./internal/scaffold/mono/... -update-golden`
2. **向后兼容**：已生成的项目不自动迁移，提供 `ncgo upgrade` 命令辅助
3. **错误库统一**：已决策 oops 为主（D3），go-framework 内部统一用 oops，对外暴露 `rpcerror.FromPkgErrors(err error) error` 转换函数。go-tools 原有 `ErrorType` 枚举保留在 go-framework 中作为辅助
4. **DI 模式不变**：`samber/do` 的 provide/inject 模式保持项目级
