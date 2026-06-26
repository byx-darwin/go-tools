# go-tools Example — 全功能验证项目

本示例项目旨在全功能验证 go-tools 所有模块（go-common、go-auth、go-middleware、go-framework）
的集成可用性，同时作为新项目的参考实现。

| 模块 | 地位 | 验证内容 |
|------|------|---------|
| `go-common` | 零框架依赖，纯工具库 | 加密、缓存、验证码、日志、错误码、网络、时间模板等 13 条路由 |
| `go-auth` | 认证工具库 | JWT 签发/验证/刷新、Session 管理、设备管理，含受保护路由 |
| `go-middleware` | 中间件客户端 | Redis、Kafka、DB、ES、ClickHouse 连接与操作 |
| `go-framework` | 框架适配层 | Config 加载/热重载/环境变量展开、Hertz OTel、Kitex RPC |

---

## Quick Start

```bash
make run              # 启动 HTTP (:8080) + RPC (:8888) 服务
make test             # 运行测试（local 模式，内存存储，跳过中间件）
make report           # 生成测试报告 test/report.md
```

服务启动后访问 `http://localhost:8080/health` 确认健康状态。

---

## Prerequisites

- **Go 1.25+** — 必须，workspace 模式依赖
- **make** — 用于执行 Makefile 命令
- **Docker**（可选）— 运行中间件集成测试时需要

---

## Configuration

配置位于 `config.yaml`，采用 YAML 格式，支持 `${VAR}` 环境变量展开。

```yaml
server:
  http_addr: ":8080"
  rpc_addr: ":8888"

store_mode: "memory"        # memory 或 redis — 控制 Session/Device 存储后端
```

`store_mode: memory` 适用于本地开发，无需外部依赖。
`store_mode: redis` 适用于集成测试或生产部署。

完整配置参见 [config.yaml](./config.yaml)。

---

## Docker Services

```bash
make docker-up        # 启动 Redis、MySQL、Kafka、ES、ClickHouse
make full-test        # 启动容器 → 等待就绪 → 运行完整测试 → 停止容器
make docker-down      # 停止所有容器
```

Docker Compose 服务定义（[docker-compose.yml](./docker-compose.yml)）：

| 服务 | 端口 |
|------|------|
| Redis 7 | `6379` |
| MySQL 8 | `3306` |
| Kafka | `9092` |
| Elasticsearch 8.12 | `9200` |
| ClickHouse | `9000` |

---

## Route List

### Health

| Method | Path | Handler |
|--------|------|---------|
| `GET` | `/health` | 健康检查，返回 `{"status":"ok"}` |

### go-common（13 条路由）

| Method | Path | 功能 |
|--------|------|------|
| `GET` | `/common/crypto` | MD5、SHA256、SHA512、HMAC、TEA 加密演示 |
| `GET` | `/common/cache` | LRU / LFU 缓存操作演示 |
| `GET` | `/common/captcha` | 生成图形验证码（返回 ID + Base64 图片） |
| `POST` | `/common/captcha/verify` | 校验验证码 |
| `GET` | `/common/log` | 结构化日志分类输出（Info / Warn / Error / Debug） |
| `GET` | `/common/error` | 预定义错误码 + 错误范围查询 |
| `GET` | `/common/httpclient` | HTTP 客户端 Retry 机制演示 |
| `GET` | `/common/netutil` | 本机内网 IP、网络连通性检测 |
| `GET` | `/common/timeutil` | 时间格式化、时区转换、半年区间计算 |
| `GET` | `/common/template` | Go 模板引擎渲染演示 |
| `GET` | `/common/executil` | 命令执行（stdout / exit_code） |
| `GET` | `/common/astutil` | Go 源码 AST 解析（导出函数、导入声明） |
| `GET` | `/common/aksk` | AK/SK 生成 + HMAC 签名 |

### go-auth（7 条路由）

| Method | Path | 功能 |
|--------|------|------|
| `POST` | `/auth/jwt/sign` | 签发 Access + Refresh Token |
| `POST` | `/auth/jwt/sign-device` | 签发携带设备声明的 JWT |
| `POST` | `/auth/jwt/verify` | 校验 JWT 有效性 |
| `POST` | `/auth/jwt/refresh` | 使用 Refresh Token 换取新 Access Token |
| `POST` | `/auth/session` | 创建 Session |
| `GET` | `/auth/session` | 查询 Session（`X-Session-Id` 请求头） |
| `DELETE` | `/auth/session` | 删除 Session |
| `POST` | `/auth/device` | 注册设备（支持踢出策略） |
| `GET` | `/auth/device` | 查询用户设备列表 |
| `DELETE` | `/auth/device` | 移除设备 |

### Protected Routes（受保护路由 — 需认证中间件）

| Method | Path | 认证方式 | 功能 |
|--------|------|---------|------|
| `GET` | `/protected/jwt` | `Authorization: Bearer <token>` | 返回 JWT claims 中的用户信息 |
| `GET` | `/protected/session` | `X-Session-Id` 请求头 | 返回 Session 用户信息 |
| `GET` | `/protected/device` | `Authorization: Bearer <device-token>` | JWT + Device 双因子 |

未认证时返回 `401`。

### Middleware Clients（5 条路由）

| Method | Path | 功能 |
|--------|------|------|
| `GET` | `/middleware/redis` | Redis SET / GET / DEL |
| `POST` | `/middleware/kafka` | Kafka 生产消息 |
| `GET` | `/middleware/kafka` | Kafka 消费消息 |
| `GET` | `/middleware/db` | 数据库 Ping |
| `GET` | `/middleware/es` | Elasticsearch 集群健康 |
| `GET` | `/middleware/clickhouse` | ClickHouse `SELECT 1` |

### Config（4 条路由）

