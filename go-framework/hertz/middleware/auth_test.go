package middleware

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/stretchr/testify/assert"
)

// ─────────────────────────────────────────────────────────────
// signRequest 单元测试（#22 规范签名格式）
// ─────────────────────────────────────────────────────────────

func TestSignRequest_Deterministic(t *testing.T) {
	body := []byte(`{"a":1}`)
	s1 := signRequest("ak", "sk", "POST", "/p?x=1", 1700000000, body)
	s2 := signRequest("ak", "sk", "POST", "/p?x=1", 1700000000, body)
	assert.Equal(t, s1, s2)
	assert.Len(t, s1, 64, "HMAC-SHA256 hex should be 64 chars")
}

func TestSignRequest_BindsMethodQueryBody(t *testing.T) {
	base := signRequest("ak", "sk", "GET", "/t?amount=1", 1700000000, nil)

	assert.NotEqual(t, base, signRequest("ak", "sk", "POST", "/t?amount=1", 1700000000, nil),
		"different method must change signature")
	assert.NotEqual(t, base, signRequest("ak", "sk", "GET", "/t?amount=9999", 1700000000, nil),
		"different query must change signature")
	assert.NotEqual(t, base, signRequest("ak", "sk", "GET", "/t?amount=1", 1700000000, []byte("x")),
		"different body must change signature")
	assert.NotEqual(t, base, signRequest("other", "sk", "GET", "/t?amount=1", 1700000000, nil))
	assert.NotEqual(t, base, signRequest("ak", "sk2", "GET", "/t?amount=1", 1700000000, nil))
	assert.NotEqual(t, base, signRequest("ak", "sk", "GET", "/t?amount=1", 1700000001, nil))
}

func TestSignRequest_EmptyBodyUsesEmptySHA256(t *testing.T) {
	empty := sha256.Sum256(nil)
	assert.Len(t, hex.EncodeToString(empty[:]), 64)

	s1 := signRequest("ak", "sk", "GET", "/p", 1, nil)
	s2 := signRequest("ak", "sk", "GET", "/p", 1, []byte{})
	assert.Equal(t, s1, s2, "nil body and empty body must hash identically")
}

func TestSignRequest_MessageFormat(t *testing.T) {
	// 手工构造期望的规范化消息，固化签名格式契约。
	ak, sk, method, uri := "ak", "sk", "GET", "/p"
	var ts int64 = 1
	body := []byte("hello")
	bodyHash := sha256.Sum256(body)
	msg := fmt.Sprintf("%s\n%s\n%s\n%d\n%s", ak, method, uri, ts, hex.EncodeToString(bodyHash[:]))

	mac := hmac.New(sha256.New, []byte(sk))
	mac.Write([]byte(msg))
	want := hex.EncodeToString(mac.Sum(nil))
	assert.Equal(t, want, signRequest(ak, sk, method, uri, ts, body))
}

func TestSignRequest_HexFormat(t *testing.T) {
	sign := signRequest("ak", "sk", "GET", "/p", 12345, nil)
	for _, ch := range sign {
		assert.True(t, (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f'),
			"sign must be lowercase hex, got '%c'", ch)
	}
}

// ─────────────────────────────────────────────────────────────
// timestampFresh 单元测试（#23 新鲜度窗口）
// ─────────────────────────────────────────────────────────────

func TestTimestampFresh(t *testing.T) {
	now := func() time.Time { return time.Unix(1700000000, 0) }
	win := 5 * time.Minute

	assert.True(t, timestampFresh(1700000000, win, now), "exact now is fresh")
	assert.True(t, timestampFresh(1700000000-299, win, now), "within past window")
	assert.True(t, timestampFresh(1700000000+299, win, now), "within future window")
	assert.True(t, timestampFresh(1700000000-300, win, now), "boundary is fresh")
	assert.False(t, timestampFresh(1700000000-301, win, now), "expired past")
	assert.False(t, timestampFresh(1700000000+301, win, now), "too far future")
	assert.False(t, timestampFresh(0, win, now), "t=0 must be stale")
}

// ─────────────────────────────────────────────────────────────
// 中间件级测试（hertz ut）
// ─────────────────────────────────────────────────────────────

// fakeAuthFace 测试用 AuthFace 实现。
type fakeAuthFace struct {
	sk      string
	isDebug bool
	err     error
}

func (f *fakeAuthFace) GetSk(_ context.Context, _ *app.RequestContext, _ string, _ int64) (string, bool, error) {
	return f.sk, f.isDebug, f.err
}

// signHeader 构造 X-Signature 头：Base64(ak=..&sign=..&t=..)。
func signHeader(ak, sk, method, requestURI string, t int64, body []byte) string {
	sign := signRequest(ak, sk, method, requestURI, t, body)
	raw := fmt.Sprintf("ak=%s&sign=%s&t=%d", ak, sign, t)
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// newAuthTestEngine 构造带 Auth 中间件与 /test(GET/POST) 路由的测试引擎。
func newAuthTestEngine(face AuthFace, opts ...Option) *route.Engine {
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(Auth(face, opts...))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.String(200, "ok")
	})
	engine.POST("/test", func(ctx context.Context, c *app.RequestContext) {
		c.String(200, "ok")
	})
	return engine
}

