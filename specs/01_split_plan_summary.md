# go-tools 拆库方案摘要

>

## 一、当前结构

```text
go-tools/                     (go workspace)
├── config/                   module github.com/byx-darwin/go-tools/config
├── hertz/                    module github.com/byx-darwin/go-tools/hertz
├── kitex/                    module github.com/byx-darwin/go-tools/kitex
├── middleware/               module github.com/byx-darwin/go-tools/middleware
└── tools/                    module github.com/byx-darwin/go-tools/tools
```

**问题**：
- 5 个模块边界模糊（config 散布在各处，middleware 只包含 Redis/Kafka）
- 工具类 (tools/) 命名不规范
- 任何项目引入 go-tools 都要拉全部 5 个模块

## 二、目标结构（三层）

```text
              go-common    ← 最底层，零框架依赖
                  ↑
               go-auth      ← 认证工具
              ↑       ↑
    ┌─────────┘       └─────────┐
go-middleware              go-framework
中间件客户端                框架适配
（Redis / Kafka / DB）      （Hertz / Kitex + 配置）
```

> 真实拓扑是 **DAG**：`go-framework` 与 `go-middleware` 为**兄弟关系**，二者均依赖 `go-auth` + `go-common`，彼此无依赖。

## 三、模块归属

### go-common（通用工具 — 零框架依赖）

| 来源 | 目标路径 | 说明 |
|------|---------|------|
| `tools/crypto/*` | `go-common/crypto/` | MD5/SHA/HMAC/TEA |
| `tools/cache/*` | `go-common/cache/` | 缓存封装 — 底层使用 `github.com/samber/hot`（替代自定义 FIFO/LRU/LFU/CLOCK/MRU 实现） |
| `tools/http_client/*` | `go-common/httpclient/` | fasthttp 客户端 + 重试 |
| `tools/time/*` | `go-common/timeutil/` | 时间格式化/月份 |
| `tools/netutil/*` | `go-common/netutil/` | 内网 IP / 网络检测 |
| `tools/captcha/*` | `go-common/captcha/` | 验证码缓存 |
| `tools/ak.go` | `go-common/auth/ak.go` | AK/SK 生成 |
| `tools/entutils/*` | `go-common/entutil/` | Ent ORM 工具 |
| — | `go-common/log/` | 结构化 JSON 日志库（Logger 接口 + 级别 + TraceID 注入） |

**依赖**：标准库 `log/slog` + `golang.org/x/crypto` + `fasthttp` + `gopkg.in/natefinch/lumberjack.v2`（仅文件轮转）+ `github.com/samber/hot`（缓存底层）

### go-middleware（中间件客户端 — 不依赖框架）

| 来源 | 目标路径 | 说明 |
|------|---------|------|
| `middleware/redis/redis.go` | `go-middleware/redis/client.go` | Redis 客户端工厂 |
| `config/redis/config.go` | `go-middleware/redis/config.go` | Redis 配置结构体 |
| `middleware/kafka/sarama/producer.go` | `go-middleware/kafka/producer.go` | Kafka 生产者 |
| `config/kafka/sarama/config.go` | `go-middleware/kafka/config.go` | Kafka 配置结构体（已决策 kafka-go D1） |
| — | `go-middleware/es/` | Elasticsearch 客户端（新增，来自 反哺） |
| — | `go-middleware/clickhouse/` | ClickHouse 客户端（新增，来自 反哺） |
| — | `go-middleware/tls/` | 火山引擎日志服务 TLS 客户端（新增：结构化日志上报） |
| `config/db/config.go` | `go-middleware/db/config.go` | 数据库配置结构体 |

**依赖**：go-common + `go-redis/v9` + `sarama`

### go-framework（框架适配 — Hertz + Kitex）

