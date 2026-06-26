# go-tools 全功能验证 Example 设计

- 日期: 2026-06-26
- 状态: 待实现

## 1. 目标

创建一个单一综合 example 项目，验证 go-tools 四层库（go-common / go-auth / go-middleware / go-framework）的全部功能。包含：

- Hertz HTTP 服务 + Kitex RPC 服务
- 全量包功能演示（27+ 个 handler）
- 鉴权中间件专项测试（JWT / Session / Device）
- 自动化测试套件 + 测试报告（含 Bug 定位信息）
- 混合运行模式（本地内存默认 + docker-compose 可选真实服务）
- 配置热更新验证 + Polaris 配置中心
- 商用 APM 可观测性集成

## 2. 项目结构

```
example/
├── go.mod                    # module: github.com/byx-darwin/go-tools/example
├── main.go                   # 入口：读取配置 → 初始化依赖 → 启动 Hertz + Kitex
├── config.yaml               # 示例配置文件
├── docker-compose.yml        # 可选外部服务（Redis, Kafka, ES, ClickHouse, MySQL）
├── Makefile                  # make run / test / report / docker-up / docker-down
├── idl/
│   └── demo.proto            # Kitex Protobuf 服务定义
├── kitex_generated/          # kitex 生成代码（提交到 git）
│   └── demo/
├── handler/                  # HTTP 路由 handlers（按包分组）
│   ├── common_crypto.go      # go-common/crypto
│   ├── common_cache.go       # go-common/cache
│   ├── common_captcha.go     # go-common/captcha
│   ├── common_log.go         # go-common/log
│   ├── common_error.go       # go-common/error
│   ├── common_httpclient.go  # go-common/httpclient
│   ├── common_netutil.go     # go-common/netutil
│   ├── common_timeutil.go    # go-common/timeutil
│   ├── common_template.go    # go-common/templateutil
│   ├── common_executil.go    # go-common/executil
│   ├── common_astutil.go     # go-common/astutil
│   ├── common_aksk.go        # go-common/auth (AK/SK)
│   ├── auth_jwt.go           # go-auth/jwt
│   ├── auth_session.go       # go-auth/session + go-middleware/auth
│   ├── auth_device.go        # go-auth/device + go-middleware/auth
│   ├── auth_protected.go     # 中间件保护路由（鉴权专项测试）
│   ├── middleware_redis.go   # go-middleware/redis
│   ├── middleware_kafka.go   # go-middleware/kafka
│   ├── middleware_db.go      # go-middleware/db
│   ├── middleware_es.go      # go-middleware/es
│   ├── middleware_ch.go      # go-middleware/clickhouse
├── middleware/
│   └── setup.go              # Hertz 中间件注册
├── rpc/
│   ├── server.go             # Kitex server 初始化（log adapter + observability）
│   ├── client.go             # Kitex client 初始化（Hertz→Kitex 调用）
│   └── service.go            # Kitex handler（echo + rpcerror 演示）
├── test/
│   ├── runner.go             # 测试执行器
│   ├── report.go             # 报告生成器（Markdown + 终端彩色输出）
│   ├── cases.go              # 测试用例定义
│   └── testdata/             # 测试用固定数据
└── README.md
```

### go.mod

- 使用 `replace` 指令引用本地 `../go-common`, `../go-auth`, `../go-middleware`, `../go-framework`
- 依赖：Hertz、Kitex、kafka-go、go-redis、polaris-go 等
- `go.work` 需新增 `./example` 目录

### main.go 职责

1. 加载 `config.yaml`
2. 初始化 log、cache、session store、device store（根据 store_mode 选择 memory 或 Redis）
3. 启动 Hertz HTTP server（默认 `:8080`）
4. 启动 Kitex RPC server（默认 `:8888`）
5. 优雅关闭

## 3. Hertz HTTP 路由

### 3.1 go-common 功能演示

