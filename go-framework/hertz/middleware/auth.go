package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// defaultTimestampWindow AK/SK 签名时间戳默认新鲜度窗口（±5 分钟）。
const defaultTimestampWindow = 5 * time.Minute

// AuthFace 鉴权接口，由业务方实现 AK/SK 查找逻辑。
type AuthFace interface {
	GetSk(ctx context.Context, c *app.RequestContext,
		ak string, timestamp int64) (sk string, isDebug bool, err error)
}

// authOptions Auth 中间件的可选配置。
type authOptions struct {
	timestampWindow time.Duration
}

// Option Auth 中间件配置选项函数。
type Option func(*authOptions)

// WithTimestampWindow 设置时间戳新鲜度窗口（±window）。
// window <= 0 时忽略，保持默认 5 分钟。
func WithTimestampWindow(window time.Duration) Option {
	return func(o *authOptions) {
		if window > 0 {
			o.timestampWindow = window
		}
	}
}

// Auth 返回 Hertz AK/SK 鉴权中间件。
//
// 验证 X-Signature 头中的签名：Base64Decode(ak=xxx&sign=xxx&t=xxx)。
// 规范签名格式见 signRequest 文档。中间件自身强制时间戳新鲜度窗口（默认 ±5 分钟，
// 可用 WithTimestampWindow 配置），使用常量时间比较签名，验签失败时不向客户端回显
// 服务端计算出的签名（避免调试模式成为凭据预言机）。
func Auth(authFace AuthFace, opts ...Option) app.HandlerFunc {
	o := &authOptions{timestampWindow: defaultTimestampWindow}
	for _, opt := range opts {
		opt(o)
	}

	return func(ctx context.Context, c *app.RequestContext) {
		ak, sign, t, err := parseAuthorization(&c.Request)
		if err != nil {
			c.AbortWithStatus(consts.StatusBadRequest)
			return
		}

		if !timestampFresh(t, o.timestampWindow, time.Now) {
			c.AbortWithStatus(consts.StatusUnauthorized)
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

		expected := signRequest(ak, sk, string(c.Request.Method()),
			string(c.Request.RequestURI()), t, c.Request.Body())
		if !hmac.Equal([]byte(expected), []byte(sign)) {
			if isDebug {
				c.AbortWithMsg("sign invalid", consts.StatusForbidden)
			} else {
				c.AbortWithStatus(consts.StatusUnauthorized)
			}
			return
		}

		c.Next(ctx)
	}
}

// timestampFresh 判断 Unix 秒级时间戳 t 是否落在 ±window 新鲜度窗口内。
// now 通过参数注入以便测试；window <= 0 时回退默认窗口。
func timestampFresh(t int64, window time.Duration, now func() time.Time) bool {
	if window <= 0 {
		window = defaultTimestampWindow
	}
	diff := now().Unix() - t
	if diff < 0 {
		diff = -diff
	}
	return diff <= int64(window/time.Second)
}

// parseAuthorization 从 X-Signature 头解析 ak, sign, t。
// 时间戳解析失败时返回错误（拒绝请求），不再吞错导致 t=0。
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
	tt, ok := kvs["t"]
	if !ok || tt == "" {
		return "", "", 0, goerror.In("auth.parseAuthorization").
			Code(goerror.CodeParamInvalid).
			Errorf("timestamp is empty")
	}
	parsed, perr := strconv.ParseInt(tt, 10, 64)
	if perr != nil {
		return "", "", 0, goerror.In("auth.parseAuthorization").
			Code(goerror.CodeParamInvalid).
			Wrap(perr)
	}
	t = parsed

	return ak, sign, t, nil
}

// signRequest 计算请求签名。
//
// 规范签名格式（client 与 server 必须一致）：
//
//	stringToSign = ak + "\n" + method + "\n" + requestURI + "\n" + timestamp + "\n" + sha256hex(body)
//	signature    = hex( HMAC-SHA256( key = sk, msg = stringToSign ) )
//
// method 为大写 HTTP 方法；requestURI 为 origin-form 请求目标（path?query）；
// timestamp 为十进制秒级时间戳；sha256hex(body) 为原始 body 的 SHA-256 小写十六进制
// （无 body 时为空输入的 SHA-256）。
func signRequest(ak, sk, method, requestURI string, t int64, body []byte) string {
	bodyHash := sha256.Sum256(body)
	msg := strings.Join([]string{
		ak,
		method,
		requestURI,
		strconv.FormatInt(t, 10),
		hex.EncodeToString(bodyHash[:]),
	}, "\n")
	h := hmac.New(sha256.New, []byte(sk))
	h.Write([]byte(msg))
	return hex.EncodeToString(h.Sum(nil))
}
