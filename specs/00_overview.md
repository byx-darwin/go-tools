# go-tools 项目总览

> 日期：2026-06-24
> 状态：✅ 三库拆分完成，正在持续优化

## 一、项目定位

**go-tools** 是 Hertz/Kitex 微服务共享基础设施库，提供配置、日志、中间件客户端、错误处理等通用能力。四个独立库（go-common / go-auth / go-middleware / go-framework）按依赖 DAG 组织。

## 二、架构

```text
                    go-common    ← 最底层，零框架依赖
                        ↑          (cache, log, captcha, error, netutil, crypto, httpclient, timeutil)
                     go-auth       ← 认证工具 (JWT, Session/Device)
                    ↑       ↑
          ┌─────────┘       └─────────┐
    go-middleware                  go-framework
    中间件客户端                    框架适配
    (redis, kafka, db,             (hertz, kitex, config,
     es, clickhouse, tls)           observability, accesslog, rpcerror)
```

> 真实拓扑是 **DAG**：`go-framework` 与 `go-middleware` 为**兄弟关系**，二者均依赖 `go-auth` + `go-common`，彼此无依赖。

## 三、各库职责

### go-common

| 包 | 职责 |
|---|------|
| `cache` | 泛型缓存（基于 samber/hot，支持 FIFO/LRU/LFU/CLOCK/MRU + TTL） |
| `captcha` | 验证码生成与校验（图片验证码 + 数字/字母码 + CacheStore） |
| `error` | 统一错误码定义（10000-59999）+ HTTP 状态码映射 |
| `log` | 结构化日志（基于 slog + OTel TraceID/SpanID + 文件轮转） |
| `log/adapters` | Hertz/Kitex 日志适配器 |
| `netutil` | 网络工具（内网 IP 获取） |
| `crypto` | 加密（MD5/SHA/HMAC/TEA） |
| `httpclient` | HTTP 客户端（重试/M3U8） |
| `auth` | AK/SK 生成 |
| `timeutil` | 时间工具 |
| `astutil` | Go AST 操作库（基于 dave/dst，代码生成辅助） |
| `executil` | 增强的命令执行包装器 |
| `templateutil` | 可插拔的模板辅助函数库 |

### go-auth

| 包 | 职责 |
|---|------|
| `jwt` | 泛型 JWT 签发/验证/刷新（基于 golang-jwt/jwt/v5，Sign/Verify/Refresh） |
| `session` | 会话存储契约（Session 结构体 + Store 接口） |
| `device` | 设备会话管理契约（Device 结构体 + Store 接口） |
| `error` | 认证错误码与预定义错误构造器（40000-40099，包名 autherror） |

### go-middleware

| 包 | 职责 |
|---|------|
| `redis` | Redis UniversalClient 工厂（单节点/哨兵，可选 OTel 追踪） |
| `kafka` | Kafka Writer（Producer）/ Consumer（kafka-go） |
| `db` | 数据库客户端工厂（MySQL/PostgreSQL/SQLite） |
| `es` | Elasticsearch 客户端 |
| `clickhouse` | ClickHouse 客户端 |
| `tls` | 火山引擎 TLS 日志上传 |

### go-framework

| 包 | 职责 |
|---|------|
| `config` | 公共配置 + Hertz/Kitex 配置（ServerConfig / ClientConfig / ObservabilityConfig） |
| `hertz/server` | Hertz HTTP 服务工厂 |
| `hertz/middleware` | CORS / Auth / AccessLog 中间件 |
| `hertz/observability` | OTel Tracing + Metrics（OTLP gRPC, 复合 Propagator, runtime metrics） |
| `kitex/option` | Kitex RPC Option 工厂（长连接池 + TTHeaderStreaming） |
| `kitex/rpcerror` | 基于 oops 的统一错误处理 + Kitex BizStatusErrorIface 适配 |
| `kitex/middleware` | AccessLog 中间件 |
| `kitex/observability` | OTel Tracing + Metrics（stats.Tracer, Suite, peer service 拓扑, RED 指标） |

## 四、错误码体系

```
go-framework  10000-10499  ── system/param/auth/config/RPC
  10000 CodeSystem            → HTTP 500
  10001 CodeParamInvalid      → HTTP 400
  10002 CodeAuthFailed        → HTTP 401
  10003 CodeConfigNotFound    → HTTP 500
  10004 CodeConfigInvalid     → HTTP 500
  10010 CodeRPCUnavailable    → HTTP 503
  10011 CodeRPCTimeout        → HTTP 504
  10012 CodeRPCDecodeError    → HTTP 500
  10013 CodeRPCEncodeError    → HTTP 500

go-middleware  20000-20699  ── redis/kafka/db/es/ch/tls/obs
  20001-20005  Redis      → HTTP 500/503
  20101-20105  Kafka      → HTTP 500/503
  20201-20204  DB         → HTTP 500/503
  20301-20302  ES         → HTTP 500/503
  20401-20402  ClickHouse → HTTP 500/503
  20501-20502  TLS        → HTTP 500/503
  20601-20602  Obs        → HTTP 500/503

项目业务       40000-59999  ── HTTP 200（RPC 调用成功）
  40010-40012  数据（NotFound/Duplicate/Conflict）
  40110-40113  认证（LoginFailed/TokenExpired/TokenInvalid/PermissionDenied）
  40210-40212  限制（RateLimit/QuotaExceeded/IPBlocked）
  40310-40314  状态（AccountDisabled/OrderInvalid/BalanceInsufficient/VerificationFailed/OperationDenied）
```

详见 `go-common/error/error.go` 和 `go-framework/kitex/rpcerror/error.go`。

## 五、关键技术决策

| 决策 | 结论 |
|------|------|
| 缓存库 | `samber/hot` 泛型 HotCache |
| 错误库 | `samber/oops` 为主体 |
| 日志库 | Go 标准 `log/slog` + OTel TraceID/SpanID |
| Kafka | `kafka-go` |
| 配置时间格式 | `time.Duration`（YAML: `30s`） |
| 构造函数 | Functional Options 模式 |

## 六、开发规范

- `.claude/rules/go.md` — Go 编码规范
- `.claude/rules/agent-engineering.md` — Agent 执行规范
- `.claude/rules/options-pattern.md` — Options 模式规范

## 七、Spec 文档

| 文件 | 内容 |
|------|------|
| `00_overview.md` | 本文档 |
| `01_split_plan_summary.md` | 三库拆分方案（已完成） |
| `02_config_schema_alignment.md` | 配置结构体参考 |
| `03_implementation_phases.md` | 实施阶段（已完成） |
| `04_cache_algorithm_guide.md` | 缓存算法选择指南 |
| `05_migration_guide.md` | 旧模块迁移指南 |
| `06_kitex_observability_migration.md` | obs-opentelemetry (Kitex) 迁移评估与实施方案 ✅ |
| `07_hertz_observability_migration.md` | obs-opentelemetry (Hertz) 迁移评估与实施方案 ✅ |
