# ncgo 模板生成内容 vs go-tools 已有能力 — 逐项对照

> 本文档列出 ncgo 所有模板中"内嵌完整实现"的部分，以及 go-tools 中已有的对应能力。

## 一、Kitex 模板对照

### 1.1 配置加载 — `internal/base/conf/conf.go`

| 维度 | ncgo 模板 (`conf.yaml`) | go-tools 已有 (`config/`) |
|------|------------------------|--------------------------|
| ServerConfig | ✅ 内嵌 `Name/Addr/ReadWriteTimeout/ExitWaitTime` | ✅ `config/kitex.ServerConfig` 有 `RPC.Port/Network` + `Timeout.ReadWriteTimeout/ExitWaitTimeout` |
| DatabaseConfig | ✅ 内嵌 `DSN/MaxConns/MinConns/...` | ✅ `config/db.Config` 存在 |
| RateLimitConfig | ✅ 完整内嵌 (~100 行结构体) | ❌ go-tools 无 rate-limit 配置 |
| AuthConfig | ✅ CallerAllowlist | ❌ go-tools 无对应 |
| Redis (rate-limit) | ✅ `RateLimitRedisConfig` | ⚠️ `config/redis.Config` 存在但字段命名不同 |
| YAML 加载逻辑 | ✅ `Init()/Get()/Load()/Default()/Validate()` | ⚠️ `config/polaris.go` 有配置加载但逻辑不同 |

**差异点**：
- ncgo 用 `oops` 做错误包装，go-tools 用 `pkg/errors`
- ncgo 用 `yaml.v3` + `samber/oops`，go-tools 用 `yaml.v2` + `pkg/errors`
- 配置字段命名风格不同：ncgo 用 `read_write_timeout_seconds`，go-tools 用 `read_write_timeout`（统一使用 `time.Duration`）

### 1.2 拦截器 — `internal/pkg/interceptor/interceptor.go`

| 维度 | ncgo 模板 | go-tools 已有 |
|------|----------|--------------|
| RequestID | ✅ 自实现 (16字节随机 + hex) | ❌ 无 |
| AccessLog | ✅ 自实现 (klog + rpcinfo) | ❌ 无 |
| Recovery | ✅ 自实现 (panic recover) | ❌ 无 |
| RequestTimeout | ✅ 自实现 (context.WithTimeout) | ❌ 无 |
| CallerAllowlist | ✅ 自实现 (header 校验) | ❌ 无 |

**结论**：ncgo 的 interceptor 是完整自实现的，go-tools 没有对应。**建议在 go-framework 中新建 `kitex/middleware/` 包提供这些中间件。**

### 1.3 RPC 错误 — `internal/pkg/rpcerror/rpcerror.go`

| 维度 | ncgo 模板 | go-tools 已有 (`kitex/rpcerror/error.go`) |
|------|----------|-------------------------------------------|
| 错误码体系 | `oops` 风格 (Code + Public msg) | `ErrorType` 枚举 (iota) |
| 转换逻辑 | `ToBizError` (oops → BizStatusError) | `NewBizStatusError` / `ParseBizStatusError` |
| 错误码常量 | `CodeInternalError=10000` 等 | `ErrorTypeInvalid=0` 起 |
| 格式化 | `FormatBiz` + `BizCode` | `ParseBizStatusError` 返回 ErrorType + msg |

**差异点**：
- ncgo 用 `oops.In("kitex.server").Code(x).Public("y").Errorf(...)` 构建错误
- go-tools 用 `ErrorType` 枚举 + 字符串拼接
- 两套体系的错误码不兼容

**建议**：统一到 go-framework 的 `kitex/rpcerror/` 包，采用 `oops` 风格（更灵活），保留 go-tools 的错误类型枚举作为辅助。

### 1.4 Server 入口 — `internal/base/server/server.go`

| 维度 | ncgo 模板 | go-tools 已有 (`kitex/option/server.go`) |
|------|----------|----------------------------------------|
| 服务地址解析 | ✅ 自实现 `net.ResolveTCPAddr` | ✅ `option.NewServerOption()` 已实现 |
| 服务注册 | ❌ 不内嵌（注释提示 canary） | ✅ Polaris 注册 `registry/polaris.go` |
| 中间件链 | ✅ RequestID/AccessLog/Recovery/CallerAllowlist/Timeout | ❌ 无 |
| DI 容器 | ✅ `samber/do` | ❌ 不用 DI 容器 |
| 数据库连接 | ✅ pgx pool + sqlc | ❌ 无 |

### 1.5 Handler — `internal/handler/<service>/handler.go`

| 维度 | ncgo 模板 | go-tools 已有 |
|------|----------|--------------|
| Handler struct + usecase 接口 | ✅ | ❌ 无 |
| 委托模式 (handler → usecase) | ✅ | ❌ 无 |

