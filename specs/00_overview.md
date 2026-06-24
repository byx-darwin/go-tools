# go-tools 项目总览

> 日期：2026-06-24
> 状态：✅ 三库拆分完成，正在持续优化

## 一、项目定位

**go-tools** 是 Hertz/Kitex 微服务共享基础设施库，提供配置、日志、中间件客户端、错误处理等通用能力。三个独立库按依赖层级组织。

## 二、架构

```text
go-common          ← 最底层，零框架依赖
    ↑                 (cache, log, captcha, errcode, netutil, crypto, httpclient, timeutil)
go-middleware       ← 中间件客户端
    ↑                 (redis, kafka, db, es, clickhouse, tls)
go-framework        ← 框架适配
                      (hertz, kitex, config, observability, accesslog, rpcerror)
```

## 三、各库职责

### go-common

| 包 | 职责 |
|---|------|
| `cache` | 泛型缓存（基于 samber/hot，支持 FIFO/LRU/LFU/CLOCK/MRU + TTL） |
| `captcha` | 验证码生成与校验（图片验证码 + 数字/字母码 + 缓存存储） |
| `errcode` | 统一错误码定义（10000-59999）+ HTTP 状态码映射 |
| `log` | 结构化日志（基于 slog + OTel TraceID/SpanID 自动关联 + 文件轮转） |
| `netutil` | 网络工具（内网 IP 获取等） |
| 其他 | crypto, httpclient, timeutil, auth |

### go-middleware

| 包 | 职责 |
|---|------|
| `redis` | Redis UniversalClient 工厂（单节点/哨兵，可选 OTel 追踪） |
| `kafka` | Kafka Writer（生产者）/ Consumer（消费者） |
| `db` | 数据库客户端工厂（MySQL/PostgreSQL/SQLite） |
| `es` | Elasticsearch 客户端 |
| `clickhouse` | ClickHouse 客户端 |
| `tls` | 火山引擎 TLS 日志上传 |

### go-framework

| 包 | 职责 |
|---|------|
| `config` | 公共配置类型 + Hertz/Kitex 配置（ServerConfig / ClientConfig / CaptchaOption 等） |
| `hertz/server` | Hertz HTTP 服务工厂 |
| `hertz/middleware` | CORS / Auth / AccessLog 中间件 |
| `kitex/option` | Kitex RPC 服务端/客户端 Option 工厂（长连接池 + TTHeaderStreaming） |
| `kitex/rpcerror` | 基于 oops 的统一错误处理 + Kitex BizStatusErrorIface 适配 |
| `kitex/middleware` | AccessLog 中间件 |

## 四、错误码体系

```
go-framework  10000-10499  ── system/param/auth/config/RPC    HTTP: 400/401/500/503/504
go-middleware  20000-20699  ── redis/kafka/db/es/ch/tls/obs   HTTP: 500/503
项目业务       40000-59999  ── 数据/认证/限制/状态            HTTP: 200
```

详见 `go-common/errcode/code.go` 和 `go-framework/kitex/rpcerror/error.go`。

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