| Method | Path | 功能 |
|--------|------|------|
| `GET` | `/config/load` | 查看当前配置（机密字段已脱敏） |
| `GET` | `/config/duration` | Duration 解析演示（`30s` / `24h`） |
| `POST` | `/config/hot-reload` | 热重载配置 |
| `GET` | `/config/polaris` | 北极星配置中心状态 |

### RPC（Hertz → Kitex，2 条路由）

| Method | Path | 功能 |
|--------|------|------|
| `GET` | `/rpc/echo?message=xxx` | 通过 Kitex 客户端调用 DemoService.Echo RPC |
| `GET` | `/rpc/health` | 通过 Kitex 客户端调用 DemoService.Health RPC |

---

## Environment Variables

`config.yaml` 通过 `os.ExpandEnv` 展开 `${VAR}` 格式的环境变量：

| 变量 | 用途 | 默认值 |
|------|------|--------|
| `OTEL_ENDPOINT` | OTel 导出地址 | `http://localhost:4318` |
| `OTEL_APP_KEY` | OTel 应用密钥 | `dev-key` |

其他中间件地址可直接在 `config.yaml` 中修改，或通过环境变量覆写。

---

## Test Modes

```bash
go run ./test/ -mode local        # 内存存储，跳过中间件测试（默认）
go run ./test/ -mode docker       # 通过 Docker 启动真实中间件服务
go run ./test/ -mode local -report  # 运行本地测试并生成 report.md
```

| 模式 | Session/Device 后端 | 中间件测试 | 要求 |
|------|-------------------|-----------|------|
| `local` | 内存 | 跳过 | 无 |
| `docker` | 内存 | 跳过 | Docker runtime |

---

## Manual Verification Checklist

- [ ] **APM Tracing** — 检查 OTel 导出是否正常（`observability.enabled=true` 时）
- [ ] **Log Masking** — 观察 `/common/log` 响应，password 字段应显示 `***`
- [ ] **Hot Reload** — 修改 `config.yaml` 后 `POST /config/hot-reload`，确认配置已更新
- [ ] **Polaris Config Center** — 设置 `polaris.enabled=true`，重启后 `GET /config/polaris`
- [ ] **Environment Variable Expansion** — 设置 `OTEL_ENDPOINT` 后观察 `/config/load` 中的展开结果
- [ ] **JWT Auth Middleware** — 无 Token 请求 `/protected/jwt` 返回 401，有效 Token 返回 200
- [ ] **Session Auth Middleware** — 无 Session-Id 请求 `/protected/session` 返回 401
- [ ] **Device Auth Middleware** — 使用普通 JWT 请求 `/protected/device` 被拒绝（缺少 device claims）
- [ ] **RPC Round-Trip** — `GET /rpc/echo?message=hello` 返回 Kitex 服务响应
- [ ] **CAPTCHA Chain** — POST `/common/captcha/verify` 使用前一步 GET 返回的 id + answer 验证
- [ ] **Kafka Produce + Consume** — POST 生产消息后 GET 消费（需 Docker）
- [ ] **Test Report** — `make report` 生成 `test/report.md` 包含分类汇总和耗时统计

---

## Project Structure

```text
example/
├── main.go                     # 入口，Hertz + Kitex 启动 + 可观测性初始化
├── config.go                   # AppConfig 类型定义 + YAML 加载 + 环境变量展开
├── config.yaml                 # 全局配置（含环境变量引用）
├── go.mod / go.sum             # 模块依赖
├── Makefile                    # 构建/测试/容器管理命令
├── docker-compose.yml          # Redis / MySQL / Kafka / ES / ClickHouse
│
├── handler/                    # HTTP 路由处理器（按 go-tools 模块分类）
│   ├── common_crypto.go
│   ├── common_cache.go
│   ├── common_captcha.go
│   ├── common_log.go
│   ├── common_error.go
│   ├── common_httpclient.go
│   ├── common_netutil.go
│   ├── common_timeutil.go
│   ├── common_template.go
│   ├── common_executil.go
│   ├── common_astutil.go
│   ├── common_aksk.go
│   ├── auth_jwt.go
│   ├── auth_session.go
│   ├── auth_device.go
│   ├── auth_protected.go
│   ├── middleware_redis.go
│   ├── middleware_kafka.go
│   ├── middleware_db.go
│   ├── middleware_es.go
│   ├── middleware_ch.go
│   ├── config_handler.go
│   └── rpc_echo.go
│
├── middleware/                 # Hertz 全局中间件 + 受保护路由注册
│   └── setup.go
│
├── rpc/                        # Kitex RPC 服务端 + 客户端 + 服务实现
│   ├── client.go
│   ├── server.go
│   └── service.go
│
├── idl/                        # Protobuf IDL 定义
│   └── demo.proto
│
├── kitex_generated/            # Kitex 代码生成产物
│   └── demo/
│       ├── demo.pb.go
│       └── demoservice/
│
└── test/                       # 端到端测试运行器
    ├── runner.go               # 测试引擎（HTTP 请求 + 断言 + 依赖管理）
    ├── cases.go                # 36+ 个测试用例定义
    └── report.go               # HTML/Markdown 报告生成
```

---

## 设计要点

- **Config 先行** — 服务启动前完成配置加载和日志初始化，确保后续组件可依赖统一日志
- **Dependency Injection** — Session Store、Device Store、RPC Client 通过 Setter 注入，解耦初始化与路由注册
- **OTel 可选** — 可观测性初始化失败时仅 Warn 不退出，保证本地开发零依赖
- **Test Runner** — 400 行自研测试引擎，支持分类/依赖链/变量传递/跳过条件/Markdown 报告
- **环境变量展开** — `os.ExpandEnv` 一行代码实现，支持 `${VAR}` 语法，无需第三方库
