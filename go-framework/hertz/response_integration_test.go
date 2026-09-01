package hertz

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authjwt "github.com/byx-darwin/go-tools/go-auth/jwt"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
	"github.com/byx-darwin/go-tools/go-framework/hertz/middleware"
)

// setupHertzEngine 创建测试用 Hertz engine。
func setupHertzEngine(t *testing.T, r *Responder) *route.Engine {
	t.Helper()
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(r.Middleware())
	engine.GET("/success", func(ctx context.Context, c *app.RequestContext) {
		resp := RespondFrom(c)
		resp.Success(c, map[string]string{"id": "123"})
	})
	engine.GET("/success-msg", func(ctx context.Context, c *app.RequestContext) {
		resp := RespondFrom(c)
		resp.SuccessWithMsg(ctx, c, nil, "操作成功")
	})
	engine.GET("/error", func(ctx context.Context, c *app.RequestContext) {
		resp := RespondFrom(c)
		err := frameworkerror.ErrParamInvalid.Wrap(errors.New("field 'name' is empty"))
		resp.Error(ctx, c, err, "参数无效")
	})
	engine.GET("/error-plain", func(ctx context.Context, c *app.RequestContext) {
		resp := RespondFrom(c)
		resp.Error(ctx, c, errors.New("something broke"), "操作失败")
	})
	engine.GET("/error-with-code", func(ctx context.Context, c *app.RequestContext) {
		resp := RespondFrom(c)
		resp.ErrorWithCode(ctx, c, http.StatusForbidden, 40300, "禁止访问")
	})
	engine.GET("/error-via-context", func(ctx context.Context, c *app.RequestContext) {
		err := frameworkerror.ErrParamInvalid.Wrap(errors.New("via context"))
		_ = c.AbortWithError(http.StatusBadRequest, err)
	})
	engine.GET("/request-id", func(ctx context.Context, c *app.RequestContext) {
		id := RequestIDFrom(c)
		c.JSON(http.StatusOK, map[string]string{"request_id": id})
	})
	engine.GET("/reply-json", func(ctx context.Context, c *app.RequestContext) {
		resp := RespondFrom(c)
		resp.Reply(c, http.StatusCreated, map[string]int{"id": 1})
	})
	return engine
}

// ── Success Tests ──

func TestResponder_Success_Integration(t *testing.T) {
	r := NewResponder()
	engine := setupHertzEngine(t, r)

	w := ut.PerformRequest(engine, http.MethodGet, "/success", nil)

	require.Equal(t, http.StatusOK, w.Code)

	// 非 protobuf 数据使用通用 map 结构序列化
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(http.StatusOK), resp["code"])
	assert.Equal(t, "ok", resp["msg"])
	assert.Equal(t, map[string]any{"id": "123"}, resp["data"])
}

func TestResponder_SuccessWithMsg_Integration(t *testing.T) {
	r := NewResponder()
	engine := setupHertzEngine(t, r)

	w := ut.PerformRequest(engine, http.MethodGet, "/success-msg", nil)

	require.Equal(t, http.StatusOK, w.Code)

	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int32(http.StatusOK), resp.Code)
	assert.Equal(t, "操作成功", resp.Msg)
}

// ── Error Tests ──

func TestResponder_Error_RPCRouting(t *testing.T) {
	r := NewResponder(WithErrorRouter(&RPCErrorRouter{}))
	engine := setupHertzEngine(t, r)

	w := ut.PerformRequest(engine, http.MethodGet, "/error", nil)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int32(frameworkerror.CodeParamInvalid), resp.Code)
	assert.Equal(t, "param_invalid", resp.Msg)
}

func TestResponder_Error_PlainError(t *testing.T) {
	r := NewResponder(WithErrorRouter(&RPCErrorRouter{}))
	engine := setupHertzEngine(t, r)

	w := ut.PerformRequest(engine, http.MethodGet, "/error-plain", nil)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int32(http.StatusInternalServerError), resp.Code)
	assert.Contains(t, resp.Msg, "操作失败")
}

func TestResponder_ErrorWithCode(t *testing.T) {
	r := NewResponder()
	engine := setupHertzEngine(t, r)

	w := ut.PerformRequest(engine, http.MethodGet, "/error-with-code", nil)

	require.Equal(t, http.StatusForbidden, w.Code)

	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int32(40300), resp.Code)
	assert.Equal(t, "禁止访问", resp.Msg)
}

// ── Debug 模式 ──

