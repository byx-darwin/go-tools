# go-common

零框架依赖的 Go 通用工具库。**go-tools 三层架构的最底层**。

## 安装

```bash
go get gitee.com/byx_darwin/go-tools/go-common
```

## 包一览

| 包 | 说明 |
|----|------|
| `auth` | AK/SK 生成与刷新 |
| `cache` | 泛型缓存（基于 `github.com/samber/hot`，LRU/LFU/FIFO/TwoQueue/ARC + TTL） |
| `captcha` | 验证码缓存存储 |
| `crypto` | MD5 / SHA1 / SHA256 / SHA512 / HMAC-SHA256 / TEA 加解密 |
| `httpclient` | fasthttp HTTP 客户端 + 重试 + m3u8 下载 |
| `log` | 结构化日志（基于 `log/slog`，JSON/Text 输出，OTel span 关联） |
| `log/adapters` | Kitex klog / Hertz hlog 适配器 |
| `netutil` | 内网 IP 获取 + 网络连通性检测 |
| `timeutil` | 自定义日期格式化 + 月份计算 |

## 依赖

- Go 标准库 `log/slog`
- `github.com/samber/hot` — 缓存底层
- `github.com/valyala/fasthttp` — HTTP 客户端
- `golang.org/x/crypto` — TEA 加密
- `go.opentelemetry.io/otel/trace` — OTel span 关联
