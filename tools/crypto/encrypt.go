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

func SHA1(content []byte) string {
	h := sha1.New()
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

func SHA512(content []byte) string {
	h := sha512.New()
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

func MD5(content []byte) string {
	return fmt.Sprintf("%x", md5.Sum(content))
}

func Hmac(key, content []byte, hFunc func() hash.Hash) string {
	h := hmac.New(hFunc, key)
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

func EncodePwd(password, ak string) string {
	return Hmac([]byte(ak), []byte(password), sha256.New)
}
