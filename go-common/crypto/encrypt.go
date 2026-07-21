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
func EncodePwd(password, ak string) string {
	return Hmac([]byte(ak), []byte(password), sha256.New)
}
