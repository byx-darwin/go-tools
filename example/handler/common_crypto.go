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

	// TEA 加密/解密示例。
	teaKey := "1234567890123456" // TEA 需要 16 字节密钥
	encoded, pad, encErr := crypto.EncodeTeaStr([]byte("sensitive-data"), teaKey)
	if encErr == nil {
		decoded, decErr := crypto.DecodeTeaStr(encoded, pad, teaKey)
		if decErr == nil {
			results["tea_encoded_hex"] = fmt.Sprintf("%x", encoded)
			results["tea_decoded"] = string(decoded)
			results["tea_pad_len"] = pad
		}
	}

	// 自定义 HMAC 使用 sha256.New 作为 hash 函数。
	results["hmac_custom"] = crypto.Hmac([]byte("key"), data, sha256.New)

	hertzresp.Success(c, results)
}
