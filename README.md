# go-tools

Go microservice toolkits — Hertz / Kitex project infrastructure.

## Architecture

```
go-common          ← Zero-dependency utility libraries
    ↑
go-middleware      ← Middleware clients (Redis / Kafka / DB / ES / CH / TLS)
    ↑
go-framework       ← Hertz / Kitex framework adapters (Config / Server / Option / Observability)
```

## Modules

| Module | Purpose | Install |
|--------|--------|---------|
| [go-common](./go-common) | Utilities: crypto / cache / log / netutil / timeutil / httpclient / captcha / auth | `go get github.com/byx-darwin/go-tools/go-common` |
| [go-middleware](./go-middleware) | Middleware clients: redis / kafka / db / es / clickhouse / tls | `go get github.com/byx-darwin/go-tools/go-middleware` |
| [go-framework](./go-framework) | Framework adapters: hertz / kitex / config / observability / accesslog / rpcerror | `go get github.com/byx-darwin/go-tools/go-framework` |

## Quick Start

```go
// Logging
logger := log.NewFromConfig(log.Config{Level: "info", FilePath: "/var/log/app.log"})
logger.Info("server started", "port", 8080)

// Cache
c := cache.New[string, int](cache.LRU, 100).Build()
c.Set("key", 42)

// Redis
client, closeFn, _ := redis.NewUniversalClient(ctx, &redis.Config{
    Addrs: []string{"localhost:6379"},
})
defer closeFn()

// Observability — end-to-end
// go-common/log → structured logging with OTel trace context
// go-middleware/tls → Volcengine TLS log shipping
// go-framework/*/observability → OTLP gRPC → Jaeger
```

## Requirements

Go 1.25+

## Migration

See [specs/05_migration_guide.md](./specs/05_migration_guide.md) for migrating from legacy go-tools.
