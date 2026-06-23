# 配置结构体 & YAML Schema 对齐方案

> 本文档说明 go-tools 和 ncgo 模板之间配置结构体和 YAML 配置文件的字段对齐策略。

## 一、对标策略

```
┌─────────────────────────────────────────────────────────────┐
│                     go-framework/config                      │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │ Kitex    │  │ Hertz    │  │ Registry │  │ Jaeger   │   │
│  │ Server   │  │ Server   │  │          │  │          │   │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
└─────────────────────────────────────────────────────────────┘
                             ↑
                             │ import
┌─────────────────────────────────────────────────────────────┐
│                    go-middleware/config                      │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                 │
│  │ Redis    │  │ Kafka    │  │ DB       │                 │
│  │ Config   │  │ Config   │  │ Config   │                 │
│  └──────────┘  └──────────┘  └──────────┘                 │
└─────────────────────────────────────────────────────────────┘
                             ↑
                             │ import
┌─────────────────────────────────────────────────────────────┐
│                    go-common/log                             │
│  ┌──────────┐                                              │
│  │ Config   │  ← 日志轮转/压缩/输出模式配置                   │
│  └──────────┘                                              │
└─────────────────────────────────────────────────────────────┘
                             ↑
                             │ import
┌─────────────────────────────────────────────────────────────┐
│   ncgo 生成的项目的 internal/base/conf/conf.go               │
│   ┌───────────────────────────────────────────────────┐     │
│   │ type Config struct {                              │     │
│   │     Log      golog.Config          // 日志配置     │     │
│   │     Server   fwconfig.KitexServer  // 框架库       │     │
│   │     Database mwdb.Config           // 中间件库      │     │
│   │     Redis    mwredis.Config        // 中间件库      │     │
│   │     Observability fwconfig.Config  // 观测性配置    │     │
│   │     // 项目特有字段:                               │     │
│   │     RateLimit RateLimitConfig      // 限流业务规则   │     │
│   │ }                                                 │     │
│   └───────────────────────────────────────────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

## 二、Redis 配置对齐

### 当前差异

| 属性 | ncgo (redis.yaml) | go-tools (config/redis/Config) | 统一方案 |
|------|------------------|-------------------------------|---------|
| `addrs` | `[]string` | `address []string` | → `Addrs []string` |
| `db` | `int` | `DB int` | → `DB int` ✅ |
| `username` | `string` | `Username string` | → `Username string` ✅ |
| `password` | `string` | `Password string` | → `Password string` ✅ |
| `pool_size` | `int` | `PoolSize int` | → `PoolSize int` ✅ |
| `min_idle_conns` | `int` | `MinIdleCons int` | → `MinIdleConns int` |
| `dial_timeout` | seconds (int) | 毫秒 (int) | → `DialTimeout time.Duration` |
| `read_timeout` | seconds (int) | 毫秒 (int) | → `ReadTimeout time.Duration` |
| `write_timeout` | seconds (int) | 毫秒 (int) | → `WriteTimeout time.Duration` |
| `master_name` | `string` | ❌ | → `MasterName string` (新增) |
| `sentinel_username` | `string` | ❌ | → `SentinelUsername string` (新增) |
| `sentinel_password` | `string` | ❌ | → `SentinelPassword string` (新增) |
| `protocol` | `int` | ❌ | → `Protocol int` (新增，RESP3) |
| `client_name` | `string` | ❌ | → `ClientName string` (新增) |
| `max_retries` | `int` | `MaxRetries int` | → `MaxRetries int` ✅ |
| `min_retry_backoff` | 毫秒 (int) | 毫秒 (int) | → `MinRetryBackoff time.Duration` |
| `max_retry_backoff` | 毫秒 (int) | 毫秒 (int) | → `MaxRetryBackoff time.Duration` |
| `conn_max_idle_time` | seconds (int) | `MaxConnAge` (毫秒) | → `ConnMaxIdleTime time.Duration` |
| `conn_max_lifetime` | seconds (int) | `IdleTimeout` (毫秒) | → `ConnMaxLifetime time.Duration` |
| `idle_check_frequency` | ❌ | `IdleCheckFrequency` (毫秒) | → `IdleCheckFrequency time.Duration` (保留) |

### 统一结构体

```go
// go-middleware/redis/config.go
package redis

import "time"

