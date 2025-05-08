package tools

import (
	"gitee.com/byx_darwin/go-tools/tools/crypto"
	"math/rand"
	"time"
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
