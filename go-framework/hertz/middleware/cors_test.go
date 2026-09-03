package middleware

import (
	"context"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/stretchr/testify/assert"
)

func newCorsTestEngine() *route.Engine {
	opt := config.NewOptions([]config.Option{})
	return route.NewEngine(opt)
}

func TestCors_DefaultBehavior_Unchanged(t *testing.T) {
	engine := newCorsTestEngine()
	engine.Use(Cors())
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Origin", Value: "https://example.com"})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
	assert.Equal(t, "*", string(res.Header.Peek("Access-Control-Allow-Origin")))
	assert.Equal(t, "Content-Type,X-Authorization, X-Signature", string(res.Header.Peek("Access-Control-Allow-Headers")))
	assert.Equal(t, "POST, GET, OPTIONS,DELETE,PUT", string(res.Header.Peek("Access-Control-Allow-Methods")))
	assert.Equal(t, "Content-Length, Access-Control-Allow-Origin,New-Token,New-Expires-At,Access-Control-Allow-Headers, Content-Type", string(res.Header.Peek("Access-Control-Expose-Headers")))
	assert.Equal(t, "true", string(res.Header.Peek("Access-Control-Allow-Credentials")))
}

func TestCors_DefaultBehavior_OptionsRequest_NoContent(t *testing.T) {
	engine := newCorsTestEngine()
	engine.Use(Cors())
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "OPTIONS", "/test", &ut.Body{Body: nil})
	res := w.Result()
	assert.Equal(t, http.StatusNoContent, res.StatusCode())
}

func TestCors_AllowOrigins_WhitelistMatch(t *testing.T) {
	engine := newCorsTestEngine()
	engine.Use(Cors(WithAllowOrigins([]string{"https://a.example.com", "https://b.example.com"})))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Origin", Value: "https://a.example.com"})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
	assert.Equal(t, "https://a.example.com", string(res.Header.Peek("Access-Control-Allow-Origin")))
}

func TestCors_AllowOrigins_WhitelistMiss_HeaderOmitted(t *testing.T) {
	engine := newCorsTestEngine()
	engine.Use(Cors(WithAllowOrigins([]string{"https://a.example.com"})))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Origin", Value: "https://evil.example.com"})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
	assert.Equal(t, "", string(res.Header.Peek("Access-Control-Allow-Origin")))
}

func TestCors_CustomHeadersMethodsExposeCredentials(t *testing.T) {
	engine := newCorsTestEngine()
	engine.Use(Cors(
		WithAllowHeaders([]string{"Content-Type", "X-Custom"}),
		WithAllowMethods([]string{"GET", "POST"}),
		WithExposeHeaders([]string{"X-Total-Count"}),
		WithAllowCredentials(false),
	))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Origin", Value: "https://example.com"})
	res := w.Result()
	assert.Equal(t, "Content-Type,X-Custom", string(res.Header.Peek("Access-Control-Allow-Headers")))
	assert.Equal(t, "GET,POST", string(res.Header.Peek("Access-Control-Allow-Methods")))
	assert.Equal(t, "X-Total-Count", string(res.Header.Peek("Access-Control-Expose-Headers")))
	assert.Equal(t, "false", string(res.Header.Peek("Access-Control-Allow-Credentials")))
}

func TestCors_EmptyOptionValuesIgnored(t *testing.T) {
	engine := newCorsTestEngine()
	// 空 slice 不应覆盖默认值。
	engine.Use(Cors(WithAllowOrigins(nil), WithAllowHeaders(nil), WithAllowMethods(nil), WithExposeHeaders(nil)))
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "Origin", Value: "https://example.com"})
	res := w.Result()
	assert.Equal(t, "*", string(res.Header.Peek("Access-Control-Allow-Origin")))
	assert.Equal(t, "Content-Type,X-Authorization, X-Signature", string(res.Header.Peek("Access-Control-Allow-Headers")))
}
