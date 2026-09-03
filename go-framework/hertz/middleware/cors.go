package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
)

// corsConfig 存储 Cors 配置选项。
type corsConfig struct {
	allowOrigins     []string
	allowHeaders     []string
	allowMethods     []string
	exposeHeaders    []string
	allowCredentials bool
}

// CorsOption 定义 Cors 配置选项函数。
type CorsOption func(*corsConfig)

// WithAllowOrigins 设置允许跨域的 Origin 白名单。空 slice 忽略，保持默认 ["*"]。
// 显式设置为非 ["*"] 值后，仅当请求 Origin 命中白名单时才回显该 Origin；
// 未命中时不设置 Access-Control-Allow-Origin 响应头。
func WithAllowOrigins(origins []string) CorsOption {
	return func(c *corsConfig) {
		if len(origins) > 0 {
			c.allowOrigins = origins
		}
	}
}

// WithAllowHeaders 设置 Access-Control-Allow-Headers 列表。空 slice 忽略。
func WithAllowHeaders(headers []string) CorsOption {
	return func(c *corsConfig) {
		if len(headers) > 0 {
			c.allowHeaders = headers
		}
	}
}

// WithAllowMethods 设置 Access-Control-Allow-Methods 列表。空 slice 忽略。
func WithAllowMethods(methods []string) CorsOption {
	return func(c *corsConfig) {
		if len(methods) > 0 {
			c.allowMethods = methods
		}
	}
}

// WithExposeHeaders 设置 Access-Control-Expose-Headers 列表。空 slice 忽略。
func WithExposeHeaders(headers []string) CorsOption {
	return func(c *corsConfig) {
		if len(headers) > 0 {
			c.exposeHeaders = headers
		}
	}
}

// WithAllowCredentials 设置 Access-Control-Allow-Credentials。
func WithAllowCredentials(allow bool) CorsOption {
	return func(c *corsConfig) {
		c.allowCredentials = allow
	}
}

// defaultAllowOrigins Cors 默认 Origin 白名单（通配符，保持历史行为）。
var defaultAllowOrigins = []string{"*"}

// defaultAllowHeaders Cors 默认允许的请求头列表。
var defaultAllowHeaders = []string{"Content-Type,X-Authorization, X-Signature"}

// defaultAllowMethods Cors 默认允许的请求方法列表。
var defaultAllowMethods = []string{"POST, GET, OPTIONS,DELETE,PUT"}

// defaultExposeHeaders Cors 默认暴露的响应头列表。
var defaultExposeHeaders = []string{"Content-Length, Access-Control-Allow-Origin,New-Token,New-Expires-At,Access-Control-Allow-Headers, Content-Type"}

// Cors 返回 CORS 中间件，允许跨域请求。
// 默认行为与历史版本完全一致：Origin 为 "*"，Header/Method/ExposeHeaders 为固定列表，
// Access-Control-Allow-Credentials 为 true。可通过 Option 定制：
//
//	engine.Use(middleware.Cors())
//
//	// 限制 Origin 白名单：
//	engine.Use(middleware.Cors(middleware.WithAllowOrigins([]string{"https://app.example.com"})))
func Cors(opts ...CorsOption) app.HandlerFunc {
	cfg := corsConfig{
		allowOrigins:     defaultAllowOrigins,
		allowHeaders:     defaultAllowHeaders,
		allowMethods:     defaultAllowMethods,
		exposeHeaders:    defaultExposeHeaders,
		allowCredentials: true,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	usesWildcard := len(cfg.allowOrigins) == 1 && cfg.allowOrigins[0] == "*"
	allowHeaders := strings.Join(cfg.allowHeaders, ",")
	allowMethods := strings.Join(cfg.allowMethods, ",")
	exposeHeaders := strings.Join(cfg.exposeHeaders, ",")

	return func(c context.Context, ctx *app.RequestContext) {
		if usesWildcard {
			ctx.Header("Access-Control-Allow-Origin", "*")
		} else if origin := string(ctx.Request.Header.Peek("Origin")); origin != "" && originAllowed(origin, cfg.allowOrigins) {
			ctx.Header("Access-Control-Allow-Origin", origin)
		}
		ctx.Header("Access-Control-Allow-Headers", allowHeaders)
		ctx.Header("Access-Control-Allow-Methods", allowMethods)
		ctx.Header("Access-Control-Expose-Headers", exposeHeaders)
		if cfg.allowCredentials {
			ctx.Header("Access-Control-Allow-Credentials", "true")
		} else {
			ctx.Header("Access-Control-Allow-Credentials", "false")
		}

		if string(ctx.Request.Method()) == "OPTIONS" {
			ctx.AbortWithStatus(http.StatusNoContent)
		}
		ctx.Next(c)
	}
}

// originAllowed 判断 origin 是否命中白名单。
func originAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == origin {
			return true
		}
	}
	return false
}