type Config struct {
    // 连接配置
    Addrs             []string      `json:"addrs" yaml:"addrs"`                             // Redis 节点地址列表，支持单节点/集群/哨兵，默认 ["127.0.0.1:6379"]
    DB                int           `json:"db" yaml:"db"`                                   // 数据库编号，默认 0
    Username          string        `json:"username" yaml:"username"`                       // 用户名（Redis 6.0+ ACL），默认空
    Password          string        `json:"password" yaml:"password"`                       // 密码，必须通过环境变量 REDIS_PASSWORD 注入
    ClientName        string        `json:"client_name" yaml:"client_name"`                 // 客户端名称，用于 Redis CLIENT LIST 标识，默认空
    Protocol          int           `json:"protocol" yaml:"protocol"`                       // 通信协议版本：2=RESP2 3=RESP3，默认 3

    // 哨兵模式配置（仅哨兵模式需要填写）
    MasterName        string        `json:"master_name" yaml:"master_name"`                 // 哨兵主节点名称，默认空（非哨兵模式不填）
    SentinelUsername  string        `json:"sentinel_username" yaml:"sentinel_username"`     // 哨兵用户名，默认空
    SentinelPassword  string        `json:"sentinel_password" yaml:"sentinel_password"`     // 哨兵密码，必须通过环境变量 REDIS_SENTINEL_PASSWORD 注入

    // 连接池配置
    PoolSize          int           `json:"pool_size" yaml:"pool_size"`                     // 连接池最大连接数，默认 10 * runtime.GOMAXPROCS
    MinIdleConns      int           `json:"min_idle_conns" yaml:"min_idle_conns"`           // 连接池最小空闲连接数，默认 0（不强制维持）
    MaxIdleConns      int           `json:"max_idle_conns" yaml:"max_idle_conns"`           // 连接池最大空闲连接数，默认不限制
    MaxActiveConns    int           `json:"max_active_conns" yaml:"max_active_conns"`       // 最大活跃连接数，0 表示不限制，默认 0

    // 超时配置（均使用 time.Duration，YAML 中写 "5s" "100ms" 等）
    DialTimeout       time.Duration `json:"dial_timeout" yaml:"dial_timeout"`               // 建立 TCP 连接超时，默认 5s
    ReadTimeout       time.Duration `json:"read_timeout" yaml:"read_timeout"`               // 读命令超时，超时后连接关闭，默认 3s
    WriteTimeout      time.Duration `json:"write_timeout" yaml:"write_timeout"`             // 写命令超时，默认等于 ReadTimeout
    PoolTimeout       time.Duration `json:"pool_timeout" yaml:"pool_timeout"`               // 从连接池获取连接的最大等待时间，默认 4s（= ReadTimeout + 1s）

    // 空闲连接管理
    ConnMaxIdleTime   time.Duration `json:"conn_max_idle_time" yaml:"conn_max_idle_time"`   // 连接最大空闲时间，超时关闭，默认 30m
    ConnMaxLifetime   time.Duration `json:"conn_max_lifetime" yaml:"conn_max_lifetime"`     // 连接最大存活时间，超时关闭，默认 0（不限制）
    IdleCheckFrequency time.Duration `json:"idle_check_frequency" yaml:"idle_check_frequency"` // 空闲连接检查频率，默认 1m

    // 重试配置
    MaxRetries        int           `json:"max_retries" yaml:"max_retries"`                 // 命令失败最大重试次数，默认 3
    MinRetryBackoff   time.Duration `json:"min_retry_backoff" yaml:"min_retry_backoff"`     // 重试最小退避时间，默认 8ms
    MaxRetryBackoff   time.Duration `json:"max_retry_backoff" yaml:"max_retry_backoff"`     // 重试最大退避时间，默认 512ms
}

// ToOptions 生成 go-redis 的 UniversalOptions
func (c *Config) ToOptions() *redis.UniversalOptions { ... }
```

## 三、Kitex 服务端配置对齐

### 统一结构体

```go
// go-framework/config/kitex/server.go
package kitex

import "time"

type ServerConfig struct {
    Name                    string        `json:"name" yaml:"name"`                           // 服务名称，必填
    Addr                    string        `json:"addr" yaml:"addr"`                           // 监听地址，格式 host:port，默认 ":8888"
    Network                 string        `json:"network" yaml:"network"`                     // 网络类型：tcp / udp，默认 tcp
    ReadWriteTimeout        time.Duration `json:"read_write_timeout" yaml:"read_write_timeout"` // RPC 读写超时，默认 30s
    ExitWaitTime            time.Duration `json:"exit_wait_time" yaml:"exit_wait_time"`       // 优雅退出等待时间，默认 10s
    RequestTimeout          time.Duration `json:"request_timeout" yaml:"request_timeout"`     // 单次请求超时，默认 3s
    MuxConnections          int           `json:"mux_connections" yaml:"mux_connections"`     // 多路复用连接数，默认 10000
    MaxConnections          int           `json:"max_connections" yaml:"max_connections"`     // 最大连接数，默认 10000
    MaxQPS                  int           `json:"max_qps" yaml:"max_qps"`                     // 最大 QPS 限流，0 表示不限制，默认 0
}