**结论**：这是架构模式，不是可复用库代码。保持模板生成。

## 二、Hertz 模板对照

### 2.1 响应工具 — `internal/pkg/response/`

| 维度 | ncgo 模板 (layout.yaml 内嵌) | go-tools 已有 (`hertz/response.go`) |
|------|---------------------------|------------------------------------|
| 响应结构 | `Response{Code, Msg, Data}` | `Response{Code, Msg, Data}` ✅ 一致 |
| 成功/失败 | `OK()/Err()/BindError()` | `Result()/ReplyWithOk()/ReplyWithErr()` |
| 错误码 | `CodeNotImplemented` 等常量 | `SUCCESS=200/ERROR=500/RELOGIN=302` |
| oops 集成 | ✅ | ❌ 用 fmt + rpc_error |
| i18n 支持 | ✅ (可选) | ❌ |

**差异点**：
- go-tools 的 response 更老派（直接传 format + logger）
- ncgo 的 response 用 oops + 结构化错误
- ncgo 有 i18n 支持，go-tools 没有

**建议**：在 go-framework 中统一为 ncgo 风格的 response（oops + i18n），同时兼容 go-tools 的简单调用方式。

### 2.2 Redis add-on — `optional/redis.go` + `optional/redis_shared.go`

| 维度 | ncgo 模板 | go-tools 已有 (`middleware/redis/redis.go`) |
|------|----------|-------------------------------------------|
| 客户端类型 | `redis.UniversalClient` (go-redis/v9) | `*redis.Client` (go-redis/v9) |
| 配置来源 | 项目内 `Config.Redis` (ncgo 自定义) | `config/redis.Config` (go-tools) |
| OTel tracing | ❌ | ✅ `redisotel.InstrumentTracing` |
| 连接验证 | ✅ `Ping()` | ✅ `Ping()` |
| 错误处理 | `oops` | `pkg/errors` |

**差异点**：
- ncgo 用 `UniversalClient`（支持单节点/集群/sentinel），go-tools 用 `*redis.Client`
- ncgo 配置字段更多（`master_name/sentinel_*/protocol` 等），go-tools 配置偏基础
- ncgo 用 `oops`，go-tools 用 `pkg/errors`

### 2.3 Kafka add-on — `optional/kafka.go`

| 维度 | ncgo 模板 | go-tools 已有 (`middleware/kafka/sarama/producer.go`) |
|------|----------|----------------------------------------------------|
| 库选择 | `segmentio/kafka-go` | `IBM/sarama` (Shopify sarama) |
| 包装 | `KafkaWriter/KafkaReader` struct | `Producer` struct |
| 配置 | 项目内 Config (yaml) | `config/kafka/sarama.Config` |

**差异点**：
- **库选择**：已决策统一为 `kafka-go`（D1），go-tools 现有 `sarama` 实现标记 deprecated

### 2.4 其他 Hertz 可选模板

| ncgo 模板 | go-tools 对应 | 对齐建议 |
|-----------|--------------|----------|
| `optional/clickhouse.go` | ❌ 无 | go-tools 不覆盖，保持模板 |
| `optional/es.go` | ❌ 无 | go-tools 不覆盖，保持模板 |
| `optional/observability_logging.go` | ⚠️ 通过 OTel 标准协议 | go-framework 统一提供 observability 中间件 + go-middleware/tls 日志客户端 + go-common/log 结构化日志库 |
| `optional/release_canary.go` | ❌ 无 | 保持模板（架构级功能） |
| `optional/rule_center_client.go` | ❌ 无 | 保持模板 |
| `optional/redis_shared.go` | ⚠️ 部分覆盖 | go-tools 需增强 |

## 二点五、可观测性对齐（新增）

ncgo 目前有独立的 `observability_logging.go` 可选模板，go-tools 有 Redis OTel tracing。双方需要统一为**标准 OpenTelemetry 协议**：

| 框架 | 集成包 | 接入方式 |
|------|--------|---------|
| Kitex | `kitex-contrib/obs-opentelemetry` | `tracing.NewServerSuite()` / `tracing.NewClientSuite()` |
| Hertz | `hertz-contrib/obs-opentelemetry` | `hertztracing.ServerMiddleware()` / `hertztracing.ClientMiddleware()` |

数据上报通过 OTLP 协议发送到 APMPlus / Jaeger / Grafana 等后端，不锁定厂商。

### 日志输出格式

ncgo 生成的日志输出统一使用 `go-common/log` 结构化 JSON 格式，标准字段：

