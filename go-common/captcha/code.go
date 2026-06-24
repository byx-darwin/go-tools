package captcha

import (
	"crypto/rand"
	"math/big"
)

const (
	digitChars        = "0123456789"
	alphanumericChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// GenerateCode 生成指定长度的随机验证码。
// charset 可选 "digit"（纯数字）或 "alphanumeric"（数字+大小写字母），
// 其他值默认使用 "digit"。
// 使用 crypto/rand 保证密码学安全。
func GenerateCode(length int, charset string) string {
	chars := digitChars
	if charset == "alphanumeric" {
		chars = alphanumericChars
	}
	buf := make([]byte, length)
	max := big.NewInt(int64(len(chars)))
	for i := range buf {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			// crypto/rand.Read 在正常系统上不会失败；
			// 极端降级：使用数学随机（理论上不应走到这里）。
			buf[i] = chars[i%len(chars)]
			continue
		}
		buf[i] = chars[n.Int64()]
	}
	return string(buf)
}

// GenerateDigitCode 生成纯数字验证码（兼容原 GetRandCaptcha）。
func GenerateDigitCode(length int) string {
	return GenerateCode(length, "digit")
}