type Registry struct {
    Enable  bool   `json:"enable" yaml:"enable"`     // 是否启用 Polaris 服务注册，默认 false
    Space   string `json:"space" yaml:"space"`       // 注册中心命名空间，默认 "default"
    Env     string `json:"env" yaml:"env"`           // 环境标识：dev / staging / prod，默认 "dev"
    Version string `json:"version" yaml:"version"`   // 服务版本号，默认 "v1.0.0"
}
```

### YAML 示例（统一后）

```yaml
server:
  name: user-service
  addr: :8888
  network: tcp
  read_write_timeout: 30s
  exit_wait_time: 10s
  request_timeout: 3s
  mux_connections: 10000
  max_connections: 10000
  max_qps: 0    # 0 = unlimited

registry:
  enable: false
  space: default
  env: dev
  version: v1.0.0

database:
  enabled: true
  dsn: postgres://user:pass@localhost:5432/db
  max_conns: 20
  min_conns: 2
  max_conn_lifetime: 30m
  max_conn_idle_time: 5m
```

## 四、Kafka 库选择（✅ 已确认）

**决策 D1**：统一为 `kafka-go`，go-tools 现有 `sarama` 实现标记 deprecated。

### 选型对比（已确认）

| 维度 | ncgo (kafka-go) ✅ | go-tools (sarama) ❌ deprecated |
|------|-------------------|-------------------------------|
| 仓库 | `github.com/segmentio/kafka-go` | `github.com/IBM/sarama` |
| 设计风格 | 原生 Go，简洁 API | Java Kafka 客户端迁移，丰富功能 |
| 性能 | 快（低分配） | 功能多但开销高 |
| Producer API | `kafka.Writer` struct | `sarama.SyncProducer` / `AsyncProducer` |
| Consumer API | `kafka.Reader` struct | `sarama.ConsumerGroup` |
| 社区活跃度 | ★★★ 活跃 | ★★★★ 非常活跃（Kafka 官方推荐） |

### go-middleware/kafka 实现
```go
// go-middleware/kafka/producer.go
package kafka

import "github.com/segmentio/kafka-go"

type WriterConfig struct {
    Brokers      []string      `json:"brokers" yaml:"brokers"`               // Kafka broker 地址列表，默认 ["localhost:9092"]
    Topic        string        `json:"topic" yaml:"topic"`                   // 默认 Topic 名称，默认空（需业务侧指定）
    Balancer     string        `json:"balancer" yaml:"balancer"`             // 分区负载均衡策略：round-robin / hash / least-bytes，默认 round-robin
    RequiredAcks int           `json:"required_acks" yaml:"required_acks"`   // 确认机制：0=不等待(RequireNone) 1=leader确认(RequireOne) -1=全部ISR(RequireAll)，默认 0
    Async        bool          `json:"async" yaml:"async"`                   // 是否异步发送，默认 false（同步）
    BatchSize    int           `json:"batch_size" yaml:"batch_size"`         // 单批次最多消息条数，默认 100
    BatchBytes   int64         `json:"batch_bytes" yaml:"batch_bytes"`       // 单批次最大字节数，默认 1048576 (1MB)
    BatchTimeout time.Duration `json:"batch_timeout" yaml:"batch_timeout"`   // 批次逗留时间，超时强制发送，默认 1s
    MaxAttempts  int           `json:"max_attempts" yaml:"max_attempts"`     // 发送失败最大重试次数，默认 10
}

func (c *WriterConfig) Build() *kafka.Writer { ... }
```

## 五、配置加载统一

ncgo 和 go-tools 的配置加载方式需要统一风格：

### 当前

| 项目 | 加载方式 | 错误库 | YAML 库 |
|------|---------|--------|---------|
| ncgo | `os.ReadFile` + `yaml.Unmarshal` + `Validate()` | `oops` | `yaml.v3` |
| go-tools | `os.ReadFile` + `yaml.Unmarshal` | `pkg/errors` | `yaml.v2` |

### 统一后

go-framework 提供标准配置加载器：

```go
// go-framework/config/loader.go
package config

