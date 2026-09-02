# Issue #79: Store 接口 optional interface 扩展约定

## Context

`session.Store` / `device.Store` 是纯接口，已被 go-middleware 的 4 个实现
（Redis/Memory × Session/Device）完整覆盖。Go 接口没有"默认方法"机制，未来给
`Store` 接口新增方法（如批量操作、TTL 续期）会立即破坏所有下游实现。

## Goal

约定后续扩展 Store 能力时采用"新增独立小接口 + 可选类型断言（optional interface
pattern）"，而不是往 `Store` 大接口里加方法，并落地一个示例接口验证该模式。

## Design

### 1. 约定记录位置：package doc 注释

不新建 `.claude/rules/go-auth.md`。约定直接写在 `Store` 接口 godoc 上方，
理由：谁实现 `Store` 谁就能看到，且与现有安全要求注释风格一致（都是紧贴接口定义的
"实现方必须满足…"说明）。

- `go-auth/session/session.go`：`Store` 接口 godoc 前追加扩展约定段落
- `go-auth/device/store.go`：同样追加

### 2. 示例接口：TTLRefresher

在 `session` 和 `device` 两个包中各自新增一个对称的 `TTLRefresher` 可选接口，
验证模式本身，不修改 `Store` 接口签名：

```go
// session 包
type TTLRefresher interface {
    RefreshTTL(ctx context.Context, sessionID string, ttl time.Duration) error
}

// device 包
type TTLRefresher interface {
    RefreshTTL(ctx context.Context, userUUID, deviceID string, ttl time.Duration) error
}
```

- `session` 包：`Session.ExpiresAt` 字段使 TTL 续期语义清晰，落地在此包合理。
- `device` 包：为保持两个 Store 扩展约定的对称性，同样落地。
- **不修改 go-middleware 的现有 4 个实现** —— 它们不实现 `TTLRefresher` 也完全合法，
  这正是该模式要验证的向后兼容性（未实现可选接口的类型断言应为 `ok == false`，
  不应编译失败或运行时 panic）。

### 3. 测试

`session_test.go` / `store_test.go` 新增用例，验证"可选接口"模式本身：

- mock 类型同时实现 `Store` + `TTLRefresher`：类型断言 `ok == true`，且断言后的
  `RefreshTTL` 调用可正常执行
- mock 类型仅实现 `Store`（不含 `TTLRefresher`）：类型断言 `ok == false`
- 编译期接口满足断言：`var _ Store = (*mock)(nil)`、`var _ TTLRefresher = (*mock)(nil)`

## Out of Scope

- 不修改 go-middleware 中任何现有实现
- 不新建独立 rules 文件
- 不为 `TTLRefresher` 编写真实的 Redis/Memory 落地实现（仅接口定义 + 测试用 mock）
