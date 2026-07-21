package auth

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
)

// akCharset 是 AK 使用的 62 个字母数字字符（a-z、A-Z、0-9）。
const akCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// skBytes 是 SK 的随机字节数（256 位熵）。
const skBytes = 32

// GetRandAk 生成并返回指定长度的随机 AK（Access Key）。
//
// 使用 crypto/rand 从 62 字符字母数字集合（a-z、A-Z、0-9）中无偏选取
// （rand.Int 拒绝采样），修复了历史上 math/rand、字符表重复 'O'、缺失 '0'
// 以及 Intn(61) 导致 '9' 永不出现的问题。length <= 0 时返回空字符串。
// 若读取 crypto/rand 失败（在支持的平台上几乎不可能），将 panic。
func GetRandAk(length int) string {
	if length <= 0 {
		return ""
	}
	ak := make([]byte, length)
	maxVal := big.NewInt(int64(len(akCharset)))
	for i := range ak {
		n, err := rand.Int(rand.Reader, maxVal)
		if err != nil {
			panic("auth: read crypto/rand: " + err.Error())
		}
		ak[i] = akCharset[n.Int64()]
	}
	return string(ak)
}

// RefreshSK 生成并返回密码学安全的随机 SK（Secret Key）。
//
// 内部使用 crypto/rand 生成 32 字节（256 位）随机数据并 hex 编码（64 字符），
// 用作 AK/SK 认证方案的 HMAC 密钥。SK 不再由 ak 或时间戳派生，具备完整秘密熵。
// 若读取 crypto/rand 失败（在支持的平台上几乎不可能），将 panic。
func RefreshSK() string {
	b := make([]byte, skBytes)
	if _, err := rand.Read(b); err != nil {
		panic("auth: read crypto/rand: " + err.Error())
	}
	return hex.EncodeToString(b)
}