| 路由 | 方法 | 演示功能 | 依赖 |
|------|------|---------|------|
| `/common/crypto` | GET | MD5/SHA/HMAC/AES/TEA 加解密 | go-common/crypto |
| `/common/cache` | GET | LRU/LFU/FIFO cache 操作 | go-common/cache |
| `/common/captcha` | GET | 验证码生成 + 缓存存储 | go-common/captcha |
| `/common/log` | GET | 结构化日志、分类日志、mask 脱敏 | go-common/log |
| `/common/error` | GET | oops 错误码包装、错误链 | go-common/error |
| `/common/httpclient` | GET | HTTP 请求、重试机制、m3u8 | go-common/httpclient |
| `/common/netutil` | GET | IP 获取、网络工具 | go-common/netutil |
| `/common/timeutil` | GET | 时间格式化 | go-common/timeutil |
| `/common/template` | GET | 模板渲染 | go-common/templateutil |
| `/common/executil` | GET | Shell 命令执行 | go-common/executil |
| `/common/astutil` | GET | AST 解析/格式化 | go-common/astutil |
| `/common/aksk` | GET | AK/SK 签名验证 | go-common/auth |

### 3.2 鉴权功能演示

| 路由 | 方法 | 演示功能 | 依赖 |
|------|------|---------|------|
| `/auth/jwt/sign` | POST | JWT 签发（返回 access_token + refresh_token） | go-auth/jwt |
| `/auth/jwt/verify` | POST | JWT 验证（有效/过期/无效 token） | go-auth/jwt |
| `/auth/jwt/refresh` | POST | JWT 刷新（用 refresh_token 换新 access_token） | go-auth/jwt |
| `/auth/session` | POST/GET/DELETE | Session 创建/读取/销毁 | go-auth/session + go-middleware/auth |
| `/auth/device` | POST/GET/DELETE | Device 注册/查询/注销 | go-auth/device + go-middleware/auth |

### 3.3 鉴权中间件保护路由（专项测试）

| 路由 | 中间件 | 测试场景 |
|------|--------|---------|
| `/protected/jwt` | JWT Auth | 无 token → 401；过期 token → 401 + 错误码 40001；有效 token → 200 |
| `/protected/session` | Session Auth | 无 session → 401；有效 session → 200 |
| `/protected/device` | Device Auth | 无效设备 → 401；有效设备 → 200 |

### 3.4 中间件客户端演示

| 路由 | 方法 | 演示功能 | 依赖 |
|------|------|---------|------|
| `/middleware/redis` | GET | Redis 操作（SET/GET/DEL） | go-middleware/redis |
| `/middleware/kafka` | POST/GET | Kafka 发送/消费消息 | go-middleware/kafka |
| `/middleware/db` | GET | DB 连接/查询 | go-middleware/db |
| `/middleware/es` | GET | ES 索引/搜索 | go-middleware/es |
| `/middleware/clickhouse` | GET | ClickHouse 查询 | go-middleware/clickhouse |

### 3.5 配置验证

| 路由 | 方法 | 演示功能 | 依赖 |
|------|------|---------|------|
| `/config/load` | GET | LoadYAML 加载所有配置结构体 | go-framework/config |
| `/config/hot-reload` | GET | 修改 config.yaml → 重新加载 → 对比前后值 | go-framework/config |
| `/config/polaris` | GET | Polaris 远程配置拉取 + ChangeListener | go-framework/config/polaris |
| `/config/duration` | GET | config.Duration 解析验证（"30s" → time.Duration） | go-framework/config |

### 3.6 RPC + 系统

| 路由 | 方法 | 演示功能 | 依赖 |
|------|------|---------|------|
| `/rpc/echo` | GET | Hertz→Kitex RPC 调用 | go-framework/kitex option |
| `/metrics` | GET | Prometheus 格式指标 | go-framework/hertz/observability |
| `/health` | GET | 健康检查 | — |

