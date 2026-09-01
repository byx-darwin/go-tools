// Package crypto 提供加密/解密、哈希和 HMAC 工具函数。
//
// 支持 AES-GCM 认证加密、MD5、SHA 系列哈希，以及 HMAC 签名验证。
package crypto

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
)

// SHA1 返回 content 的 SHA-1 十六进制摘要。
//
// 安全告诫：SHA-1 已被证明存在碰撞攻击，且不带盐、非慢哈希，不得用于密码存储、
// 数字签名等安全敏感场景，仅适用于校验和、去重等非安全用途。
func SHA1(content []byte) string {
	h := sha1.New()
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// SHA512 返回 content 的 SHA-512 十六进制摘要。
func SHA512(content []byte) string {
	h := sha512.New()
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// MD5 返回 content 的 MD5 十六进制摘要。
//
// 安全告诫：MD5 已被证明存在碰撞攻击，且不带盐、非慢哈希，不得用于密码存储、
// 数字签名等安全敏感场景，仅适用于校验和、去重等非安全用途。
func MD5(content []byte) string {
	return fmt.Sprintf("%x", md5.Sum(content))
}

// Hmac 返回使用指定 hash 函数计算的 HMAC 十六进制摘要。
func Hmac(key, content []byte, hFunc func() hash.Hash) string {
	h := hmac.New(hFunc, key)
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// SHA256 返回 content 的 SHA-256 十六进制摘要。
func SHA256(content []byte) string {
	h := sha256.New()
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// HMACSHA256 computes HMAC-SHA256(data, key).
func HMACSHA256(data, key []byte) string {
	return Hmac(key, data, sha256.New)
}

// EncodePwd 使用 ak 作为密钥对 password 进行 HMAC-SHA256 编码。
//
// 安全告诫：本函数底层是 HMAC-SHA256，属于快速哈希算法，不带盐、不支持可调
// 工作因子，不具备专用密码哈希算法（如 bcrypt/scrypt/argon2）应有的抗暴力
// 破解特性。不得直接用于用户密码的存储或校验；如需安全的密码哈希方案，请
// 使用 bcrypt/scrypt/argon2 等专用算法（当前仓库未内置封装）。
func EncodePwd(password, ak string) string {
	return Hmac([]byte(ak), []byte(password), sha256.New)
}
