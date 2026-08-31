# fasthttpTransport ctx 已过期时退化为无超时请求 — 修复设计

- Issue: #64
- Type: bounded bug fix
- Files touched: `go-common/httpclient/transport_fasthttp.go`, `go-common/httpclient/transport_fasthttp_test.go`

## 问题

`fasthttpTransport.Do` 中：

```go
timeout := time.Until(deadlineOrZero(ctx))
if timeout > 0 {
    err = fasthttp.DoTimeout(fReq, fResp, timeout)
} else {
    err = fasthttp.Do(fReq, fResp)
}
```

`deadlineOrZero(ctx)` 在 ctx **无 deadline** 时返回零值，ctx **已过期**时 `time.Until(...)` 同样算出负数——两种情况都落入 `timeout <= 0` 的 `else` 分支，调用不带超时的 `fasthttp.Do`。已取消/已过期的 ctx 被完全忽略，请求会无限期发起。

`nethttpTransport` 不受影响，因为 `http.NewRequestWithContext` + `http.Client.Do` 会正确尊重已取消/过期的 ctx。

## 方案

在 `Do` 入口处增加 `ctx.Err()` 检查，非 nil 时直接返回该错误，不获取 fasthttp 请求/响应对象、不发起网络调用：

```go
func (t *fasthttpTransport) Do(ctx context.Context, req *Request) (*Response, error) {
    if err := ctx.Err(); err != nil {
        return nil, err
    }
    fReq := fasthttp.AcquireRequest()
    ...
}
```

原有 `timeout > 0` / `else` 分支保持不变，继续区分"有效 deadline"（走 `DoTimeout`）与"无 deadline"（走 `Do`，保持现状）两种情况。

## 测试

新增两个测试用例（不可达地址佐证未真正发起网络调用）：

1. `ctx` 通过 `context.WithCancel` 立即 cancel → 断言 `errors.Is(err, context.Canceled)`
2. `ctx` 通过 `context.WithDeadline` 设为过去时间 → 断言 `errors.Is(err, context.DeadlineExceeded)`

## 验收标准

- [ ] `fasthttpTransport.Do` 入口检查 `ctx.Err()`，非 nil 时直接返回，不发起请求
- [ ] 无 deadline 情况保持现状（无超时请求）
- [ ] 已取消 / 已过期 deadline 的 ctx 单元测试覆盖
