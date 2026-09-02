# Design: Verify Issuer 校验（Issue #75）

## Context

`WithIssuer` 的 godoc 未说明它仅在 Sign 时生效；调用方若误以为
`jwt.Verify(token, secret, jwt.WithIssuer("myapp"))` 会校验 Token 的 issuer
是否等于 "myapp"，实际上该选项在 Verify 路径被静默忽略（`cfg.issuer` 只在
`setClaimsDefaults` 里被使用，该函数只在 Sign 中调用）。跨服务共享同一签名
密钥的场景下，如果开发者误以为传入 WithIssuer 就能限定 Verify 只接受特定
issuer 的 Token，实际上任何 issuer 的 Token 都会通过校验，可能导致越权。

## Goal

在 Verify 中真正实现 issuer 校验，同时不破坏 `Refresh` 现有的
`WithIssuer` 用法（`Refresh` 内部先 `Verify` 旧 token 再 `Sign` 新
token，两步共用同一份 `opts`）。

## 设计冲突与决策

若直接让 `WithIssuer` 同时影响 Sign 与 Verify，`Refresh` 内部透传给
`Verify` 的 opts 会把"调用方想设置到新 token 上的 issuer"误用于校验
"旧 token 的 issuer"，导致合法的 Refresh（旧 token 无 issuer 或 issuer
不同，只是想给新 token 设置新 issuer）被误判为不匹配而拒绝——现有测试
`TestRefreshWithIssuer` 正是这个场景。

**决策**：新增独立选项 `WithExpectedIssuer`，专用于 Verify 路径的校验；
`WithIssuer` 保持现状（仅 Sign 时生效，只补文档说明）。两个选项语义不
重叠，`Refresh` 无需任何特殊处理，现有测试不受影响。

## API 变更

`go-auth/jwt/options.go`：

```go
func WithExpectedIssuer(issuer string) Option {
    return func(c *config) {
        if issuer != "" {
            c.expectedIssuer = issuer
        }
    }
}
```

`config` 结构体新增 `expectedIssuer string` 字段。

## 核心逻辑

`go-auth/jwt/token.go` 的 `Verify`：调用 `gojwt.ParseWithClaims` 时，若
`cfg.expectedIssuer != ""`，追加 `gojwt.WithIssuer(cfg.expectedIssuer)`
作为 `gojwt.ParserOption`。校验失败（issuer 不匹配或 token 缺失 `iss`
claim）时走现有 `mapJWTError` 默认分支，返回 `autherror.ErrTokenInvalid`
（不新增错误码）。

## godoc 更新

- `WithIssuer`：明确"仅在 Sign 时生效，用于设置签发者；Verify 不读取此
  选项。校验 issuer 请用 `WithExpectedIssuer`"
- `WithExpectedIssuer`：明确"仅在 Verify 时生效，要求 token 的 `iss`
  claim 必须等于给定值；不匹配或缺失均返回错误。Sign 不读取此选项"

## 测试

`go-auth/jwt/token_test.go` 新增：
- issuer 匹配 → Verify 成功
- issuer 不匹配 → Verify 返回 `autherror.CodeTokenInvalid`
- token 缺失 `iss` claim 但设了 `WithExpectedIssuer` → Verify 返回错误
- 不设置 `WithExpectedIssuer` 时，Verify 忽略 token 的 `iss`（向后兼容
  回归测试）

`TestRefreshWithIssuer` 不改动，确认仍然通过。

## 范围外

- 不改动 `Refresh` 的内部逻辑——它不会用到新选项，除非调用方显式传入。
