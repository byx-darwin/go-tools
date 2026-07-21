package handler

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/byx-darwin/go-tools/go-common/crypto"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterCryptoRoutes 注册 crypto 示例路由。
func RegisterCryptoRoutes(h *server.Hertz) {
	h.GET("/common/crypto", cryptoHandler)
}

func cryptoHandler(_ context.Context, c *app.RequestContext) {
	data := []byte("hello go-tools")

	results := map[string]any{
		"md5":         crypto.MD5(data),
		"sha256":      crypto.SHA256(data),
		"sha512":      crypto.SHA512(data),
		"sha1":        crypto.SHA1(data),
		"hmac_sha256": crypto.HMACSHA256(data, []byte("secret-key")),
		"encode_pwd":  crypto.EncodePwd("mypassword", "access-key"),
	}

	// AES-GCM 认证加密/解密示例。
	aesKey := []byte("0123456789abcdef") // AES-128 需要 16 字节密钥
	aesCipher, newErr := crypto.NewAESGCM(aesKey)
	if newErr == nil {
		encoded, sealErr := aesCipher.Seal([]byte("sensitive-data"))
		if sealErr == nil {
			decoded, openErr := aesCipher.Open(encoded)
			if openErr == nil {
				results["aesgcm_encoded_hex"] = fmt.Sprintf("%x", encoded)
				results["aesgcm_decoded"] = string(decoded)
			}
		}
	}

	// 自定义 HMAC 使用 sha256.New 作为 hash 函数。
	results["hmac_custom"] = crypto.Hmac([]byte("key"), data, sha256.New)

	hertzresp.Success(c, results)
}