func TestResponder_Error_DebugMode(t *testing.T) {
	r := NewResponder(WithDebug(true), WithErrorRouter(&RPCErrorRouter{}))
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(r.Middleware())
	engine.GET("/debug-error", func(ctx context.Context, c *app.RequestContext) {
		resp := RespondFrom(c)
		err := frameworkerror.ErrParamInvalid.Wrap(errors.New("sensitive detail"))
		resp.Error(ctx, c, err, "参数无效")
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/debug-error", nil)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp.Msg, "internal:")
	assert.Contains(t, resp.Msg, "sensitive detail")
}

// ── Request ID 测试 ──

func TestResponder_RequestID_Header(t *testing.T) {
	r := NewResponder(WithRequestIDGenerator(func() string { return "gen-abc-123" }))
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(r.Middleware())
	engine.GET("/id", func(ctx context.Context, c *app.RequestContext) {
		id := RequestIDFrom(c)
		c.JSON(http.StatusOK, map[string]string{"request_id": id})
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/id", nil)

	require.Equal(t, http.StatusOK, w.Code)

	// 验证响应头包含 X-Request-ID
	assert.Equal(t, "gen-abc-123", w.Header().Get("X-Request-ID"))

	// 验证 ctx 中可读取
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "gen-abc-123", body["request_id"])
}

func TestResponder_RequestID_FromIncomingHeader(t *testing.T) {
	r := NewResponder()
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(r.Middleware())
	engine.GET("/id", func(ctx context.Context, c *app.RequestContext) {
		id := RequestIDFrom(c)
		c.JSON(http.StatusOK, map[string]string{"request_id": id})
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/id", nil,
		ut.Header{Key: "X-Request-ID", Value: "client-sent-id"})

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "client-sent-id", body["request_id"])
}

// ── Content Negotiation ──

func TestResponder_Reply_JSON(t *testing.T) {
	r := NewResponder()
	engine := setupHertzEngine(t, r)

	w := ut.PerformRequest(engine, http.MethodGet, "/reply-json", nil)

	require.Equal(t, http.StatusCreated, w.Code)

	var resp map[string]int
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp["id"])
}

// ── Middleware 未注入时使用 Default Responder ──

func TestRespondFrom_DefaultWhenNoMiddleware(t *testing.T) {
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.GET("/no-middleware", func(ctx context.Context, c *app.RequestContext) {
		resp := RespondFrom(c)
		assert.NotNil(t, resp)
		assert.False(t, resp.debug) // 默认值
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/no-middleware", nil)
	require.Equal(t, http.StatusOK, w.Code)
}

// ── Translator 集成测试 ──

func TestResponder_WithTranslator(t *testing.T) {
	tr := &mockTranslator{
		translate: func(ctx context.Context, lang, key string) string {
			return "已翻译-" + key
		},
	}
	r := NewResponder(WithTranslator(tr))
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(r.Middleware())
	engine.GET("/translated", func(ctx context.Context, c *app.RequestContext) {
		resp := RespondFrom(c)
		resp.SuccessWithMsg(ctx, c, nil, "success_message")
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/translated", nil)

	require.Equal(t, http.StatusOK, w.Code)

	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "已翻译-success_message", resp.Msg)
}

// ── anypb.Any JSON 展开测试 ──

// TestResponder_Success_ProtoData_AnyExpansion 验证 protobuf 消息作为 data 时，
// JSON 序列化会展开 anypb.Any（而非 {type_url, value} 原始格式）。
func TestResponder_Success_ProtoData_AnyExpansion(t *testing.T) {
	r := NewResponder()
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(r.Middleware())
	engine.GET("/success-proto", func(ctx context.Context, c *app.RequestContext) {
		resp := RespondFrom(c)
		// 使用 Response 作为 protobuf 消息测试数据
		resp.Success(c, &Response{Code: 1, Msg: "inner"})
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/success-proto", nil)

	require.Equal(t, http.StatusOK, w.Code)

	body := w.Body.String()
	// 验证 anypb.Any 被展开，包含内部字段
	assert.Contains(t, body, `"code"`)
	assert.Contains(t, body, `"msg"`)
	// 不应包含 @type 字段（protojson 展开 Any 时默认会带 @type，需要移除）
	assert.NotContains(t, body, `"@type"`)
	// 不应包含 type_url 和 value 字段（未展开的 anypb.Any 格式）
	assert.NotContains(t, body, `"type_url"`)
	assert.NotContains(t, body, `"value"`)
}

func TestResponder_Middleware_RewritesContextError(t *testing.T) {
	r := NewResponder()
	engine := setupHertzEngine(t, r)

	w := ut.PerformRequest(engine, http.MethodGet, "/error-via-context", nil)

	// AbortWithError 已经写出正确状态码；Middleware() 收尾后 body 应变成
	// 完整协商后的 JSON（而不是空 body）。
	require.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(frameworkerror.CodeParamInvalid), resp["code"])
	assert.Equal(t, "param_invalid", resp["msg"])
}

func TestResponder_Middleware_NoErrorsNoOverwrite(t *testing.T) {
	r := NewResponder()
	engine := setupHertzEngine(t, r)

	w := ut.PerformRequest(engine, http.MethodGet, "/success", nil)

	// 成功路径 c.Errors 为空，收尾逻辑不应有任何影响。
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "ok", resp["msg"])
}

// ── JWTAuth + Responder.Middleware() 联通测试 ──
//
// 验证 go-framework/hertz/middleware 的 JWTAuth（用 c.AbortWithError 记录
// 错误）与本包 Responder.Middleware()（读取 c.Errors 补齐协商响应体）两者
// 组合后，能产出正确的状态码 + 业务码 + 消息，而不只是分别测试。

type integrationClaims struct {
	gojwt.RegisteredClaims
}

func TestJWTAuthWithResponderMiddleware_TokenInvalid(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	r := NewResponder()

	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(r.Middleware())
	engine.Use(middleware.JWTAuth[integrationClaims](secret))
	engine.GET("/protected", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/protected", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer not-a-valid-token"})

	require.Equal(t, http.StatusUnauthorized, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(40001), resp["code"]) // autherror.CodeTokenInvalid
	assert.Equal(t, "token_invalid", resp["msg"])
}

func TestJWTAuthWithResponderMiddleware_Success(t *testing.T) {
	secret := []byte("test-secret-key-32bytes-long!!!!!")
	r := NewResponder()

	claims := integrationClaims{
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token, err := authjwt.Sign(claims, secret)
	require.NoError(t, err)

	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(r.Middleware())
	engine.Use(middleware.JWTAuth[integrationClaims](secret))
	engine.GET("/protected", func(ctx context.Context, c *app.RequestContext) {
		resp := RespondFrom(c)
		resp.Success(c, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/protected", &ut.Body{Body: nil},
		ut.Header{Key: "Authorization", Value: "Bearer " + token})

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "ok", resp["msg"])
}
