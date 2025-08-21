package middleware

import (
	"context"
	"encoding/base64"
	"fmt"
	"hash"
	"strconv"
	"strings"

	"gitee.com/byx_darwin/go-tools/tools/crypto"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/pkg/errors"
)

type AuthFace interface {
	GetSk(context context.Context, ctx *app.RequestContext,
		ak string, timestamp int64) (context.Context, string, bool, error)
}

func Auth(authFace AuthFace, hFunc func() hash.Hash) app.HandlerFunc {
	return func(context context.Context, ctx *app.RequestContext) {
		ak, sign, t, err := parseAuthorization(&ctx.Request)
		if err != nil {
			hlog.CtxErrorf(context, "authRequest failure,err:%s", err.Error())
			ctx.AbortWithStatus(consts.StatusBadRequest)
			return
		}
		ctx2, sk, isDebug, err := authFace.GetSk(context, ctx, ak, t)
		if err != nil {
			hlog.CtxErrorf(context, "getSk failure,ak:%s,err:%s", ak, err.Error())
			if isDebug {
				ctx.AbortWithMsg(err.Error(), consts.StatusForbidden)
			} else {
				ctx.AbortWithStatus(consts.StatusUnauthorized)
			}
			return
		}
		signed := GetSigned(ak, sk, string(ctx.Request.Path()), t, hFunc)
		if signed != sign {
			msg := fmt.Sprintf("sign invalid,client sign:%s,server client:%s", sign, signed)
			hlog.CtxErrorf(context, msg)

			if isDebug {
				ctx.AbortWithMsg(msg, consts.StatusForbidden)
			} else {
				ctx.AbortWithStatus(consts.StatusUnauthorized)
			}
			return
		}
		ctx.Next(ctx2)
	}
}

func parseAuthorization(request *protocol.Request) (string, string, int64, error) {
	authorization := request.Header.Get("X-Signature")
	if authorization == "" {
		return "", "", 0, errors.New("authorization not null")
	}
	authorizationBytes, err := base64.StdEncoding.DecodeString(authorization)
	if err != nil {
		return "", "", 0, errors.Wrap(err, "authorization base64 decode failure")
	}
	kvs := make(map[string]string)
	for _, v := range strings.Split(string(authorizationBytes), "&") {
		if strings.Contains(v, "=") {
			kv := strings.Split(v, "=")
			if len(kv) == 2 {
				kvs[kv[0]] = kv[1]
			}
		}
	}
	ak := ""
	sign := ""
	t := int64(0)
	if tempAk, ok := kvs["ak"]; !ok || kvs["ak"] == "" {
		return "", "", 0, errors.New("ak not null")
	} else {
		ak = tempAk
	}

	if tempSign, ok := kvs["sign"]; !ok || kvs["sign"] == "" {

		return "", "", 0, errors.New("sign not null")
	} else {
		sign = tempSign
	}

	if timeStr, ok := kvs["t"]; !ok || kvs["t"] == "" {
		return "", "", 0, errors.New("timestamp not null")
	} else {
		t, _ = strconv.ParseInt(timeStr, 10, 64)
	}
	return ak, sign, t, nil
}

func GetSigned(ak, sk, path string, t int64, hFunc func() hash.Hash) string {
	msg := fmt.Sprintf("%s%s/%d/%s", ak, path, t, ak)
	return crypto.Hmac([]byte(sk), []byte(msg), hFunc)
}