## 4. Hertz 中间件（全局注册）

| 中间件 | 来源 | 演示点 |
|--------|------|--------|
| JWT Auth | go-framework/hertz/middleware | `/protected/jwt` 受保护路由 |
| Session Auth | go-framework/hertz/middleware | `/protected/session` 受保护路由 |
| Device Auth | go-framework/hertz/middleware | `/protected/device` 受保护路由 |
| AccessLog | go-framework/hertz/middleware | 所有请求结构化日志 |
| CORS | go-framework/hertz/middleware | 跨域配置 |
| HertzRequestID | go-framework/hertz/log | 请求 ID 注入 |

## 5. Kitex RPC 服务

### IDL（Protobuf）

简单 `DemoService`：
- `Echo(EchoRequest) → EchoResponse` — 回声测试，展示 rpcerror 错误码
- `Health(HealthRequest) → HealthResponse` — 健康检查

### 重点验证（编码集成）

- `rpc/server.go`：Kitex log adapter 替换默认日志、observability suite 接入、option 配置、accesslog 中间件
- `rpc/client.go`：Kitex client 端日志 + observability，Hertz 调用 Kitex 的链路
- `rpc/service.go`：简单 echo handler，展示 rpcerror 错误码映射

## 6. 鉴权专项测试

### 6.1 JWT 测试用例

| 用例 | 验证点 |
|------|--------|
| `auth/jwt-sign` | POST `/auth/jwt/sign` → 返回 access_token + refresh_token |
| `auth/jwt-verify-valid` | POST `/auth/jwt/verify` 带有效 token → 200 + claims |
| `auth/jwt-verify-expired` | POST `/auth/jwt/verify` 带过期 token → 错误码 40001 (token expired) |
| `auth/jwt-verify-invalid` | POST `/auth/jwt/verify` 带篡改 token → 错误码 40002 (token invalid) |
| `auth/jwt-refresh` | POST `/auth/jwt/refresh` 带 refresh_token → 新 access_token |
| `auth/jwt-middleware-no-token` | GET `/protected/jwt` 不带 Authorization → 401 |
| `auth/jwt-middleware-expired` | GET `/protected/jwt` 带过期 token → 401 + 错误码 40001 |
| `auth/jwt-middleware-valid` | GET `/protected/jwt` 带有效 token → 200 |

### 6.2 Session 测试用例

| 用例 | 验证点 |
|------|--------|
| `auth/session-create` | POST `/auth/session` → 返回 session_id |
| `auth/session-get` | GET `/auth/session` 带 session_id → 返回数据 |
| `auth/session-delete` | DELETE `/auth/session` → session 销毁，再查返回 404 |
| `auth/session-middleware-no-session` | GET `/protected/session` 无 session → 401 |
| `auth/session-middleware-valid` | GET `/protected/session` 带有效 session → 200 |
| `auth/session-expire` | 等待过期后查询 → session 不存在 |

### 6.3 Device 测试用例

| 用例 | 验证点 |
|------|--------|
| `auth/device-register` | POST `/auth/device` → 返回 device_token |
| `auth/device-query` | GET `/auth/device` → 返回设备列表 |
| `auth/device-deactivate` | DELETE `/auth/device` → 设备注销，再查返回 401 |
| `auth/device-middleware-invalid` | GET `/protected/device` 无效设备 → 401 |
| `auth/device-middleware-valid` | GET `/protected/device` 有效设备 → 200 |
| `auth/device-max-limit` | 注册超过 maxDevices → 错误码 40010+ |

### 6.4 错误码全量覆盖

遍历 `go-auth/error` 中定义的全部错误码（40000-40099），确保每个错误码在对应场景下正确返回：

| 错误码 | 场景 | 预期 |
|--------|------|------|
| 40000 | token missing | 请求无 Authorization 头 |
| 40001 | token expired | 使用过期 token |
| 40002 | token invalid | 使用篡改 token |
| 40003 | token refresh failed | 使用无效 refresh_token |
| ... | ... | 遍历全部 |

