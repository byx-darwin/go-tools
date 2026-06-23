# go-tools

Go 微服务工具库 — Hertz / Kitex 项目基础设施。

## 架构

```
go-common          ← 零框架依赖工具库
    ↑
go-middleware      ← 中间件客户端（Redis / Kafka / DB / ES / CH / TLS）
    ↑
go-framework       ← Hertz / Kitex 框架适配（Config / Server / Option / Observability）
```

## 模块

| 模块 | 用途 | 独立模块 |
|------|------|---------|
| [go-common](./go-common) | 通用工具：crypto / cache / log / netutil / timeutil / httpclient / captcha / auth | `go get gitee.com/byx_darwin/go-tools/go-common` |
| [go-middleware](./go-middleware) | 中间件客户端：redis / kafka / db / es / clickhouse / tls | `go get gitee.com/byx_darwin/go-tools/go-middleware` |
| [go-framework](./go-framework) | 框架适配：hertz / kitex / config / observability / accesslog / rpcerror | `go get gitee.com/byx_darwin/go-tools/go-framework` |

## 快速开始

```go
// 日志
logger := log.New(log.Config{Level: "info", FilePath: "/var/log/app.log"})
logger.Info("server started", "port", 8080)

// 缓存
c := cache.New[string, int](cache.LRU, 100).Build()
c.Set("key", 42)

// Redis
client, closeFn, _ := redis.NewUniversalClient(ctx, &redis.Config{
    Addrs: []string{"localhost:6379"},
})

// 可观测 — 全链路
// go-common/log → go-middleware/tls → 火山引擎 TLS
// go-framework/*/observability → OTLP gRPC → Jaeger
```

## 版本要求

Go 1.25+

## 迁移

从旧 go-tools 迁移见 [specs/07_migration_guide.md](./specs/07_migration_guide.md)
