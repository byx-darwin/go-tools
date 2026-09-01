# JWT 密钥参数支持非对称算法（Issue #73）

## 背景

`go-auth/jwt/token.go` 中 `Sign`/`Verify`/`Refresh` 的密钥参数硬编码为 `[]byte`，但
`WithSigningMethod` 的 godoc 承诺支持 RS256/ES256 等非对称算法——这些算法需要
`*rsa.PrivateKey`/`*ecdsa.PrivateKey` 等类型，传入 `[]byte` 会在运行时失败。测试中
RS256 仅出现在"算法混淆攻击"防御用例里（预期返回错误），从未有用例真正用 RSA
密钥签发/验证成功。

## 范围

完整支持非对称算法（Issue 中的"中期"验收标准），而非仅澄清文档。

## 设计

1. **`go-auth/error/error.go`**：新增 `CodeJWTKeyTypeMismatch`（40010）+
   `ErrJWTKeyTypeMismatch`，与现有 `CodeJWTSignFailed`(40007)/`CodeJWTVerifyFailed`(40008)/
   `CodeJWTRefreshFailed`(40009) 平行。

2. **`go-auth/jwt/token.go`**：`Sign`/`Verify`/`Refresh` 的 `secret []byte` 参数改为
   `secret any`。对现有调用方非破坏性——`[]byte` 天然满足 `any`（`go-framework/hertz/middleware/jwt_auth.go`
   等消费方无需改动）。

3. 新增内部函数 `validateKeyType(method gojwt.SigningMethod, key any, forSigning bool) error`，
   对 golang-jwt/jwt/v5 已支持的算法族做类型开关校验：

   | 算法族 | 签名期望类型 | 验证期望类型 |
   |--------|------------|------------|
   | `*SigningMethodHMAC`（HS256/384/512） | `[]byte` | `[]byte` |
   | `*SigningMethodRSA`（RS256/384/512） | `*rsa.PrivateKey` | `*rsa.PublicKey` |
   | `*SigningMethodRSAPSS`（PS256/384/512） | `*rsa.PrivateKey` | `*rsa.PublicKey` |
   | `*SigningMethodECDSA`（ES256/384/512） | `*ecdsa.PrivateKey` | `*ecdsa.PublicKey` |
   | `*SigningMethodEd25519`（EdDSA） | `ed25519.PrivateKey` | `ed25519.PublicKey` |

   类型不匹配时返回 `autherror.ErrJWTKeyTypeMismatch`。

4. **调用时机（关键，保护现有安全测试语义）**：
   - `Sign`：在 `token.SignedString(secret)` 之前调用
     `validateKeyType(cfg.signingMethod, secret, true)`。
   - `Verify`：`validateKeyType` 放在 `keyfunc` 内部，且**必须在**现有
     `tok.Method != cfg.signingMethod` 比较**之后**才执行——这样算法混淆攻击
     （Token 实际算法与调用方声明的 `WithSigningMethod` 不一致）继续优先命中
     现有防御分支，返回 `autherror.CodeTokenInvalid`（`TestVerifyAlgorithmConfusion`
     不受影响）；只有当 Token 声明的算法与调用方期望一致、但传入密钥类型本身
     不匹配该算法时，才返回新的 `ErrJWTKeyTypeMismatch`。

5. **文档更新**：`options.go` 包注释与 `WithSigningMethod` godoc 补充各算法族的
   密钥类型要求；`token.go` 中 `Sign`/`Verify`/`Refresh` 的 `secret` 参数注释同步。

6. **测试**：
   - RS256、ES256 真实签发/验证成功的往返测试（`token_test.go`）。
   - 密钥类型不匹配返回 `ErrJWTKeyTypeMismatch` 的用例（`Sign` 与 `Verify` 各一个）。
   - 确认现有 `TestVerifyAlgorithmConfusion` 的 `CodeTokenInvalid` 断言不受影响。

## 非目标

- 不引入密钥管理/轮换机制。
- 不改变 HMAC 路径的既有行为。