## 7. Observability 集成

### 环境变量（商用 APM）

```bash
OTEL_ENDPOINT=your-apm-otlp-endpoint:4317   # 商用 APM OTLP 接收地址
OTEL_APP_KEY=your-apm-secret-key             # 商用 APM 应用密钥
POLARIS_ADDRESS=127.0.0.1:8091               # Polaris 地址（可选）
```

### 日志链路

```
请求 → HertzRequestID → AccessLog(含 trace_id)
                        → handler → log.InfoContext(ctx, ...)
                                    → slog handler chain:
                                       CategoryHandler
                                       → ReleaseInfoHandler (service_name, version)
                                       → ContextHandler (trace_id from OTel ctx)
                                       → MaskHandler (password, token 脱敏)
                                       → MultiHandler (console + file)
```

### 验证分工

| 验证类型 | 范围 | 方式 |
|---------|------|------|
| **自动化** | 代码集成正确性 | `make test` |
| — | 中间件注册成功、不 panic | 启动时检查 |
| — | 路由返回 200/预期 JSON | HTTP 断言 |
| — | 日志格式正确（含 trace_id 字段） | 解析日志断言 |
| — | 请求头 trace context 传播 | 检查响应头 |
| — | rpcerror 错误码映射 | 断言错误码 |
| — | JWT/Session/Device 中间件拦截正确 | 401/200 断言 |
| **人工** | 外部平台数据合规 | README 提供检查步骤 |
| — | APM 平台查看完整 Hertz→Kitex 链路 | 按 trace_id 搜索 |
| — | APM 平台查看 HTTP/RPC 指标 | 检查 dashboard |
| — | 日志文件脱敏是否生效 | 查看日志文件 |
| — | Polaris 配置推送是否生效 | 修改 Polaris 配置后观察 |

## 8. 配置与热更新

### config.yaml

```yaml
server:
  http_addr: ":8080"
  rpc_addr: ":8888"

log:
  level: "info"
  format: "json"
  mode: "both"
  file:
    dir: "./logs"
    filename: "example.log"
    max_size: 100
  masking:
    enabled: true
    masked_fields: ["password", "token", "secret"]

jwt:
  secret: "example-secret-key"
  issuer: "go-tools-example"
  access_expiration: 30s
  refresh_expiration: 24h

store_mode: "memory"  # memory 或 redis

redis:
  addrs: ["${REDIS_ADDR:-localhost:6379}"]
  db: 0
  password: ""

kafka:
  brokers: ["${KAFKA_BROKERS:-localhost:9092}"]
  topic: "go-tools-demo"

db:
  dsn: "${MYSQL_DSN:-root:root@tcp(localhost:3306)/demo}"

elasticsearch:
  addresses: ["${ES_ADDRESSES:-http://localhost:9200}"]

clickhouse:
  dsn: "${CH_DSN:-clickhouse://default:@localhost:9000/demo}"

captcha:
  key_long: 6
  img_width: 240
  img_height: 80
  cache_length: 1024
  cache_expires_time: 120s

hertz:
  registry:
    enable: false
    space: "default"
    name: "example-service"
  http:
    network: "tcp"
    port: ":8080"
    exit_wait_time: 10s
    idle_timeout: 60s
    is_cors: true
    is_recovery: true
  auth:
    enable: true
    ak: "example-ak"
    sk: "example-sk"
    tea_key: "0123456789abcdef"

kitex:
  rpc:
    port: ":8888"
    network: "tcp"
  limit:
    enable: false
    max_connections: 1000
    max_qps: 500
  timeout:
    read_write_timeout: 5s
    exit_wait_timeout: 10s

observability:
  enabled: true
  endpoint: "${OTEL_ENDPOINT}"
  app_key: "${OTEL_APP_KEY}"
  service_name: "go-tools-example"
  sample_rate: 1.0
  enable_metrics: true
  metrics_interval: 15s

polaris:
  enabled: false
  namespace: "go-tools-example"
  file_group: "example"
  file_name: "config.yaml"
```