// utBody 构造带正确 Len 的 ut.Body（hertz ut 需要 Len 才会把 body 写入请求）。
func utBody(b []byte) *ut.Body {
	return &ut.Body{Body: bytes.NewReader(b), Len: len(b)}
}

func TestAuth_ValidSignature_Passes(t *testing.T) {
	now := time.Now().Unix()
	engine := newAuthTestEngine(&fakeAuthFace{sk: "secret"})
	hdr := signHeader("ak1", "secret", "GET", "/test", now, nil)
	w := ut.PerformRequest(engine, "GET", "/test", nil,
		ut.Header{Key: "X-Signature", Value: hdr})
	assert.Equal(t, 200, w.Result().StatusCode())
}

// #21：调试模式验签失败不得回显服务端有效签名（凭据预言机）。
func TestAuth_DebugDoesNotEchoServerSignature(t *testing.T) {
	now := time.Now().Unix()
	face := &fakeAuthFace{sk: "secret", isDebug: true}
	engine := newAuthTestEngine(face)
	badHdr := signHeader("ak1", "wrong-sk", "GET", "/test", now, nil)
	w := ut.PerformRequest(engine, "GET", "/test", nil,
		ut.Header{Key: "X-Signature", Value: badHdr})

	res := w.Result()
	assert.Equal(t, 403, res.StatusCode(), "debug mode rejects with 403")
	body := string(res.Body())
	expected := signRequest("ak1", "secret", "GET", "/test", now, nil)
	assert.NotContains(t, body, expected, "server signature must not be echoed")
	assert.NotContains(t, body, "server:", "legacy 'server:' leak must be gone")
	assert.NotContains(t, body, "client:", "legacy 'client:' leak must be gone")
}

func TestAuth_WrongSignature_NonDebug_401(t *testing.T) {
	now := time.Now().Unix()
	engine := newAuthTestEngine(&fakeAuthFace{sk: "secret"})
	badHdr := signHeader("ak1", "wrong-sk", "GET", "/test", now, nil)
	w := ut.PerformRequest(engine, "GET", "/test", nil,
		ut.Header{Key: "X-Signature", Value: badHdr})
	assert.Equal(t, 401, w.Result().StatusCode())
}

// #22：query 篡改后原签名失效。
func TestAuth_QueryTamper_Rejected(t *testing.T) {
	now := time.Now().Unix()
	engine := newAuthTestEngine(&fakeAuthFace{sk: "secret"})
	// 客户端对 amount=1 签名
	hdr := signHeader("ak1", "secret", "GET", "/test?amount=1", now, nil)
	// 重放到 amount=9999
	w := ut.PerformRequest(engine, "GET", "/test?amount=9999", nil,
		ut.Header{Key: "X-Signature", Value: hdr})
	assert.Equal(t, 401, w.Result().StatusCode(), "query tamper must be rejected")
}

