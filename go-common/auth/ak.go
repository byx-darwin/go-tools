package auth

import (
	"math/rand"
	"time"

	"github.com/byx-darwin/go-tools/go-common/crypto"
)

// GetRandAk 生成随机ak
func GetRandAk(length int) string {
	patter := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLOM" +
		"NOPQRSTUVWXYZ123456789"
	ak := ""
	for index := 0; index < length; index++ {
		n := rand.Intn(61)
		ak += patter[n : n+1]
	}
	return ak
}

// RefreshSK 刷新SK
func RefreshSK(ak string) string {
	signer := ak + "/" + time.Now().String()
	return crypto.MD5([]byte(signer))
}