import (
    "os"
    "gopkg.in/yaml.v3"
    "github.com/samber/oops"
)

// LoadYAML loads and validates YAML config from file.
func LoadYAML[T any](path string) (*T, error) {
    content, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, oops.In("config").Code(10308).Public("config_not_found").With("path", path).Wrap(err)
        }
        return nil, oops.In("config").Code(10308).Public("config_read_error").With("path", path).Wrap(err)
    }
    var cfg T
    if err := yaml.Unmarshal(content, &cfg); err != nil {
        return nil, oops.In("config").Code(10308).Public("config_invalid").With("path", path).Wrap(err)
    }
    // 如果 T 实现了 Validator 接口，自动校验
    if v, ok := any(&cfg).(interface{ Validate() error }); ok {
        if err := v.Validate(); err != nil {
            return nil, err
        }
    }
    return &cfg, nil
}
```

统一为 `yaml.v3` + `oops`，废弃 `yaml.v2` 和 `pkg/errors`。

## 六、敏感配置处理

以下字段**不允许**直接硬编码在 YAML 配置文件中，必须通过环境变量注入：

| 字段路径 | 环境变量 | 说明 |
|---------|---------|------|
| `redis.password` | `REDIS_PASSWORD` | Redis 密码 |
| `redis.sentinel_password` | `REDIS_SENTINEL_PASSWORD` | Sentinel 密码 |
| `kafka.sasl.password` | `KAFKA_SASL_PASSWORD` | Kafka SASL 密码 |
| `database.dsn` | `DATABASE_DSN` | 数据库连接串（含密码） |
| `elasticsearch.password` | `ES_PASSWORD` | ES 密码 |
| `clickhouse.password` | `CH_PASSWORD` | ClickHouse 密码 |
| `tls.access_key_id` | `TLS_ACCESS_KEY_ID` | 火山引擎 TLS 密钥 ID |
| `tls.access_key_secret` | `TLS_ACCESS_KEY_SECRET` | 火山引擎 TLS 密钥 Secret |
| `observability.app_key` | `APMPLUS_APP_KEY` | APMPlus 鉴权密钥 |

### 实现方式

Config 结构体在 `Unmarshal` 后自动从环境变量覆盖空值：

```go
// go-framework/config/loader.go
func LoadYAML[T any](path string) (*T, error) {
    content, err := os.ReadFile(path)
    // ... unmarshal ...
    // 自动从环境变量覆盖敏感字段
    overrideFromEnv(&cfg)
    return &cfg, nil
}
```

ncgo 生成的 `.env.example` 模板中预设所有环境变量占位符。

## 七、Elasticsearch 配置结构体

```go
// go-middleware/es/config.go
package es

import "time"

type Config struct {
    // 连接配置
    Addrs       []string      `json:"addrs" yaml:"addrs"`                     // ES 集群节点地址列表，默认 ["http://localhost:9200"]
    Username    string        `json:"username" yaml:"username"`               // 用户名，默认空
    Password    string        `json:"password" yaml:"password"`               // 密码，必须通过环境变量 ES_PASSWORD 注入
    CloudID     string        `json:"cloud_id" yaml:"cloud_id"`               // Elastic Cloud 部署 ID（与 Addrs 二选一），默认空
    APIKey      string        `json:"api_key" yaml:"api_key"`                 // API Key 认证（与 Password 二选一），环境变量注入

    // TLS 配置
    EnableTLS   bool          `json:"enable_tls" yaml:"enable_tls"`           // 是否启用 TLS 加密连接，默认 false
    CAFingerprint string      `json:"ca_fingerprint" yaml:"ca_fingerprint"`   // CA 证书指纹，用于证书 pinning，默认空

    // 超时配置
    DialTimeout       time.Duration `json:"dial_timeout" yaml:"dial_timeout"`           // 建立连接超时，默认 30s
    RequestTimeout    time.Duration `json:"request_timeout" yaml:"request_timeout"`     // 单次请求超时，默认 30s
    MaxRetryTimeout   time.Duration `json:"max_retry_timeout" yaml:"max_retry_timeout"` // 重试最大等待时间，默认 2m
    MaxIdleConnDuration time.Duration `json:"max_idle_conn_duration" yaml:"max_idle_conn_duration"` // 空闲连接最大存活时间，默认 90s

    // 重试配置
    MaxRetries    int           `json:"max_retries" yaml:"max_retries"`       // 失败最大重试次数，默认 3

    // Bulk 批量写入配置
    BulkWorkers   int           `json:"bulk_workers" yaml:"bulk_workers"`     // Bulk 并发 worker 数，默认 1
    BulkActions   int           `json:"bulk_actions" yaml:"bulk_actions"`     // 触发 Bulk 的最小文档数，默认 1000
    BulkSize      int64         `json:"bulk_size" yaml:"bulk_size"`           // 触发 Bulk 的最小字节数，默认 5MB
    FlushInterval time.Duration `json:"flush_interval" yaml:"flush_interval"` // Bulk 刷新间隔，超时强制发送，默认 30s
}