| 来源 | 目标路径 | 说明 |
|------|---------|------|
| `hertz/server.go` | `go-framework/hertz/server.go` | HTTP 服务工厂 |
| `hertz/response.go` | `go-framework/hertz/response.go` | 统一响应 |
| `hertz/middleware/*` | `go-framework/hertz/middleware/` | CORS/Auth/Casbin/Location |
| `hertz/registry/polaris/*` | `go-framework/hertz/registry/polaris.go` | Polaris 注册 |
| `kitex/option/*` | `go-framework/kitex/option/` | RPC 服务端/客户端 Option |
| `kitex/rpc_error/*` | `go-framework/kitex/rpcerror/` | 错误码 |
| `kitex/registry/polaris/*` | `go-framework/kitex/registry/polaris.go` | Polaris 注册 |
| `kitex/discover/polaris/*` | `go-framework/kitex/discover/polaris.go` | Polaris 发现 |
| `config/config.go` | `go-framework/config/config.go` | 通用配置 (Jaeger/Registry) |
| `config/polaris.go` | `go-framework/config/polaris.go` | Polaris 配置加载 |
| `config/hertz/*` | `go-framework/config/hertz/` | Hertz 配置 |
| `config/kitex/*` | `go-framework/config/kitex/` | Kitex 配置 |
| — | `go-framework/kitex/observability/` | Kitex 链路追踪中间件（新增：OTel OTLP） |
| — | `go-framework/hertz/observability/` | Hertz 链路追踪中间件（新增：OTel OTLP） |
| — | `go-framework/config/observability.go` | 可观测统一配置（Endpoint/AppKey/采样率） |
| — | `go-framework/kitex/middleware/accesslog.go` | Kitex Access Log 中间件（结构化 JSON 日志） |
| — | `go-framework/hertz/middleware/accesslog.go` | Hertz Access Log 中间件（结构化 JSON 日志） |

**依赖**：go-common + go-auth + `hertz` + `kitex` + `polaris-go`

## 四、导入路径变化

```text
# 拆分前
import "github.com/byx-darwin/go-tools/config/redis"
import "github.com/byx-darwin/go-tools/tools/crypto"
import "github.com/byx-darwin/go-tools/hertz"
import "github.com/byx-darwin/go-tools/kitex/option"

# 拆分后
import "github.com/byx-darwin/go-middleware/redis"
import "github.com/byx-darwin/go-common/crypto"
import "github.com/byx-darwin/go-framework/hertz"
import "github.com/byx-darwin/go-framework/kitex/option"
```

## 五、新增能力需求（来自  对齐）

拆分过程中需要从 "回流"到 go-tools 的能力：

