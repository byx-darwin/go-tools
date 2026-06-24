# 迁移指南：go-tools v1 → v2（三库拆分）

## 概览

旧 5 模块已废弃，新项目统一使用三库：

```text
旧模块（已删除）                   新库
───────────────────────────       ──────────────
config/{db,redis,kafka,hertz,kitex}  → go-middleware + go-framework/config
hertz/{server,response,middleware} → go-framework/hertz
kitex/{option,registry,discover,rpc_error} → go-framework/kitex
middleware/{redis,kafka}           → go-middleware
tools/{crypto,cache,http_client,...} → go-common
```

## 导入路径对照表

| 旧路径 | 新路径 |
|--------|--------|
| `gitee.com/byx_darwin/go-tools/tools/crypto` | `gitee.com/byx_darwin/go-tools/go-common/crypto` |
| `gitee.com/byx_darwin/go-tools/tools/cache` | `gitee.com/byx_darwin/go-tools/go-common/cache` |
| `gitee.com/byx_darwin/go-tools/tools/time` | `gitee.com/byx_darwin/go-tools/go-common/timeutil` |
| `gitee.com/byx_darwin/go-tools/tools/netutil` | `gitee.com/byx_darwin/go-tools/go-common/netutil` |
| `gitee.com/byx_darwin/go-tools/tools/http_client` | `gitee.com/byx_darwin/go-tools/go-common/httpclient` |
| `gitee.com/byx_darwin/go-tools/tools/captcha` | `gitee.com/byx_darwin/go-tools/go-common/captcha` |
| `gitee.com/byx_darwin/go-tools/tools/ak.go` | `gitee.com/byx_darwin/go-tools/go-common/auth` |
| `gitee.com/byx_darwin/go-tools/tools/entutils` | 已废弃 |
| `gitee.com/byx_darwin/go-tools/config/redis` | `gitee.com/byx_darwin/go-tools/go-middleware/redis` |
| `gitee.com/byx_darwin/go-tools/config/db` | `gitee.com/byx_darwin/go-tools/go-middleware/db` |
| `gitee.com/byx_darwin/go-tools/config/kafka/sarama` | `gitee.com/byx_darwin/go-tools/go-middleware/kafka` |
| `gitee.com/byx_darwin/go-tools/middleware/redis` | `gitee.com/byx_darwin/go-tools/go-middleware/redis` |
| `gitee.com/byx_darwin/go-tools/middleware/kafka/sarama` | `gitee.com/byx_darwin/go-tools/go-middleware/kafka` |
| `gitee.com/byx_darwin/go-tools/config` | `gitee.com/byx_darwin/go-tools/go-framework/config` |
| `gitee.com/byx_darwin/go-tools/config/hertz` | `gitee.com/byx_darwin/go-tools/go-framework/config/hertz` |
| `gitee.com/byx_darwin/go-tools/config/kitex` | `gitee.com/byx_darwin/go-tools/go-framework/config/kitex` |
| `gitee.com/byx_darwin/go-tools/hertz` | `gitee.com/byx_darwin/go-tools/go-framework/hertz` |
| `gitee.com/byx_darwin/go-tools/hertz/middleware` | `gitee.com/byx_darwin/go-tools/go-framework/hertz/middleware` |
| `gitee.com/byx_darwin/go-tools/kitex/option` | `gitee.com/byx_darwin/go-tools/go-framework/kitex/option` |
| `gitee.com/byx_darwin/go-tools/kitex/rpc_error` | `gitee.com/byx_darwin/go-tools/go-framework/kitex/rpcerror` |
| `gitee.com/byx_darwin/go-tools/kitex/registry/polaris` | `gitee.com/byx_darwin/go-tools/go-framework/kitex/option` (合并) |

## 关键变更

### 1. 缓存 API

```go
// 旧
c := core.NewFifoCache[string, string](100)  // 每种算法一个构造函数

// 新
c := cache.New[string, string](cache.LRU, 100).Build()  // 统一 builder
c.Set("k", "v")
v, ok, _ := c.Get("k")
```

### 2. 时间单位

```go
// 旧 — int 毫秒
type Config struct {
    DialTimeout int `yaml:"dial_timeout"` // 单位ms
}

// 新 — time.Duration，YAML 写 "30s"
type Config struct {
    DialTimeout time.Duration `yaml:"dial_timeout"`
}
```

### 3. Redis 客户端

```go
// 旧
client := redis.NewRedisClient(ctx, cfg, true) // *redis.Client

// 新
client, closeFn, err := redis.NewUniversalClient(ctx, cfg) // UniversalClient
```

### 4. Kafka 库

```go
// 旧 — sarama
producer := sarama.NewKafkaProducer(opt) // IBM/sarama

// 新 — kafka-go
writer := kafka.NewWriter(cfg) // segmentio/kafka-go
```

### 5. 日志

```go
// 新 — go-common/log
l := log.New(log.Config{Level: "info", FilePath: "/var/log/app.log"})
l.Info("msg", "key", "value")
```

## 添加依赖

```bash
# 项目 go.mod
require (
    gitee.com/byx_darwin/go-tools/go-common v0.1.0
    gitee.com/byx_darwin/go-tools/go-middleware v0.1.0
    gitee.com/byx_darwin/go-tools/go-framework v0.1.0
)
```
```

### 6. TLS 日志上报（新增）

```go
import "gitee.com/byx_darwin/go-tools/go-middleware/tls"
import "gitee.com/byx_darwin/go-tools/go-common/log"

// 方式一：直接发送
p, _ := tls.NewProducer(tls.ProducerConfig{
    Endpoint: "tls-cn-beijing.volces.com", Region: "cn-beijing",
    AccessKeyID: os.Getenv("VOLC_AK"), AccessKeySecret: os.Getenv("VOLC_SK"),
    TopicID: "xxx-xxx-xxx",
})
defer p.Close()
p.SendLog(ctx, map[string]string{"level": "info", "msg": "hello"})

// 方式二：文件自动上报（配合 go-common/log）
logger := log.New(log.Config{FilePath: "/var/log/app.log", JSON: true})
shipper, _ := tls.NewFileShipper(tls.FileShipperConfig{
    ProducerConfig: producerConfig,
    FilePath:       "/var/log/app.log",
})
shipper.Start()
defer shipper.Close()
```
