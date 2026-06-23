# TLS — 火山引擎日志服务

## 是什么

把本地日志文件 / 内存日志**自动批量上报**到火山引擎 TLS（Tinder Log Service）云端。

## 为什么需要

```
本地日志:  /var/log/app.log → 磁盘满了丢失, 出问题要 SSH grep
TLS 上报:  /var/log/app.log → 火山引擎云端 → Web 控制台全文检索 → 全链路串联
```

## 快速开始

### 1. 获取凭证

在火山引擎控制台创建 TLS 日志项目，获取：

- **Endpoint**: 服务地址，如 `tls-cn-beijing.volces.com`
- **Region**: 区域，如 `cn-beijing`
- **TopicID**: 日志主题 ID
- **AK/SK**: 访问密钥（建议环境变量注入，不要硬编码）

### 2. 代码

```go
package main

import (
    "context"
    "os"
    "gitee.com/byx_darwin/go-tools/go-middleware/tls"
)

func main() {
    p, err := tls.NewProducer(tls.ProducerConfig{
        Endpoint:        "tls-cn-beijing.volces.com",
        AccessKeyID:     os.Getenv("VOLC_ACCESS_KEY"),
        AccessKeySecret: os.Getenv("VOLC_SECRET_KEY"),
        Region:          "cn-beijing",
        TopicID:         "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
        Source:          "my-service",       // 标识日志来源（可选，默认 go-tools）
    })
    if err != nil {
        panic(err)
    }
    defer p.Close() // 确保退出时刷新缓冲区

    // 发送单条日志
    p.SendLog(context.Background(), map[string]string{
        "level":   "info",
        "message": "user logged in",
        "user_id": "12345",
    })

    // 发送多条日志
    p.SendLogs(context.Background(), []map[string]string{
        {"level": "info",  "message": "request started",  "path": "/api/v1/users"},
        {"level": "info",  "message": "request finished", "path": "/api/v1/users", "latency_ms": "42"},
    })

    // 强制刷新（正常关闭时 Close() 会自动刷新）
    p.Flush(context.Background())
}
```

## 批量发送机制

```
应用代码                     tls.Producer              火山引擎 TLS
   │                            │                         │
   ├─ SendLog("info","a") ──→ 缓冲区[a]                  │
   ├─ SendLog("info","b") ──→ 缓冲区[a,b]               │
   ├─ SendLog("info","c") ──→ 缓冲区[a,b,c]  ← 满10条 ──→ PUT /PutLogs (lz4压缩)
   │                            │                         │
   │                    每 5s 定时器 ────────────────────→ 刷新残留日志
   │                            │                         │
   ├─ Close() ─────────────────→ 刷新缓冲区所有日志 ────→
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `BatchSize` | 10 | 累积多少条触发一次批量发送 |
| `FlushInterval` | 5s | 最大等待时间，到时间强制刷新 |
| `CompressType` | lz4 | 日志压缩（减少带宽和费用） |

## 和生产环境集成

### 与 go-common/log 配合

```go
// 1. 本地写日志
logger := log.New(log.Config{
    Level:    "info",
    FilePath: "/var/log/app.log",
    JSON:     true,
})

// 2. 上报到 TLS
producer, _ := tls.NewProducer(tls.ProducerConfig{
    Endpoint:        os.Getenv("TLS_ENDPOINT"),
    AccessKeyID:     os.Getenv("VOLC_AK"),
    AccessKeySecret: os.Getenv("VOLC_SK"),
    Region:          "cn-beijing",
    TopicID:         os.Getenv("TLS_TOPIC_ID"),
    Source:          "app-server",
    BatchSize:        50,          // 生产环境调大
    FlushInterval:    10 * time.Second,
})
defer producer.Close()

// 3. 关键事件上报
http.HandleFunc("/api/order", func(w http.ResponseWriter, r *http.Request) {
    // 业务逻辑...
    producer.SendLog(r.Context(), map[string]string{
        "event":    "order_created",
        "order_id": orderID,
        "user_id":  userID,
        "amount":   strconv.Itoa(amount),
    })
})
```

### FileShipper — 文件自动上报

FileShipper 定时读取本地日志文件新增内容，自动解析 JSON 行并上报。配合 `go-common/log` 实现零侵入的日志上云。

```go
shipper, _ := tls.NewFileShipper(tls.FileShipperConfig{
    ProducerConfig: tls.ProducerConfig{
        Endpoint:        "tls-cn-beijing.volces.com",
        AccessKeyID:     os.Getenv("VOLC_AK"),
        AccessKeySecret: os.Getenv("VOLC_SK"),
        Region:          "cn-beijing",
        TopicID:         "xxx-xxx-xxx",
        Source:          "my-service",
        BatchSize:        50,
        FlushInterval:    10 * time.Second,
    },
    FilePath:      "/var/log/app.log",  // go-common/log 输出的文件
    CheckInterval: 2 * time.Second,      // 每 2s 扫描新行
})
shipper.Start()
defer shipper.Close()
```

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `CheckInterval` | 2s | 文件扫描间隔 |
| `MaxLineSize` | 64KB | 单行最大长度 |

### 配置建议

| 场景 | BatchSize | FlushInterval |
|------|-----------|---------------|
| 低流量（<10 QPS） | 10 | 5s |
| 中流量（10-100 QPS） | 50 | 10s |
| 高流量（>100 QPS） | 200 | 30s |

批量越大越省 API 调用费用，但丢失风险也越大（进程崩溃时缓冲区未刷新的日志会丢失）。

## 完整可观测链路

```
go-common/log    →  本地文件日志（JSON 结构）
go-middleware/tls →  FileShipper 自动上报 → 火山引擎 TLS 云端检索
go-framework/*   →  OTel Traces → Jaeger 调用链

三层组合 = 完整可观测性
```
