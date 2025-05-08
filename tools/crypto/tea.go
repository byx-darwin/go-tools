package crypto

import (
	"bytes"
	"encoding/hex"
	"github.com/pkg/errors"
	"golang.org/x/crypto/tea"
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

func GetTeaPadLen(length int) int {
	if length%tea.BlockSize == 0 {
		return 0
	} else {
		padding := tea.BlockSize - length%tea.BlockSize
		return padding
	}
}

func TeaHexDecode(hexBody []byte, bLen int, teaKey string) ([]byte, error) {
	body := make([]byte, hex.DecodedLen(len(hexBody)))
	_, err := hex.Decode(body, hexBody)
	if err != nil {
		return []byte{}, err
	}
	//解析数据
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
