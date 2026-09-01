# Design: Refresh Token 轮换 + 复用检测（Issue #74）

## Context

`jwt.Refresh` 目前只是"验证旧 token → 用新过期时间重新签发"，旧 token 在到期前依然
可反复用于刷新，没有一次性 refresh token、复用检测（reuse detection）机制。如果
refresh token 泄露，攻击者可以和合法用户同时用同一个 token 无限续期，无法被发现或
阻断。

`go-auth/revocation`（Checker/Revoker/Store 接口）、`autherror.ErrTokenRevoked`、
`device.Store.RemoveAllDevices`、`jwt.ExtractJTI` 均已在此前的 PR 中落地
（#72/#82/#73），本设计只需将这些已有构建块接线进 `jwt.Refresh`。

## Goal

在 Refresh 成功后撤销旧 JTI；同一 JTI 被再次用于 Refresh 时视为复用攻击，返回
`ErrTokenRevoked`；全设备登出留给调用方按需触发。

## API 变更（Breaking Change）

```go
func Refresh[T any](ctx context.Context, tokenStr string, secret any, store revocation.Store, opts ...Option) (string, error)
```

- 新增 `ctx context.Context`：`revocation.Checker`/`Revoker` 方法都要求 ctx，
  `Sign`/`Verify` 无 IO 需求，保持不变。
- 新增 `store revocation.Store`：必选位置参数，强制调用方显式决策是否启用复用检测，
  不留"静默跳过安全检查"的默认值。

## 核心逻辑

1. `Verify[T](tokenStr, secret, opts...)` 验证旧 token，取出 `claims *T`（不变）。
2. `ExtractJTI(claims)`：
   - **无 JTI**（`ok == false`）→ 跳过轮换/复用检测，行为与现在完全一致
     （向后兼容不携带 JTI 的 Claims）。
   - **有 JTI**：
     1. `store.IsRevoked(ctx, jti)` → `true` → 判定复用攻击，返回
        `autherror.ErrTokenRevoked`，**不签发新 token**。
     2. `IsRevoked` 返回 error → fail-closed，包装后直接返回，不签发新 token
        （安全默认：存储故障时拒绝而非放行）。
     3. 未撤销 → `store.Revoke(ctx, jti, ttl)` 撤销旧 JTI；`ttl` = 旧 token
        剩余有效期（`rc.ExpiresAt.Time - now`，非正值兜底为一个最小 TTL）。
     4. `Revoke` 返回 error → 同样 fail-closed，直接返回错误。
     5. 成功后为新 token 生成新 JTI（`uuid.NewString()`），写回 claims 副本的
        `RegisteredClaims.ID`（复用 `extractRegisteredClaims` 已有的反射寻址逻辑，
        与 `setClaimsDefaults` 写回 `ExpiresAt`/`Issuer` 的方式一致）。
3. `Sign(*claims, secret, signOpts...)` 签发新 token（沿用现有的
   `withIgnoreClaimsExpiration` 语义：新 token 使用新的过期时间，不复用旧 token
   的剩余有效期）。

**全设备登出**：调用方在业务层收到 `ErrTokenRevoked` 后自行决定是否调用
`device.Store.RemoveAllDevices`。`jwt` 包不引入 `device` 包依赖，保持职责单一。

## 新增依赖

- `go-auth/go.mod` 新增直接依赖 `github.com/google/uuid`（仓库内
  `example/handler/auth_device.go`、`auth_session.go`、
  `go-framework/hertz/response.go` 均已使用，非新库）。

## go-middleware/auth 新增 `MemoryRevocationStore`

`revocation.Store` 此前只有 `RedisRevocationStore` 实现。镜像
`MemoryDeviceStore`/`MemorySessionStore` 的写法（`samber/hot` LRU +
`SetWithTTL`），补齐内存实现，供 example 接线与后续测试使用。

## example 接线

- `example/handler/auth_jwt.go` 新增 `revocationStore revocation.Store` 包级变量 +
  `SetRevocationStore` 注入函数（与现有 `deviceStore`/`sessionStore` 模式一致）。
- `example/main.go` `initDeps` 中 `deps.RevocationStore = mwauth.NewMemoryRevocationStore()`，
  注入 handler（Redis 分支当前本就 fallback 到内存，不额外处理）。
- `jwtRefreshHandler` 改为
  `jwt.Refresh[AppClaims](ctx, req.RefreshToken, jwtSecret, revocationStore, opts...)`。

## 测试

- `go-auth/jwt/token_test.go`：新增 `fakeRevocationStore`（内存 map 测试替身，避免
  `go-auth` 反向依赖 `go-middleware`）：
  - 正常轮换：旧 JTI 被 Revoke，新 token 的 JTI 与旧 JTI 不同。
  - 复用检测：同一旧 token 二次 Refresh → 返回 `ErrTokenRevoked`。
  - 无 JTI 的 Claims → 跳过检测，行为与变更前一致。
  - `IsRevoked`/`Revoke` 返回 error → `Refresh` 返回错误，不签发新 token。
- `go-middleware/auth/revocation_memory_test.go`：新增，覆盖 TTL 过期、
  Revoke/IsRevoked 基本语义。

## 文档

- `Refresh` godoc 明确新签名、复用检测语义、fail-closed 行为、无 JTI 时跳过的
  兼容说明。
- `go-auth/jwt/options.go` 包注释中的用法示例同步更新（`Refresh` 调用需要补上
  `ctx`、`store` 参数）。

## 范围外

- 不新增 `WithReuseHandler` 类的自动全设备登出选项——调用方自行处理更清晰、职责
  更单一。
- 不为 `Sign`/`Verify` 引入 `ctx`（它们没有 IO 需求）。
