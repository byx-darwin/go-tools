package hertz

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		err := goerror.ErrParamInvalid.Wrap(errors.New("field 'name' is empty"))
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

	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, "ok", resp.Msg)
	assert.Equal(t, map[string]any{"id": "123"}, resp.Data)
}

func TestResponder_SuccessWithMsg_Integration(t *testing.T) {
	r := NewResponder()
	engine := setupHertzEngine(t, r)

	w := ut.PerformRequest(engine, http.MethodGet, "/success-msg", nil)

	require.Equal(t, http.StatusOK, w.Code)

	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, http.StatusOK, resp.Code)
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
	assert.Equal(t, goerror.CodeParamInvalid, resp.Code)
	assert.Equal(t, "param_invalid", resp.Msg)
}

func TestResponder_Error_PlainError(t *testing.T) {
	r := NewResponder(WithErrorRouter(&RPCErrorRouter{}))
	engine := setupHertzEngine(t, r)

	w := ut.PerformRequest(engine, http.MethodGet, "/error-plain", nil)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, http.StatusInternalServerError, resp.Code)
	assert.Contains(t, resp.Msg, "操作失败")
}

func TestResponder_ErrorWithCode(t *testing.T) {
	r := NewResponder()
	engine := setupHertzEngine(t, r)

	w := ut.PerformRequest(engine, http.MethodGet, "/error-with-code", nil)

	require.Equal(t, http.StatusForbidden, w.Code)

	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 40300, resp.Code)
	assert.Equal(t, "禁止访问", resp.Msg)
}

// ── Debug 模式 ──

func TestResponder_Error_DebugMode(t *testing.T) {
	r := NewResponder(WithDebug(true), WithErrorRouter(&RPCErrorRouter{}))
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(r.Middleware())
	engine.GET("/debug-error", func(ctx context.Context, c *app.RequestContext) {
		resp := RespondFrom(c)
		err := goerror.ErrParamInvalid.Wrap(errors.New("sensitive detail"))
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

// ── 废弃 API 兼容测试 ──

func TestDeprecated_OK(t *testing.T) {
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(NewResponder().Middleware())
	engine.GET("/deprecated-ok", func(ctx context.Context, c *app.RequestContext) {
		OK(c, map[string]string{"x": "y"})
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/deprecated-ok", nil)

	require.Equal(t, http.StatusOK, w.Code)

	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, http.StatusOK, resp.Code)
}

func TestDeprecated_Err(t *testing.T) {
	engine := route.NewEngine(config.NewOptions([]config.Option{}))
	engine.Use(NewResponder().Middleware())
	engine.GET("/deprecated-err", func(ctx context.Context, c *app.RequestContext) {
		Err(c, errors.New("something broke"))
	})

	w := ut.PerformRequest(engine, http.MethodGet, "/deprecated-err", nil)
	require.Equal(t, http.StatusInternalServerError, w.Code)
}