func (c *Config) ToClientConfig() elasticsearch.Config { ... }
```

## 八、ClickHouse 配置结构体

```go
// go-middleware/clickhouse/config.go
package clickhouse

import "time"

type Config struct {
    // 连接配置
    Addrs      []string      `json:"addrs" yaml:"addrs"`           // ClickHouse 节点地址列表，默认 ["localhost:9000"]
    Database   string        `json:"database" yaml:"database"`     // 默认数据库名，默认 "default"
    Username   string        `json:"username" yaml:"username"`     // 用户名，默认 "default"
    Password   string        `json:"password" yaml:"password"`     // 密码，必须通过环境变量 CH_PASSWORD 注入
    Secure     bool          `json:"secure" yaml:"secure"`         // 是否启用 TLS 加密连接，默认 false

    // 连接池配置
    MaxOpenConns    int           `json:"max_open_conns" yaml:"max_open_conns"`       // 最大打开连接数，默认 10
    MaxIdleConns    int           `json:"max_idle_conns" yaml:"max_idle_conns"`       // 最大空闲连接数，默认 5
    ConnMaxLifetime time.Duration `json:"conn_max_lifetime" yaml:"conn_max_lifetime"` // 连接最大存活时间，默认 1h

    // 超时配置
    DialTimeout     time.Duration `json:"dial_timeout" yaml:"dial_timeout"`     // 建立连接超时，默认 30s
    ReadTimeout     time.Duration `json:"read_timeout" yaml:"read_timeout"`     // 读取超时（按 block 生效），默认 5m
    WriteTimeout    time.Duration `json:"write_timeout" yaml:"write_timeout"`   // 写入超时，默认 30s

    // 查询限制
    MaxExecutionTime   time.Duration `json:"max_execution_time" yaml:"max_execution_time"` // 单次查询最大执行时间，默认 0（不限制）
    MaxMemoryUsage     int64         `json:"max_memory_usage" yaml:"max_memory_usage"`     // 单次查询最大内存使用（字节），默认 0（不限制）
    MaxResultRows      int           `json:"max_result_rows" yaml:"max_result_rows"`       // 单次查询最大返回行数，默认 0（不限制）

    // 压缩
    CompressionMethod  string        `json:"compression_method" yaml:"compression_method"` // 压缩算法：lz4 / zstd / none，默认 lz4
}

func (c *Config) ToOptions() *clickhouse.Options { ... }
```

## 九、Kafka 配置结构体（最终版）

已决策统一为 `kafka-go`（D1）。

## 十、TLS 日志服务配置结构体

```go
// go-middleware/tls/config.go
package tls

import "time"

// ProducerConfig 日志生产者配置
type ProducerConfig struct {
    Endpoint        string        `json:"endpoint" yaml:"endpoint"`                     // TLS 服务端点地址
    Region          string        `json:"region" yaml:"region"`                         // 地域，如 cn-beijing
    AccessKeyID     string        `json:"access_key_id" yaml:"access_key_id"`           // 访问密钥 ID，必须通过环境变量注入
    AccessKeySecret string        `json:"access_key_secret" yaml:"access_key_secret"`   // 访问密钥 Secret，必须通过环境变量注入
    TopicID         string        `json:"topic_id" yaml:"topic_id"`                     // 日志主题 ID

    // 批次控制
    MaxBatchSize    int64         `json:"max_batch_size" yaml:"max_batch_size"`         // 单批次最大字节数，默认 512KB
    MaxBatchCount   int           `json:"max_batch_count" yaml:"max_batch_count"`       // 单批次最大日志条数，默认 4096
    LingerTime      time.Duration `json:"linger_time" yaml:"linger_time"`               // 批次逗留时间，超时自动发送，默认 2s
    TotalSizeLnBytes int64        `json:"total_size_ln_bytes" yaml:"total_size_ln_bytes"` // 内存缓存总大小上限，默认 100MB

    // 重试配置
    MaxRetries      int           `json:"max_retries" yaml:"max_retries"`               // 发送失败最大重试次数，默认 10
    RetryBackoff    time.Duration `json:"retry_backoff" yaml:"retry_backoff"`           // 重试间隔时间

    // 压缩
    CompressType    string        `json:"compress_type" yaml:"compress_type"`           // 压缩算法：lz4 / zstd / none，默认 lz4
}
```

## 十一、可观测性统一配置结构体

```go
// go-framework/config/observability.go
package config