### 热更新验证

两种机制均需验证：

**1. 手动热更新（`/config/hot-reload` 端点）**：
```
1. 记录当前配置快照（如 jwt.access_expiration = 30s）
2. 程序修改 config.yaml（jwt.access_expiration: 30s → 60s）
3. POST /config/hot-reload → 服务端重新 LoadYAML 并应用
4. GET /config/load → 断言运行时配置已更新（60s）
5. 恢复原 config.yaml
```

**2. 文件监听热更新（可选，依赖 fsnotify 或轮询）**：
```
1. 启动时注册 config.yaml 文件监听
2. 外部修改 config.yaml
3. 服务端自动 reload → 运行时配置更新
4. 断言更新生效
```

> 注：手动热更新为必须实现；文件监听为可选增强（如果 go-framework/config 已支持则直接验证，否则仅验证手动模式）。

## 9. docker-compose.yml

```yaml
services:
  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]

  mysql:
    image: mysql:8
    environment:
      MYSQL_ROOT_PASSWORD: root
      MYSQL_DATABASE: demo
    ports: ["3306:3306"]

  kafka:
    image: apache/kafka:latest
    ports: ["9092:9092"]

  elasticsearch:
    image: elasticsearch:8.12.0
    environment:
      discovery.type: single-node
      xpack.security.enabled: "false"
    ports: ["9200:9200"]

  clickhouse:
    image: clickhouse/clickhouse-server:latest
    ports: ["9000:9000"]
```

无 Jaeger/Prometheus，observability 直接上报商用 APM。

## 10. 自动化测试套件

### 测试用例结构

```go
type TestCase struct {
    Name     string            // "go-common/crypto"
    Method   string            // "GET"
    Path     string            // "/common/crypto"
    Body     any               // POST 请求体
    Headers  map[string]string // 请求头（如 Authorization）
    Assert   AssertFunc        // 自定义断言函数
    SkipIf   SkipCondition     // 条件跳过（如 "kafka not configured"）
}
```

### 执行流程

测试运行器为独立进程（`test/runner.go`），通过 `go run ./test/...` 启动：

```
make test:
1. runner 在后台启动 example server（go run .）
2. 轮询 /health 等待服务就绪
3. 依次调用所有端点（含鉴权中间件测试）
4. 对比预期响应（状态码 + 关键字段 + 错误码）
5. 生成测试报告（终端 + Markdown）
6. 关闭 server，输出退出码（0=全部通过，1=有失败）
```

### 报告格式

**终端输出**：
```
=== go-tools 功能验证报告 ===
时间: 2026-06-26 22:00:00
模式: 本地内存 (memory)

[PASS] go-common/crypto      — MD5/SHA/AES/TEA 加解密正常
[PASS] go-common/cache       — LRU/LFU/FIFO 操作正常
[FAIL] go-common/httpclient  — m3u8 解析异常: expected 3 segments, got 2
[PASS] go-auth/jwt           — Sign/Verify/Refresh 正常
[PASS] go-auth/session       — MemoryStore CRUD 正常
[PASS] go-auth/middleware    — JWT 中间件拦截正确（无 token→401, 有效→200）
...

结果: 35/37 通过, 2 失败, 0 跳过
耗时: 5.2s
```

**Markdown 报告**（`test/report.md`）：
```markdown
# go-tools 功能验证报告
- 日期: 2026-06-26
- 模式: memory / docker
- 通过: 35/37

## 失败项
| 模块 | 端点 | 实际 | 预期 | 错误分类 | 建议修复 |
|------|------|------|------|---------|---------|
| go-common/httpclient | /common/httpclient | got 2 segments | 3 segments | 代码 bug | 检查 m3u8 parser |
| go-middleware/kafka | /middleware/kafka | 连接超时 | 200 OK | 依赖未启动 | 运行 docker-compose |
```

