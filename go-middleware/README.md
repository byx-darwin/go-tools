# go-middleware

中间件客户端库。**go-tools 三层架构的中间层**，不依赖 Hertz/Kitex 框架。

依赖 `go-common`。

## 安装

```bash
go get github.com/byx-darwin/go-tools/go-middleware
```

## 包一览

| 包 | 说明 |
|----|------|
| `redis` | Redis 客户端（支持 Sentinel，UniversalClient，OTel 追踪） |
| `kafka` | Kafka 生产者和消费者（基于 `github.com/segmentio/kafka-go`） |
| `db` | 数据库配置 + 连接池工厂 |
| `es` | Elasticsearch v8 客户端 |
| `clickhouse` | ClickHouse 原生协议客户端（基于 `clickhouse-go/v2`；含包内错误码 20401-20403） |
| `tls` | 火山引擎日志服务（Producer + FileShipper；含包内错误码 20501-20504） |

## 配置对齐

所有时间配置统一使用 `time.Duration`，与 ncgo 模板 YAML `30s` 格式兼容。

## 可观测链路

```
go-common/log   →  本地 JSON 日志文件
go-middleware/tls  →  日志上报火山引擎 TLS
go-framework/*/observability  →  OTel Traces → Jaeger
```

详见 [tls/README.md](tls/README.md)。

## 依赖

- `go-common`
- `github.com/redis/go-redis/v9`
- `github.com/segmentio/kafka-go`
- `github.com/elastic/go-elasticsearch/v8`
- `github.com/ClickHouse/clickhouse-go/v2`
- `github.com/volcengine/volc-sdk-golang/service/tls`
