package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// AuthFace 鉴权接口，由业务方实现 AK/SK 查找逻辑。
type AuthFace interface {
	GetSk(ctx context.Context, c *app.RequestContext,
		ak string, timestamp int64) (sk string, isDebug bool, err error)
}

// Auth 返回 Hertz 鉴权中间件。
// 验证 X-Signature 头中的签名：Base64Decode(ak=xxx&sign=xxx&t=xxx)
// 签名算法：HmacSHA256(sk, ak+path+/+t+/+ak) → hex
func Auth(authFace AuthFace) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		ak, sign, t, err := parseAuthorization(&c.Request)
		if err != nil {
			c.AbortWithStatus(consts.StatusBadRequest)
			return
		}

		sk, isDebug, err := authFace.GetSk(ctx, c, ak, t)
		if err != nil {
			if isDebug {
				c.AbortWithMsg(err.Error(), consts.StatusForbidden)
			} else {
				c.AbortWithStatus(consts.StatusUnauthorized)
			}
			return
		}

		expected := signRequest(ak, sk, string(c.Request.Path()), t)
		if expected != sign {
			if isDebug {
				c.AbortWithMsg(fmt.Sprintf("sign invalid, client:%s server:%s", sign, expected), consts.StatusForbidden)
			} else {
				c.AbortWithStatus(consts.StatusUnauthorized)
			}
			return
		}

		c.Next(ctx)
	}
}

// parseAuthorization 从 X-Signature 头解析 ak, sign, t
func parseAuthorization(request *protocol.Request) (ak, sign string, t int64, err error) {
	auth := string(request.Header.Peek("X-Signature"))
	if auth == "" {
		return "", "", 0, goerror.In("auth.parseAuthorization").
			Code(goerror.CodeParamInvalid).
			Errorf("authorization header is empty")
	}

	decoded, err := base64.StdEncoding.DecodeString(auth)
	if err != nil {
		return "", "", 0, goerror.In("auth.parseAuthorization").
			Code(goerror.CodeParamInvalid).
			Wrap(err)
	}

	kvs := make(map[string]string)
	for _, part := range strings.Split(string(decoded), "&") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			kvs[kv[0]] = kv[1]
		}
	}

	if ak = kvs["ak"]; ak == "" {
		return "", "", 0, goerror.In("auth.parseAuthorization").
			Code(goerror.CodeParamInvalid).
			Errorf("ak is empty")
	}
	if sign = kvs["sign"]; sign == "" {
		return "", "", 0, goerror.In("auth.parseAuthorization").
			Code(goerror.CodeParamInvalid).
			Errorf("sign is empty")
	}
	if tt, ok := kvs["t"]; !ok || tt == "" {
		return "", "", 0, goerror.In("auth.parseAuthorization").
			Code(goerror.CodeParamInvalid).
			Errorf("timestamp is empty")
	} else {
		t, _ = strconv.ParseInt(tt, 10, 64)
	}

	return ak, sign, t, nil
}

// signRequest 计算请求签名
func signRequest(ak, sk, path string, t int64) string {
	msg := fmt.Sprintf("%s%s/%d/%s", ak, path, t, ak)
	h := hmac.New(sha256.New, []byte(sk))
	h.Write([]byte(msg))
	return hex.EncodeToString(h.Sum(nil))
}