| 字段 | 说明 | 来源 |
|------|------|------|
| `timestamp` | RFC3339 时间戳 | go-common/log |
| `level` | DEBUG/INFO/WARN/ERROR/FATAL | go-common/log |
| `service` | 服务名称 | 启动配置 |
| `message` | 日志内容 | 业务代码 |
| `trace_id` | OTel trace ID | observability 中间件自动注入 |
| `request_id` | 请求 ID | interceptor 中间件自动注入 |
| `duration_ms` | 请求耗时 | access log 中间件 |
| `status_code` | 响应码 | access log 中间件 |

access log 中间件由 go-framework 提供（`kitex/middleware/accesslog.go` 和 `hertz/middleware/accesslog.go`），自动从 rpcinfo/request context 提取字段。

## 三、ncgo 独有（go-tools 无对应）

以下功能仅存在于 ncgo 模板中，go-tools 完全没有：

| 功能 | 说明 | 建议 |
|------|------|------|
| Rate Limit (全链路) | ~500 行代码：database-backed + gRPC + memory/redis backend | 考虑抽到 go-middleware 或独立库 |
| Caller Allowlist | Header-based 服务间调用鉴权 | 抽到 go-framework |
| i18n 支持 | 国际化 payload | 保持 ncgo 独有 |
| SQLC 集成 | sqlc.yaml + migration + schema | 保持 ncgo 独有（脚手架功能） |
| Canary/灰度发布 | Traffic metadata 传播 | 保持模板 |
| Rule Center Client | 规则中心客户端 | 保持模板 |
| samber/do DI 模式 | 依赖注入 wiring | 保持模板（架构模式） |
| ncgo:wire 锚点 | 用于 `ncgo add infra` 注入代码 | ncgo 独有 |

## 四、go-tools 独有（ncgo 未引用）

| go-tools 能力 | ncgo 是否应该引用 |
|--------------|-----------------|
| `tools/crypto/` (MD5/SHA/HMAC/TEA) | ✅ 生成的项目可能需要加密工具 |
| `tools/cache/` (FIFO/LRU/LFU/CLOCK/MRU) | ✅ rate-limit 可以用 LFU 缓存 |
| `tools/http_client/` (fasthttp + retry) | ⚠️ 可选 |
| `tools/netutil/` (GetInternalIP) | ✅ go-tools 的 server 工厂已在用 |
| `tools/time/` (格式化/月份) | ⚠️ 可选 |
| `tools/captcha/` | ⚠️ 可选 |
| `tools/ak.go` (AK/SK 生成) | ✅ hertz auth middleware 需要 |
| `tools/entutils/` | ⚠️ 可选 |
| `hertz/middleware/auth.go` (AK/SK 认证) | ✅ 应该引用 |
| `hertz/middleware/casbin.go` | ⚠️ 可选 |
| `hertz/middleware/cors.go` | ✅ 应该引用 |
| `hertz/middleware/location.go` | ⚠️ 可选 |
| `hertz/registry/polaris/` | ✅ 应该引用 |
| `kitex/option/` (server/client option 工厂) | ✅ 应该引用 |
| `kitex/discover/polaris/` | ✅ 应该引用 |
| `config/polaris.go` | ✅ 应该引用 |

## 五、配置 YAML schema 对照

### Redis

| ncgo 字段 (optional-config/redis.yaml) | go-tools 字段 (config/redis/config.go) | 对齐建议 |
|---------------------------------------|---------------------------------------|----------|
| `addrs: []string` | `address: []string` | 统一为 `addrs` |
| `db: int` | `db: int` | ✅ 一致 |
| `username: string` | `username: string` | ✅ 一致 |
| `password: string` | `password: string` | ✅ 一致 |
| `master_name: string` | ❌ 无 | go-tools 需增加（支持 sentinel） |
| `pool_size: int` | `pool_size: int` | ✅ 一致 |
| `min_idle_conns: int` | `min_idle_conns: int` | ✅ 一致（修复拼写：旧 go-tools 字段为 `min_idle_cons`，统一为 `min_idle_conns`） |
| `dial_timeout_seconds: int` | `dial_timeout: time.Duration` | 统一为 `time.Duration`（D2，YAML 写 `30s`） |
| `read_timeout_seconds: int` | `read_timeout: time.Duration` | 统一为 `time.Duration`（D2） |
| `protocol: int` | ❌ 无 | go-tools 需增加（支持 RESP3） |

### Kafka

| ncgo 字段 (kafka-go) | go-tools 字段 (sarama) | 对齐建议 |
|---------------------|----------------------|----------|
| `brokers: []string` | 类似 | ✅ |
| `topic: string` | 类似 | ✅ |
| `group_id: string` | 类似 | ✅ |
| **库完全不同** | kafka-go vs sarama | **需要决策统一** |
