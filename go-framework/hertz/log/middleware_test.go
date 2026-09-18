package hertzlog_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/byx-darwin/go-tools/go-common/log"
	hertzlog "github.com/byx-darwin/go-tools/go-framework/hertz/log"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/stretchr/testify/assert"
)

func newMiddlewareTestEngine() *route.Engine {
	opt := config.NewOptions([]config.Option{})
	return route.NewEngine(opt)
}

func TestHertzRequestIDMiddleware_HeaderPresent(t *testing.T) {
	engine := newMiddlewareTestEngine()
	engine.Use(hertzlog.HertzRequestIDMiddleware())
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.String(http.StatusOK, log.RequestIDFromContext(ctx))
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil},
		ut.Header{Key: "X-Request-ID", Value: "req-abc-123"})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
	assert.Equal(t, "req-abc-123", string(res.Body()))
}

func TestHertzRequestIDMiddleware_HeaderAbsent(t *testing.T) {
	engine := newMiddlewareTestEngine()
	engine.Use(hertzlog.HertzRequestIDMiddleware())
	engine.GET("/test", func(ctx context.Context, c *app.RequestContext) {
		c.String(http.StatusOK, log.RequestIDFromContext(ctx))
	})

	w := ut.PerformRequest(engine, "GET", "/test", &ut.Body{Body: nil})
	res := w.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode())
	assert.Equal(t, "", string(res.Body()))
}