// ObservabilityConfig 链路追踪 & 日志统一配置
type ObservabilityConfig struct {
    Enabled     bool    `json:"enabled" yaml:"enabled"`                         // 是否启用可观测性（链路追踪+日志上报）
    Endpoint    string  `json:"endpoint" yaml:"endpoint"`                       // OTLP 数据上报端点，如 apmplus-cn-beijing.volces.com:4317
    AppKey      string  `json:"app_key" yaml:"app_key"`                         // APMPlus 鉴权密钥，必须通过环境变量 APMPLUS_APP_KEY 注入
    ServiceName string  `json:"service_name" yaml:"service_name"`               // 服务名称，用于在 APM 平台标识和检索
    ServiceVersion string `json:"service_version" yaml:"service_version"`       // 服务版本号，如 v1.0.0
    SampleRate  float64 `json:"sample_rate" yaml:"sample_rate"`                 // 链路采样率 0.0-1.0，生产环境建议 0.1（10%）
    Insecure    bool    `json:"insecure" yaml:"insecure"`                       // 是否使用非 TLS 连接，仅开发环境可设为 true

    // TLS 日志上报（可选）
    TLSEnabled  bool    `json:"tls_enabled" yaml:"tls_enabled"`                 // 是否启用火山引擎 TLS 结构化日志上报，默认 false
}
```

### YAML 示例

```yaml
observability:
  enabled: true
  endpoint: apmplus-cn-beijing.volces.com:4317
  app_key: ${APMPLUS_APP_KEY}         # 环境变量注入
  service_name: user-service
  service_version: v1.0.0
  sample_rate: 1.0                    # 全量采样（开发环境）
  insecure: false
  tls_enabled: false                  # 暂不启用日志上报
```

## 十二、日志输出配置结构体

```go
// go-common/log/config.go
package log

// Config 日志配置
type Config struct {
    Path             string `json:"path" yaml:"path"`                           // 日志文件路径，默认 ./log
    MaxSize          int    `json:"max_size" yaml:"max_size"`                   // 单个日志文件最大大小（单位 MB），默认 10
    MaxBackups       int    `json:"max_backups" yaml:"max_backups"`             // 最大保留的旧日志文件数，默认 30
    MaxAge           int    `json:"max_age" yaml:"max_age"`                     // 日志文件最大保留天数，默认 7
    Compress         bool   `json:"compress" yaml:"compress"`                   // 旧日志文件是否 gzip 压缩，默认 true
    OutputMode       int    `json:"output_mode" yaml:"output_mode"`             // 输出模式：1-仅控制台 2-仅文件 3-控制台+文件，默认 3
    Suffix           string `json:"suffix" yaml:"suffix"`                       // 日志文件后缀名，默认 ".log"
    RotationDuration int    `json:"rotation_duration" yaml:"rotation_duration"` // 按时间切割间隔（单位分钟），0 表示仅按文件大小切割，默认 1440（24小时）
    MinSpanLevel     string `json:"min_span_level" yaml:"min_span_level"`       // 写入 OTel Span Event 的最低日志级别，默认 "warn"
    ErrorSpanLevel   string `json:"error_span_level" yaml:"error_span_level"`   // 触发 Span 标记为 Error 的日志级别，默认 "error"
    RecordStack      bool   `json:"record_stack" yaml:"record_stack"`           // 异常日志是否记录调用堆栈，默认 true
}

// DefaultConfig 返回生产环境默认配置
func DefaultConfig() Config {
    return Config{
        Path:             "./log",
        MaxSize:          10,     // 10MB（uptrace 默认值）
        MaxBackups:       30,
        MaxAge:           7,      // 7 天
        Compress:         true,
        OutputMode:       3,
        Suffix:           ".log",
        RotationDuration: 1440,   // 24 小时切割（单位分钟）
        MinSpanLevel:     "warn",
        ErrorSpanLevel:   "error",
        RecordStack:      true,
    }
}
```

### 底层引擎：Go 标准库 `log/slog`

基于 Go 1.21+ `log/slog`，零外部日志依赖。通过自定义 `slog.Handler` 实现文件轮转、级别分流、OTel 联动。

```go
// go-common/log/logger.go
package log

