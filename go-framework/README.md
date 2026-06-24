# go-framework

Hertz / Kitex 框架适配层。**go-tools 三层架构的最顶层**。

依赖 `go-common`。

## 安装

```bash
go get github.com/byx-darwin/go-tools/go-framework
```

## 包一览

| 包 | 说明 |
|----|------|
| `config` | 通用配置类型（Registry / Jaeger / Duration）+ YAML 加载器（`LoadYAML[T]`）+ 可观测配置 |
| `config/hertz` | Hertz HTTP 服务配置 |
| `config/kitex` | Kitex RPC 服务端/客户端配置 |
| `hertz` | 统一 JSON 响应（`OK` / `Err`）+ HTTP 服务工厂 |
| `hertz/middleware` | CORS / Auth（HMAC-SHA256 签名鉴权） / AccessLog |
| `hertz/observability` | OTel 链路追踪 Provider（OTLP gRPC 导出） |
| `kitex/middleware` | RPC AccessLog 中间件 |
| `kitex/middleware/compat` | kitex `endpoint.Middleware` 类型转换 |
| `kitex/observability` | OTel 链路追踪 Provider（OTLP gRPC 导出） |
| `kitex/option` | ServerOption / ClientOption 工厂（限流/超时/多路复用/负载均衡/重试） |
| `kitex/rpcerror` | 错误码定义（19 种 ErrorType）+ BizStatusError（oops 兼容） |

## 配置对齐

- 所有时间字段统一为 `time.Duration`（D2 决策）
- 配置文件 YAML 格式：`30s` / `5m` / `1h`
- 可观测配置：`ObservabilityConfig` 统一入口

## 依赖

- `go-common`
- `github.com/cloudwego/hertz`
- `github.com/cloudwego/kitex`
- `github.com/polarismesh/polaris-go`
- `go.opentelemetry.io/otel/*`（OTLP gRPC 导出）