// #22：method 篡改后原签名失效。
func TestAuth_MethodTamper_Rejected(t *testing.T) {
	now := time.Now().Unix()
	engine := newAuthTestEngine(&fakeAuthFace{sk: "secret"})
	hdr := signHeader("ak1", "secret", "GET", "/test", now, nil)
	w := ut.PerformRequest(engine, "POST", "/test", nil,
		ut.Header{Key: "X-Signature", Value: hdr})
	assert.Equal(t, 401, w.Result().StatusCode(), "method tamper must be rejected")
}

// #22：body 篡改后原签名失效。
func TestAuth_BodyTamper_Rejected(t *testing.T) {
	now := time.Now().Unix()
	engine := newAuthTestEngine(&fakeAuthFace{sk: "secret"})
	hdr := signHeader("ak1", "secret", "POST", "/test", now, []byte(`{"a":1}`))
	w := ut.PerformRequest(engine, "POST", "/test", utBody([]byte(`{"a":9999}`)),
		ut.Header{Key: "X-Signature", Value: hdr})
	assert.Equal(t, 401, w.Result().StatusCode(), "body tamper must be rejected")
}

// #22：带 query+body 的合法签名应通过。
func TestAuth_ValidSignature_WithQueryAndBody_Passes(t *testing.T) {
	now := time.Now().Unix()
	engine := newAuthTestEngine(&fakeAuthFace{sk: "secret"})
	hdr := signHeader("ak1", "secret", "POST", "/test?amount=1", now, []byte(`{"a":1}`))
	w := ut.PerformRequest(engine, "POST", "/test?amount=1", utBody([]byte(`{"a":1}`)),
		ut.Header{Key: "X-Signature", Value: hdr})
	assert.Equal(t, 200, w.Result().StatusCode())
}

// #23：过期 / 未来时间戳被拒绝。
func TestAuth_ExpiredTimestamp_Rejected(t *testing.T) {
	stale := time.Now().Add(-10 * time.Minute).Unix()
	engine := newAuthTestEngine(&fakeAuthFace{sk: "secret"})
	hdr := signHeader("ak1", "secret", "GET", "/test", stale, nil)
	w := ut.PerformRequest(engine, "GET", "/test", nil,
		ut.Header{Key: "X-Signature", Value: hdr})
	assert.Equal(t, 401, w.Result().StatusCode())
}

func TestAuth_FutureTimestamp_Rejected(t *testing.T) {
	future := time.Now().Add(10 * time.Minute).Unix()
	engine := newAuthTestEngine(&fakeAuthFace{sk: "secret"})
	hdr := signHeader("ak1", "secret", "GET", "/test", future, nil)
	w := ut.PerformRequest(engine, "GET", "/test", nil,
		ut.Header{Key: "X-Signature", Value: hdr})
	assert.Equal(t, 401, w.Result().StatusCode())
}

// #23：非法时间戳（解析失败）→ 400。
func TestAuth_InvalidTimestamp_BadRequest(t *testing.T) {
	raw := base64.StdEncoding.EncodeToString([]byte("ak=ak1&sign=deadbeef&t=abc"))
	engine := newAuthTestEngine(&fakeAuthFace{sk: "secret"})
	w := ut.PerformRequest(engine, "GET", "/test", nil,
		ut.Header{Key: "X-Signature", Value: raw})
	assert.Equal(t, 400, w.Result().StatusCode())
}

// #23：WithTimestampWindow 放大窗口后，10 分钟前的时间戳应被接受。
func TestAuth_CustomTimestampWindow(t *testing.T) {
	stale := time.Now().Add(-10 * time.Minute).Unix()
	engine := newAuthTestEngine(&fakeAuthFace{sk: "secret"}, WithTimestampWindow(30*time.Minute))
	hdr := signHeader("ak2", "secret", "GET", "/test", stale, nil)
	w := ut.PerformRequest(engine, "GET", "/test", nil,
		ut.Header{Key: "X-Signature", Value: hdr})
	assert.Equal(t, 200, w.Result().StatusCode())
}

// 缺失 X-Signature 头 → 400。
func TestAuth_MissingHeader_BadRequest(t *testing.T) {
	engine := newAuthTestEngine(&fakeAuthFace{sk: "secret"})
	w := ut.PerformRequest(engine, "GET", "/test", nil)
	assert.Equal(t, 400, w.Result().StatusCode())
}
