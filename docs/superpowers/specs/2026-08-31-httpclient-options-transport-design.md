# httpclient Options 化 + 接口抽象设计

- Issue: https://github.com/byx-darwin/go-tools/issues/59
- Module: `go-common/httpclient`
- Date: 2026-08-31

## Context

`go-common/httpclient` 当前直接硬编码依赖 `fasthttp`，`Send`/`SendWithRetry` 是 5-6
个位置参数的函数，不符合 `.claude/rules/options-pattern.md` 规范；`Retry` 仅在
HTTP 5xx 时重试、网络层错误不重试；`Send`/`SendWithRetry` 直接修改调用方传入的
`headers` map；无 `context.Context` 支持；测试覆盖薄弱。

仓库内除 `go-common/httpclient` 自身外，仅 `example/handler/common_httpclient.go`
引用了 `Send`/`SendWithRetry`，`m3u8.go` 导出的 `GetM3u8TsSize`/
`DownloadM3u8TsData` 在仓库内无任何调用方。

## Goals

- 引入 `Client` 结构体 + `Transport` 接口 + Options 模式，替代裸露的位置参数函数
- 修复重试逻辑缺陷（网络层错误也应重试）
- 支持 `context.Context` 取消/超时传播
- 修复 headers map 副作用
- 保持旧 API（`Send`/`SendWithRetry`/`Retry`）行为不变，标记 `Deprecated`
- 补齐 `retry` 相关测试覆盖

## Non-Goals

- 不迁移 `m3u8.go`——该文件在仓库内无调用方，本次直接删除（不属于 httpclient
  职责，见下方「m3u8.go 删除」章节），不再补充其测试
- 不引入可配置的重试判断谓词（`WithRetryIf` 之类），重试规则固定为「网络错误
  或 5xx」

## Architecture

```
Client struct
  ├─ transport      Transport         // 可替换传输层，默认 fasthttp 实现
  ├─ maxRetries     int               // 默认 0（不重试）
  ├─ retryInterval  time.Duration     // 默认 500ms（沿用 DefaultSleep）
  ├─ timeout        time.Duration     // 默认超时；ctx 无 deadline 时生效
  └─ userAgent      string

Transport interface {
    Do(ctx context.Context, req *Request) (*Response, error)
}
  ├─ fasthttpTransport（默认实现，内部包装 fasthttp.Client）
  └─ nethttpTransport（新增，基于 net/http.Client，供未来迁移使用）

Request  struct { Method, URL string; Body []byte; Headers map[string]string }
Response struct { StatusCode int; Body []byte; Header http.Header }
```

### Option 列表

| Option | 作用 | 默认值 |
|--------|------|--------|
| `WithTransport(Transport)` | 替换底层传输实现 | `fasthttpTransport` |
| `WithMaxRetries(int)` | 最大重试次数 | `0`（不重试） |
| `WithRetryInterval(time.Duration)` | 首次重试等待间隔（后续指数退避+抖动） | `DefaultSleep`（500ms） |
| `WithTimeout(time.Duration)` | 请求默认超时（ctx 有 deadline 时以 ctx 为准） | 无（依赖 ctx 或 transport 默认） |
| `WithUserAgent(string)` | 自定义 User-Agent | `sznc-fasthttp-client-<version>` |

`NewClient(opts ...Option) *Client` 遵循项目 Options 规范：先填默认值，再应用
`opts`，依赖 opts 结果的初始化放在最后。

### 唯一请求方法

```go
func (c *Client) Do(ctx context.Context, method, url string, body []byte, headers map[string]string) (*Response, error)
```

不再区分 `Send`/`SendWithRetry` 两个方法——重试次数由 `Client` 构造时的
`WithMaxRetries` 决定，`0` 即等价于旧的 `Send`（不重试）。

## Retry 与 Context 处理

- 判断规则固定：`err != nil`（网络层错误）或 `resp.StatusCode >= 500` → 重试；
  `4xx` 不重试（与现状一致）
- 重试等待期间使用 `select` 同时监听 `ctx.Done()`：若 ctx 在等待中被取消/超时，
  立即返回 `ctx.Err()`，不再等待完整 sleep
- 沿用现有指数退避 + 随机抖动算法（`retryInterval` 每轮翻倍并加抖动），封装为
  `Client` 内部私有方法，不再导出 `Retry`/`BodyFunc` 供新代码使用
- `Do` 内部：若传入的 `ctx` 无 deadline 且 `Client.timeout > 0`，用
  `context.WithTimeout` 包一层后再调用 `transport.Do`

## Headers 副作用修复

`Do` 内部对传入的 `headers` 做浅拷贝（`clone := make(map[string]string, len(headers)+1)`
逐项复制，或使用 `maps.Clone` 后置 User-Agent），写入 `User-Agent` 到 clone，
不修改调用方传入的原始 map。

## 旧 API 兼容（Deprecated）

- `Send(url, method string, body []byte, headers map[string]string, timeout time.Duration) (*fasthttp.Response, int, error)`：
  内部构造一个默认 fasthttp Transport 的临时 `Client`（`maxRetries=0`），调用
  `Do(context.Background(), ...)`，再将统一 `*Response` 转换回
  `*fasthttp.Response`（构造 `fasthttp.Response` 并写入 `StatusCode`/`Body`/
  `Header`）。标记 `// Deprecated: 使用 NewClient 配合 Options 替代。`
- `SendWithRetry(...)`：同理，内部 `Client` 按传入的 `retry`/`sleep` 设置
  `WithMaxRetries`/`WithRetryInterval`。标记 `Deprecated`。
- `Retry`/`BodyFunc`：保留原实现供旧调用方使用（不再被新代码内部调用），标记
  `Deprecated`。

`example/handler/common_httpclient.go` 无需改动（继续使用 Deprecated 旧 API，
行为不变）。

## m3u8.go 删除

`m3u8.go`（`GetM3u8TsSize`/`DownloadM3u8TsData`）在仓库内无任何调用方，且其职责
（m3u8 分片下载）与 httpclient 包定位（通用 HTTP 客户端）不符。本次改动直接删除
该文件及对应测试点，不再纳入 Options/Transport 迁移范围。

## Testing

- `retry_test.go`：通过一个可控的假 `Transport`（返回预设的 err/status 序列）
  驱动 `Client.Do`，覆盖：
  - 网络层错误触发重试并最终成功
  - 5xx 触发重试，4xx 不触发重试
  - 重试次数耗尽后返回最后一次错误/状态
  - ctx 在重试等待期间被取消，提前返回 `ctx.Err()`
- `http_test.go`：适配新的 `Client`/`Request`/`Response` 结构，保留常量断言，
  新增 headers 不被修改的用例
- 删除 `m3u8.go` 后无需新增 m3u8 测试

## Migration Impact

- 破坏性：无。旧导出函数保留且行为不变（标记 Deprecated）。
- 删除：`m3u8.go` 及其导出函数 `GetM3u8TsSize`/`DownloadM3u8TsData`（仓库内无
  调用方）。
- 新增依赖：无新增第三方库（`net/http` 为标准库）。
