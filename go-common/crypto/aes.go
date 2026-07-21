package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

// ErrInvalidKeySize 表示 key 长度不是 16/24/32 字节（AES-128/192/256）。
var ErrInvalidKeySize = errors.New("crypto: invalid AES key size; must be 16, 24, or 32 bytes")

// ErrCiphertextTooShort 表示密文长度不足以包含 nonce 与认证 tag。
var ErrCiphertextTooShort = errors.New("crypto: ciphertext too short")

// AESGCM 是基于 AES-GCM 的认证加密器。
// Seal 产出的密文格式为 nonce(12 字节) ‖ 密封数据（含 16 字节 tag）。
type AESGCM struct {
	aead cipher.AEAD
	aad  []byte
}

// aesGCMOptions 持有 AESGCM 的可选配置。
type aesGCMOptions struct {
	aad []byte
}

// Option 配置 AESGCM。
type Option func(*aesGCMOptions)

// WithAssociatedData 设置附加认证数据 (AAD)。
// AAD 不参与加密，但参与完整性校验；Seal 与 Open 必须使用相同的 AAD。
func WithAssociatedData(aad []byte) Option {
	return func(o *aesGCMOptions) {
		o.aad = aad
	}
}

// NewAESGCM 使用 key 创建 AES-GCM 加密器。
// key 长度必须为 16/24/32 字节（对应 AES-128/192/256），否则返回 ErrInvalidKeySize。
func NewAESGCM(key []byte, opts ...Option) (*AESGCM, error) {
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: new AES cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new GCM: %w", err)
	}

	var o aesGCMOptions
	for _, opt := range opts {
		opt(&o)
	}

	return &AESGCM{aead: aead, aad: o.aad}, nil
}

// Seal 加密 plaintext，返回 nonce ‖ 密文 ‖ tag。
// 每次调用使用 crypto/rand 生成新的随机 nonce。
func (a *AESGCM) Seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, a.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("crypto: generate nonce: %w", err)
	}

	return a.aead.Seal(nonce, nonce, plaintext, a.aad), nil
}

// Open 解密 Seal 产出的密文并校验完整性。
// 密文长度非法返回 ErrCiphertextTooShort；密文被篡改或 AAD 不匹配时返回认证错误。
func (a *AESGCM) Open(ciphertext []byte) ([]byte, error) {
	nonceSize := a.aead.NonceSize()
	if len(ciphertext) < nonceSize+a.aead.Overhead() {
		return nil, ErrCiphertextTooShort
	}

	nonce, sealed := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := a.aead.Open(nil, nonce, sealed, a.aad)
	if err != nil {
		return nil, fmt.Errorf("crypto: decrypt: %w", err)
	}

	return plaintext, nil
}