| 能力 | 来源 | 归入模块 | 说明 |
|------|------|---------|------|
| Kitex interceptor 中间件 |  `interceptor.yaml` | go-framework `kitex/middleware/` | RequestID/AccessLog/Recovery/Timeout/CallerAllowlist |
| oops 风格 rpcerror ✅ | — | go-framework `kitex/rpcerror/` | 已完成：基于 samber/oops，替代 ErrorType 枚举 |
| Hertz response 工具 |  `layout.yaml` 内嵌 | go-framework `hertz/response.go` | 升级现有的 response.go |
| 统一 Redis UniversalClient |  `optional/redis.go` | go-middleware `redis/` | 升级 *redis.Client → UniversalClient |
| Kafka 库统一 |  `optional/kafka.go` | go-middleware `kafka/` | 已决策 kafka-go（D1），替代现有 sarama 实现 |
| 配置结构体增强 |  conf.yaml | go-middleware/* + go-framework/config/* | 字段对齐 + 单位统一（D2：time.Duration） |
| Kitex 链路追踪 |  `observability_logging.go` | go-framework `kitex/observability/` | OTel 标准协议，通过 `kitex-contrib/obs-opentelemetry` 集成 |
| Hertz 链路追踪 |  `observability_logging.go` | go-framework `hertz/observability/` | OTel 标准协议，通过 `hertz-contrib/obs-opentelemetry` 集成 |
| 日志服务客户端 |  `observability_logging.go` | go-middleware `tls/` | 火山引擎 TLS 日志 Producer/Consumer |

## 六、库间接口契约

### go-common 对外暴露

```go
// go-common/cache/cache.go — 基于 github.com/samber/hot 的封装（替代自定义 FIFO/LRU/LFU/CLOCK/MRU）
import "github.com/samber/hot"

// HotCache 是 samber/hot 的泛型缓存，支持 LRU/FIFO 淘汰 + TTL
// 用法: cache := hot.NewHotCache[K, V](hot.LRU, maxEntries).Build()
type HotCache[K comparable, V any] = hot.HotCache[K, V]

// NewCache 创建带 TTL 的缓存（对齐 中的 signature/ratelimit/idempotency 用法）
func NewCache[K comparable, V any](policy hot.Policy, size int) *hot.HotCache[K, V] {
    return hot.NewHotCache[K, V](policy, size).Build()
}

// go-common/crypto/  — 纯函数包，无 interface，直接暴露 func
func MD5(data []byte) string
func SHA256(data []byte) string
func HMACSHA256(data, key []byte) string
func TEAEncrypt(data, key []byte) ([]byte, error)

// go-common/log/logger.go — 基于 Go 标准库 log/slog，零外部日志依赖
type Logger struct { *slog.Logger; config *Config }
func New(cfg Config) *Logger  // 自动处理文件轮转+压缩+OTel span联动

// go-common/log/adapters/kitex.go — klog.FullLogger 适配器（对齐 ）
type KitexAdapter struct { ... }
func NewKitexAdapter(l *Logger) klog.FullLogger
// 用法: klog.SetLogger(log.NewKitexAdapter(logger))

// go-common/log/adapters/hertz.go — hlog.FullLogger 适配器（对齐 ）
type HertzAdapter struct { ... }
func NewHertzAdapter(l *Logger) hlog.FullLogger
// 用法: hlog.SetLogger(log.NewHertzAdapter(logger))
```

### go-middleware 对外暴露

```go
// go-middleware/redis/client.go
type Client interface {
    redis.UniversalClient  // 继承 go-redis 接口
}
func NewUniversalClient(ctx context.Context, cfg *Config) (Client, func(), error)

// go-middleware/kafka/producer.go
type Writer interface {
    WriteMessages(ctx context.Context, msgs ...kafka.Message) error
    Close() error
}
func NewWriter(cfg *WriterConfig) *Writer

// go-middleware/db/client.go
type DB struct { *sql.DB }
func NewDB(cfg *Config) (*DB, error)
func (db *DB) Ping(ctx context.Context) error

// go-middleware/es/client.go
type Client struct { *elasticsearch.Client }
func NewClient(cfg *Config) (*Client, error)
func (c *Client) Search(ctx context.Context, index string, query io.Reader) (*SearchResult, error)

// go-middleware/clickhouse/client.go
type Client struct { *clickhouse.Conn }
func NewClient(cfg *Config) (*Client, error)
func (c *Client) Query(ctx context.Context, query string) (*sql.Rows, error)

// go-middleware/tls/producer.go
type Producer struct { ... }
func NewProducer(cfg *ProducerConfig) (*Producer, error)
func (p *Producer) SendLog(ctx context.Context, topicID, log string) error
func (p *Producer) Close()
```

### go-framework 对外暴露

```go
// go-framework/kitex/observability/provider.go
type ObservabilityConfig struct {
    Enabled     bool    `yaml:"enabled"`
    Endpoint    string  `yaml:"endpoint"`
    AppKey      string  `yaml:"app_key"`   // 环境变量注入
    ServiceName string  `yaml:"service_name"`
    SampleRate  float64 `yaml:"sample_rate"`
}
func NewProvider(cfg ObservabilityConfig) (*otelProvider, func(), error)
func ServerSuite() []server.Option
func ClientSuite() []client.Option

// go-framework/hertz/observability/provider.go
func NewProvider(cfg ObservabilityConfig) (*otelProvider, func(), error)
func ServerMiddleware(cfg Config) app.HandlerFunc
func ClientMiddleware(cfg Config) app.HandlerFunc

// go-framework/kitex/middleware/accesslog.go — Kitex RPC Access Log
func AccessLog(logger log.Logger) kitexendpoint.Middleware

// go-framework/hertz/middleware/accesslog.go — Hertz HTTP Access Log
func AccessLog(logger log.Logger) app.HandlerFunc

// go-framework/kitex/option/server.go
func NewServerOption(ctx context.Context, cfg ServerConfig, reg Registry) ([]kitexserver.Option, error)

// go-framework/hertz/response.go
func OK(c *app.RequestContext, data interface{})
func Err(c *app.RequestContext, err error)

// go-framework/config/loader.go
func LoadYAML[T any](path string) (*T, error)
```

## 七、错误码段分配

```
go-framework:
  10000-99  系统级 (Internal/Timeout/Unavailable)
  10100-99  参数校验 (InvalidParam/MissingParam/BindError)
  10200-99  认证授权 (Unauthorized/Forbidden/TokenExpired)
  10300-99  配置/资源 (NotFound/Conflict/ConfigMissing)
  10400-99  RPC中间件 (CallerNotAllowed/PanicRecovered/MaxConcurrency)

go-middleware:
  20000-99  Redis (连接/池/命令/Sentinel/锁)
  20100-99  Kafka (连接/Producer/Consumer/消息)
  20200-99  DB (连接/查询/事务/约束)
  20300-99  Elasticsearch (连接/索引/文档/查询/集群)
  20400-99  ClickHouse (连接/写入/查询/表操作)
  20500-99  预留扩展
  20600-99  可观测性 (TLS日志上报/链路追踪)

项目自定义:
  40000-99  业务模块 (User/Order/Payment...)
  50000-99  外部依赖 (第三方服务调用)
```

详见各中间件包 `errors.go` 定义。
