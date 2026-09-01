# PR #82 代码审查报告

- **仓库**: byx-darwin/go-tools
- **PR**: [#82](https://github.com/byx-darwin/go-tools/pull/82) — feat(go-auth): JWT 密钥参数支持非对称算法类型 (RS256/ES256/EdDSA)
- **分支**: `feat/73-jwt-key-type-any` → `main`
- **关联 Issue**: #73
- **审查时间**: 2026-09-01
- **审查人**: Claude (gf-review 自动化审查)
- **PR 状态**: 已合并（`state: closed`, `mergedAt: 2026-09-01T10:10:06Z`）——本次为合并后的补充正式审查，未做代码修改。

## 变更概览

| 文件 | 改动 |
|------|------|
| `go-auth/error/error.go` | 新增 `CodeJWTKeyTypeMismatch = 40010` 及对应 `ErrJWTKeyTypeMismatch`，注册 HTTP 500 |
| `go-auth/error/error_test.go` | 补充新错误码的三处表驱动测试用例 |
| `go-auth/jwt/token.go` | `Sign`/`Verify`/`Refresh` 密钥参数 `[]byte` → `any`；新增 `validateKeyType` 内部函数 |
| `go-auth/jwt/token_test.go` | 新增 RS256/ES256 真实签发验证往返测试 + 密钥类型不匹配测试（签名/验证两侧） |
| `go-auth/jwt/options.go` | 包注释与 `WithSigningMethod` godoc 补充各算法族密钥类型说明 |
| `plans/…`, `specs/…` | 设计文档与实施计划（非代码） |

## 关注点逐项核查

### 1. `validateKeyType` 调用时机 —— 通过

`go-auth/jwt/token.go` 的 `Verify` 中，`keyfunc` 内部顺序为：

```go
if tok.Method != cfg.signingMethod {
    return nil, fmt.Errorf("unexpected signing method: ...")
}
if err := validateKeyType(cfg.signingMethod, secret, false); err != nil {
    keyTypeErr = err
    return nil, err
}
```

算法混淆防御分支（`tok.Method != cfg.signingMethod`）严格先于密钥类型校验执行，且注释明确说明了原因（"算法不匹配是安全防御，优先级高于调用方的密钥类型配置错误"）。

实测验证：
- `go test ./go-auth/... -run TestVerifyAlgorithmConfusion -v` → PASS。该用例中 token 以 HS256 签发、`Verify` 期望 RS256、`secret` 为 `[]byte`（对 RS256 而言类型也不匹配），验证了即便密钥类型同时不匹配，`tok.Method != cfg.signingMethod` 分支仍先触发、`keyTypeErr` 保持 `nil`，最终经 `mapJWTError` 返回 `CodeTokenInvalid`，语义未变。
- 新增的 `TestVerifyKeyTypeMismatch` 用例进一步验证了"算法匹配但密钥类型错误"场景下才会返回 `CodeJWTKeyTypeMismatch`，两者边界清晰、无重叠风险。

结论：调用时机正确，无回归风险。

### 2. `CodeJWTKeyTypeMismatch = 40010` 错误码规范 —— 通过

- 数值 40010 落在 `go-auth` 错误码范围 40000-40099 内（CLAUDE.md 表格 D6 / 错误码范围章节），且紧接现有最大值 40009，无跳号或冲突。
- 常量注释 `// JWT 密钥类型与签名算法不匹配` 采用与同组常量一致的行尾注释风格。
- `ErrJWTKeyTypeMismatch` 的 `Public("jwt_key_type_mismatch")` 字符串命名风格与同组 `jwt_sign_failed`/`jwt_verify_failed`/`jwt_refresh_failed` 一致。
- HTTP 状态注册为 500，与同组 `CodeJWTSignFailed`/`CodeJWTVerifyFailed`/`CodeJWTRefreshFailed` 一致（均视为服务端/调用方配置类错误而非客户端输入错误），定位合理。
- `error_test.go` 的三张表（`TestCodeConstants`/`TestPredefinedErrors`/`TestHTTPStatusRegistration`）均已补充对应用例并通过。

结论：错误码定义与既有规范完全一致。

### 3. `[]byte → any` 对现有调用方的非破坏性 —— 通过

- 检查 `go-framework/hertz/middleware/jwt_auth.go`：`JWTAuth[T any](secret []byte, opts ...JWTAuthOption)` 签名未改动，内部调用 `gojwt.Verify[T](token, secret)`——`[]byte` 隐式满足新的 `any` 形参，属于 Go 类型系统天然兼容的场景，无需修改调用方代码。
- 全仓库编译验证：`go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...` 通过。
- 全仓库测试验证：`go test ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1` 全部 PASS（含 `go-framework/hertz/middleware` 的 `jwt_auth_test.go`、`device_auth_test.go`）。
- 运行时行为：原有 HMAC 场景下 `validateKeyType` 对 `[]byte` 类型校验必然通过（`case *gojwt.SigningMethodHMAC: _, ok = key.([]byte)`），不会给现有 HS256 调用方引入新的失败路径。

结论：改动对现有调用方（编译期与运行时）均非破坏性。

### 4. godoc 注释规范（`.claude/rules/go.md` § 8.3） —— 通过

抽查改动涉及的导出/内部符号：

| 符号 | 注释首词 | 是否合规 |
|------|---------|---------|
| `Sign` | `// Sign 签发 JWT，...` | ✅ 以符号名开头 |
| `Verify` | `// Verify 验证 JWT，...` | ✅ |
| `Refresh` | `// Refresh 刷新 JWT（...）...` | ✅ |
| `validateKeyType`（非导出） | `// validateKeyType 校验密钥类型...` | ✅（非导出符号非强制要求，但已遵循同样规范，风格统一） |
| `WithSigningMethod` | 原有注释基础上补充说明，未破坏首词规范 | ✅ |
| `CodeJWTKeyTypeMismatch` / `ErrJWTKeyTypeMismatch` | 分组常量/变量，组前已有分组注释（`// 认证错误码 40000-40099。` / `// 预定义认证错误构造器。`），单项用行尾注释 | ✅ 符合 8.3 节"分组常量/变量，每个分组前加一行分组注释即可"的约定 |

`golangci-lint run --timeout=5m ./go-auth/... ./go-framework/...`（含 `revive` 导出符号注释检查）均为 **0 issues**。

结论：godoc 注释符合项目规范。

## 验证记录

在 PR 分支 worktree（`.worktree/feat/73-jwt-key-type-any`，HEAD `da0dcf6`）中复核：

```text
go build ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...   → 通过
go vet   ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/...   → 通过
go test  ./go-common/... ./go-auth/... ./go-middleware/... ./go-framework/... -count=1 → 全部 PASS
gofmt -l <all .go files>                                                        → 无输出（已格式化）
golangci-lint run ./go-auth/...      → 0 issues
golangci-lint run ./go-framework/... → 0 issues
go test ./go-auth/... -run TestVerifyAlgorithmConfusion -v → PASS
```

与 PR 描述中声明的 Test plan 一致，结论可复现。

## 次要观察（非阻断，仅供参考）

- `validateKeyType` 中 `*gojwt.SigningMethodRSAPSS` 与 `*gojwt.SigningMethodRSA` 两个 `case` 分支逻辑完全相同，可用 `case *gojwt.SigningMethodRSAPSS, *gojwt.SigningMethodRSA:` 合并以减少重复；属代码风格优化，不影响正确性，不要求本次处理。
- `default` 分支对未识别的自定义 `SigningMethod` 跳过前置校验、交由 golang-jwt 自身处理，注释已说明设计意图，行为合理。

## 结论

四项关注点均核实通过，未发现阻断性问题；构建、vet、lint、全量测试与专项安全测试（`TestVerifyAlgorithmConfusion`）均通过。**建议：Approve。**

（注：PR 在审查前已合并至 `main`，本报告为合并后的追认性正式审查记录。）
