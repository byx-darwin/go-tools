# go-tools

Go microservice toolkits — Hertz / Kitex project infrastructure.

## Architecture

```
                go-common    ← Zero-dependency utility libraries
                    ↑
                 go-auth      ← Auth utilities (JWT / Session / Device)
                ↑       ↑
      ┌─────────┘       └─────────┐
go-middleware                  go-framework
Middleware clients             Hertz / Kitex framework adapters
(Redis / Kafka / DB /          (Config / Server / Option /
 ES / CH / TLS)                 Observability)
```

> Real topology is a **DAG**: `go-framework` and `go-middleware` are **siblings** — both depend on `go-auth` + `go-common`, and neither depends on the other.

## Modules

| Module | Purpose | Install |
|--------|--------|---------|
| [go-common](./go-common) | Utilities: crypto / cache / log / netutil / timeutil / httpclient / captcha / auth / templateutil / executil / astutil | `go get github.com/byx-darwin/go-tools/go-common` |
| [go-middleware](./go-middleware) | Middleware clients: redis / kafka / db / es / clickhouse / tls | `go get github.com/byx-darwin/go-tools/go-middleware` |
| [go-framework](./go-framework) | Framework adapters: hertz / kitex / config / observability / accesslog / rpcerror | `go get github.com/byx-darwin/go-tools/go-framework` |

## Quick Start

```go
// Logging — enhanced structured logging
log.Init(log.Config{
    Level:  "info",
    Format: "json",
    Mode:   "both",
    File: log.FileConfig{
        Dir:      "/var/log/app",
        Filename: "app.log",
        MaxSize:  100,
    },
    Masking: log.MaskConfig{
        Enabled:      true,
        MaskedFields: []string{"password", "token"},
    },
}, log.ReleaseInfo{
    ServiceName: "user-service",
    Version:     "v1.0.0",
    Environment: "production",
})
defer log.Close()

// Use with categories
accessLog := log.L().WithCategory(log.CategoryAccess)
accessLog.InfoContext(ctx, "request handled", "method", "GET", "path", "/api/users")

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

golangci-lint v2 (>= v2.12.2) — 用于本地静态分析（CI 已启用）。安装/升级：

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

## Migration

See [specs/05_migration_guide.md](./specs/05_migration_guide.md) for migrating from legacy go-tools.