import (
    "context"
    "log/slog"
)

type Logger struct {
    *slog.Logger
    config *Config
}

// New 创建 Logger，自动处理文件轮转 + 压缩 + OTel span 联动
func New(cfg Config) *Logger
```

### slog Handler 架构

```
                    ┌─────────────────────┐
                    │   slog.Logger        │
                    └─────────┬───────────┘
                              │
                    ┌─────────┴───────────┐
                    │   OopsHandler        │  ← 🆕 oops 结构化提取
                    │ (Code/Domain/Public) │
                    └─────────┬───────────┘
                              │
              ┌───────────────┼───────────────┐
              │               │               │
         console          file             otel
         Handler          Handler          Handler
    (text/console)   (JSON + rotate)   (span event)
```

**4 个 Handler 组合**：
1. **OopsHandler** — oops.OopsError 自动提取（Code/Domain/Public/Private/Stack），非 oops 零开销透传
2. **ConsoleHandler** — 控制台输出（开发 `text`，生产 `JSON`）
3. **RotateHandler** — 文件输出（lumberjack 轮转 + gzip 压缩 + 按级别分文件）
4. **OtelHandler** — OTel Span Event 注入（自动 trace_id/span_id + span.SetStatus）

```go
// go-common/log/handler.go — 多 Handler 链式组合
func NewMultiHandler(cfg Config) slog.Handler {
    // 第一层：oops 结构化提取（所有日志必经）
    var handler slog.Handler
    handler = newConsoleOrFileHandler(cfg)
    handler = &OopsHandler{next: handler}  // 包在最内层，所有 Attr 先经过 oops 提取

    // 第二层：OTel span 注入
    handler = &OtelHandler{next: handler, config: cfg}

    return handler
}
```

### OopsHandler 完整实现

```go
// go-common/log/oops_handler.go
type OopsHandler struct { next slog.Handler }

func (h *OopsHandler) Handle(ctx context.Context, r slog.Record) error {
    newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
    r.Attrs(func(attr slog.Attr) bool {
        if err, ok := attr.Value.Any().(error); ok {
            if oopsErr, ok := oops.AsOopsError(err); ok {
                newRecord.AddAttrs(
                    slog.Int("error_code", oopsErr.Code()),
                    slog.String("error_domain", oopsErr.Domain()),
                    slog.String("error_public", oopsErr.Public()),
                    slog.String("error_private", oopsErr.Private()),
                )
                if stack := oopsErr.StackTrace(); stack != "" {
                    newRecord.AddAttrs(slog.String("error_stack", stack))
                }
                newRecord.AddAttrs(attr) // 保留原始 error attr
                return true
            }
        }
        newRecord.AddAttrs(attr) // 非 oops → 原封不动
        return true
    })
    return h.next.Handle(ctx, newRecord)
}
```

**正常日志输出**（无 oops，零开销）：
```json
{"time":"...","level":"INFO","msg":"user login","user_id":12345,"ip":"10.0.1.25"}
```

**oops 错误输出**（自动提取 5 个结构化字段）：
```json
{"time":"...","level":"ERROR","msg":"get user failed",
 "error":"user #20100: user_not_found: query failed",
 "error_code":20100,"error_domain":"user",
 "error_public":"user_not_found","error_private":"userId=12345"}
```

### 文件轮转（lumberjack）

文件轮转仅依赖 `gopkg.in/natefinch/lumberjack.v2`：

```go
// go-common/log/rotate.go
import "gopkg.in/natefinch/lumberjack.v2"

