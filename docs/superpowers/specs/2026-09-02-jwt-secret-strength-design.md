# JWT 密钥强度校验设计（Issue #76）

## 背景

`go-auth/jwt` 的 `Sign`/`Verify` 将 `secret []byte` 直接透传给 `golang-jwt`，
库内目前只校验密钥**类型**（`validateKeyType`），未校验 HMAC 密钥的**长度**。
RFC 7518 要求 HMAC 密钥长度不低于对应哈希算法的输出长度：

| 算法 | 最小密钥长度 |
|------|-------------|
| HS256 | 32 字节（256 bit） |
| HS384 | 48 字节（384 bit） |
| HS512 | 64 字节（512 bit） |

若调用方配置了短/弱密钥，HMAC 签名可被离线暴力破解，伪造任意 Token。

## 方案

这是对 `go-auth/jwt/token.go` 现有代码路径的有界改动，范围：

1. **错误码**：`go-auth/error` 新增 `CodeJWTWeakSecret = 40011`、
   `ErrJWTWeakSecret`，HTTP 状态码 500（与 `ErrJWTKeyTypeMismatch` 一致 ——
   均属调用方密钥配置错误，而非客户端请求错误）。

2. **校验位置**：扩展 `token.go` 中已有的 `validateKeyType`，在 HMAC 分支
   类型断言通过后追加密钥字节长度检查，按 `method.Alg()` 匹配上表最小长度，
   不满足时返回 `ErrJWTWeakSecret`。`Sign` 与 `Verify` 都调用该函数，天然
   覆盖签发与验证两条路径。

3. **GenerateSecret helper**：新增 `GenerateSecret(method gojwt.SigningMethod)
   ([]byte, error)`，使用 `crypto/rand` 生成对应算法所需最小合规长度的密钥，
   方便调用方生成合规密钥。

4. **godoc**：更新 `options.go` 包注释，说明 HMAC 密钥强度要求及
   `GenerateSecret` 用法。

## 测试

- `Sign`/`Verify` 使用短于最小长度的 HMAC 密钥时返回 `ErrJWTWeakSecret`。
- `GenerateSecret` 对 HS256/384/512 返回长度分别为 32/48/64 字节的密钥，且
  用其签发验证的 Token 可正常往返。

## 范围之外

- 非对称算法（RSA/ECDSA/EdDSA）密钥强度校验不在本次范围内（其密钥强度由
  密钥生成算法本身保证，不适用固定字节长度阈值）。
- 熵检测（如拒绝全零/重复字节的弱密钥）不在本次范围内，仅做长度下限校验。