### Bug 修复流程

测试报告 `[FAIL]` 项输出：
- **实际响应** vs **预期响应**
- **错误分类**：代码 bug / 配置缺失 / 依赖未启动
- **建议修复方向**

开发者根据报告定位代码，修复后重新 `make test` 验证。

## 11. 错误处理

所有 handler 统一使用 `go-common/error` + `oops` 风格：

```go
// 成功响应
c.JSON(http.StatusOK, response.Success(data))

// 错误响应 — 带错误码
err := oops.Code(40001).Msg("token expired")
c.JSON(http.StatusUnauthorized, response.Error(err))
```

错误码覆盖：
- `go-framework`: 10000-10499（system, param, auth, config, RPC middleware）
- `go-middleware`: 20000-20699（redis, kafka, db, es, clickhouse, observability）
- `go-auth`: 40000-40099（token, session, device auth errors）

## 12. Makefile

```makefile
.PHONY: run test report docker-up docker-down full-test gen-kitex

gen-kitex:
	kitex -module github.com/byx-darwin/go-tools/example -I idl idl/demo.proto

run:
	go run .

test:
	go test ./test/... -count=1 -v

report:
	go run ./test/... -report

docker-up:
	docker compose up -d

docker-down:
	docker compose down

full-test: docker-up
	@sleep 5
	go run ./test/... -report -mode=docker
	docker compose down
```

## 13. 全量覆盖检查清单

| 层 | 包 | handler 文件 | 覆盖 |
|----|-----|-------------|------|
| go-common | crypto | common_crypto.go | ✅ |
| go-common | cache | common_cache.go | ✅ |
| go-common | captcha | common_captcha.go | ✅ |
| go-common | log | common_log.go | ✅ |
| go-common | error | common_error.go | ✅ |
| go-common | httpclient | common_httpclient.go | ✅ |
| go-common | netutil | common_netutil.go | ✅ |
| go-common | timeutil | common_timeutil.go | ✅ |
| go-common | templateutil | common_template.go | ✅ |
| go-common | executil | common_executil.go | ✅ |
| go-common | astutil | common_astutil.go | ✅ |
| go-common | auth | common_aksk.go | ✅ |
| go-auth | jwt | auth_jwt.go | ✅ |
| go-auth | session | auth_session.go | ✅ |
| go-auth | device | auth_device.go | ✅ |
| go-auth | error | auth_protected.go（错误码全量遍历） | ✅ |
| go-middleware | auth | auth_session.go + auth_device.go | ✅ |
| go-middleware | redis | middleware_redis.go | ✅ |
| go-middleware | kafka | middleware_kafka.go | ✅ |
| go-middleware | db | middleware_db.go | ✅ |
| go-middleware | es | middleware_es.go | ✅ |
| go-middleware | clickhouse | middleware_ch.go | ✅ |
| go-middleware | tls | （配置示例 + 跳过实测） | ✅ |
| go-framework | config | `/config/*` 路由 | ✅ |
| go-framework | hertz/server | main.go | ✅ |
| go-framework | hertz/response | handler/*.go | ✅ |
| go-framework | hertz/middleware | middleware/setup.go + `/protected/*` | ✅ |
| go-framework | hertz/observability | main.go + `/metrics` | ✅ |
| go-framework | hertz/log | middleware/setup.go | ✅ |
| go-framework | kitex/observability | rpc/server.go | ✅ |
| go-framework | kitex/log | rpc/server.go | ✅ |
| go-framework | kitex/rpcerror | rpc/service.go | ✅ |
| go-framework | kitex/option | rpc/server.go + rpc/client.go | ✅ |
| go-framework | kitex/middleware | rpc/server.go | ✅ |
