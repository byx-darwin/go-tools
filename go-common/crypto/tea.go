package crypto

import (
	"bytes"
	"encoding/hex"
	"errors"

	"golang.org/x/crypto/tea" //nolint:staticcheck // TEA 加密仍在使用中，后续迁移到 AES
)

func teaPad(cipherText []byte) ([]byte, int) {
	if len(cipherText)%tea.BlockSize == 0 {
		return cipherText, 0
	} else {
		padding := tea.BlockSize - len(cipherText)%tea.BlockSize
		padText := bytes.Repeat([]byte{64}, padding)
		return append(cipherText, padText...), padding
	}
}

// DecodeTeaStr 使用 TEA 密钥解密 src，pad 为填充长度。
func DecodeTeaStr(src []byte, pad int, teaKey string) ([]byte, error) {
	cr, err := tea.NewCipherWithRounds([]byte(teaKey), 32)
	if err != nil {
		return []byte{}, nil
	}

	if len(src)%tea.BlockSize != 0 {
		return []byte{}, errors.New("src length not correct")
	}

	n := 0
	l := len(src)
	rs := make([]byte, 0)
	for {
		left := l - n
		if left <= 0 {
			break
		} else {
			if left > 8 {
				left = 8
			}

			b := src[n : n+left]

			de := make([]byte, 8)
			cr.Decrypt(de, b)

			rs = append(rs, de...)

			n += 8
		}
	}

	if pad > 0 {
		return rs[:l-pad], nil
	} else {
		return rs, nil
	}
}

// EncodeTeaStr 使用 TEA 密钥加密 src，返回密文和总填充长度。
func EncodeTeaStr(src []byte, teaKey string) ([]byte, int, error) {
	cr, err := tea.NewCipherWithRounds([]byte(teaKey), 32)
	if err != nil {
		return []byte{}, 0, nil
	}

	n := 0
	l := len(src)
	rs := make([]byte, 0)
	totalPad := 0
	for {
		left := l - n
		if left <= 0 {
			break
		} else {
			if left > 8 {
				left = 8
			}

			b := src[n : n+left]
			bb, pad := teaPad(b)
			totalPad += pad

			en := make([]byte, 8)
			cr.Encrypt(en, bb)

			rs = append(rs, en...)

			n += 8
		}
	}

	return rs, totalPad, nil
}

// GetTeaPadLen 返回 TEA 分组对齐所需的填充长度。
func GetTeaPadLen(length int) int {
	if length%tea.BlockSize == 0 {
		return 0
	} else {
		padding := tea.BlockSize - length%tea.BlockSize
		return padding
	}
}

// TeaHexDecode 将十六进制密文解码后使用 TEA 密钥解密。
func TeaHexDecode(hexBody []byte, bLen int, teaKey string) ([]byte, error) {
	body := make([]byte, hex.DecodedLen(len(hexBody)))
	_, err := hex.Decode(body, hexBody)
	if err != nil {
		return []byte{}, err
	}
	if bLen > 0 {
		pad := GetTeaPadLen(bLen)
		decoded, err := DecodeTeaStr(body, pad, teaKey)
		if err != nil {
			return []byte{}, errors.New("encoded byte length error")
		}
		return decoded, nil
	}
	return []byte{}, nil
}