func newRotateWriter(path, level string, cfg Config) io.Writer {
    return &lumberjack.Logger{
        Filename:   path + "/" + level + "/app" + cfg.Suffix,
        MaxSize:    cfg.MaxSize,    // MB
        MaxAge:     cfg.MaxAge,     // 天
        MaxBackups: cfg.MaxBackups,
        Compress:   cfg.Compress,   // gzip
    }
}
```

### 按级别分文件

```
./log/
├── all/app.log       ← DEBUG+ 全量
├── debug/app.log     ← 仅 DEBUG
├── info/app.log      ← 仅 INFO
├── warn/app.log      ← 仅 WARN
├── error/app.log     ← 仅 ERROR
└── fatal/app.log     ← 仅 FATAL
```

### slog 标准 JSON 字段

slog 默认 JSON 输出字段（对齐 Go 标准）：

| 字段 | slog key | 说明 |
|------|---------|------|
| `time` | `slog.TimeKey` | RFC3339 时间戳 |
| `level` | `slog.LevelKey` | DEBUG/INFO/WARN/ERROR |
| `msg` | `slog.MessageKey` | 日志消息 |
| `source` | `slog.SourceKey` | 调用位置 (file:line) |
| `trace_id` | Handler 注入 | OTel trace ID（有 ctx 时自动） |
| `span_id` | Handler 注入 | OTel span ID（有 ctx 时自动） |

### OTel Span 联动

```go
// go-common/log/otel_handler.go
// 自动从 context 提取 OTel span，注入 trace_id/span_id
func (h *otelHandler) Handle(ctx context.Context, r slog.Record) error {
    span := trace.SpanFromContext(ctx)
    if span.SpanContext().IsValid() {
        r.AddAttrs(
            slog.String("trace_id", span.SpanContext().TraceID().String()),
            slog.String("span_id", span.SpanContext().SpanID().String()),
        )
    }
    // ERROR 级别自动标记 span status
    if r.Level >= slog.LevelError {
        span.SetStatus(codes.Error, r.Message)
    }
    return h.next.Handle(ctx, r)
}
```

### ncgo 对齐：klog/hlog 适配器

ncgo 模板生成的项目使用 Kitex `klog` / Hertz `hlog` 原生接口。通过适配器桥接：

```go
// go-common/log/adapters/kitex.go
// 实现 klog.FullLogger 接口，内部委托给 slog
type KitexAdapter struct {
    logger *slog.Logger
}

func NewKitexAdapter(l *Logger) klog.FullLogger {
    return &KitexAdapter{logger: l.Logger}
}
// 在 main.go 中替换：klog.SetLogger(NewKitexAdapter(logger))

// go-common/log/adapters/hertz.go
// 实现 hlog.FullLogger 接口，内部委托给 slog
type HertzAdapter struct {
    logger *slog.Logger
}
func NewHertzAdapter(l *Logger) hlog.FullLogger { ... }
// 在 main.go 中替换：hlog.SetLogger(NewHertzAdapter(logger))
```

**对齐效果**：ncgo 模板中的 `klog.Infof(...)` / `hlog.Debugf(...)` 自动走 slog → JSON 输出，无需修改模板代码。

### YAML 示例

```yaml
# 开发环境：仅控制台
log:
  path: ./log
  max_size: 10
  max_age: 3
  max_backups: 5
  compress: false
  output_mode: 1                    # 仅控制台
  suffix: .log
  rotation_duration: 0              # 仅按 size 轮转
  min_span_level: debug
  error_span_level: error
  record_stack: true

# 生产环境：控制台 + 文件
log:
  path: /var/log/app
  max_size: 100                     # 100MB 切割（生产建议调大，SDK 默认仅 10MB）
  max_age: 7
  max_backups: 30
  compress: true                    # gzip
  output_mode: 3                    # 控制台 + 文件
  suffix: .log
  rotation_duration: 1440           # 24h 切割（分钟）
  min_span_level: warn
  error_span_level: error
  record_stack: true
```

### Kitex Access Log 输出示例

```json
{
  "time": "2026-06-23T15:30:00.123Z",
  "level": "INFO",
  "msg": "rpc_access",
  "source": "middleware/accesslog.go:28",
  "trace_id": "a1b2c3d4e5f6a7b8",
  "span_id": "1a2b3c4d5e6f",
  "method": "UserService/GetUser",
  "duration_ms": 15.2,
  "status_code": 0
}
```

### Hertz Access Log 输出示例

```json
{
  "time": "2026-06-23T15:30:00.456Z",
  "level": "INFO",
  "msg": "http_access",
  "source": "middleware/accesslog.go:15",
  "trace_id": "a1b2c3d4e5f6a7b8",
  "span_id": "1a2b3c4d5e6f",
  "method": "GET",
  "path": "/api/v1/users/12345",
  "duration_ms": 22.5,
  "status_code": 200,
  "body_size": 256,
  "client_ip": "10.0.1.100"
}
```

### 依赖对比

| 依赖 | zap 方案 | slog 方案 |
|------|---------|----------|
| 日志核心 | `go.uber.org/zap` | **标准库 `log/slog`** |
| 文件轮转 | `lumberjack` + `file-rotatelogs` | **仅 `lumberjack`** |
| 框架适配 | 自实现 | **自实现（klog.FullLogger / hlog.FullLogger）** |
| 外部依赖数 | 3 个 | **1 个**（lumberjack） |
