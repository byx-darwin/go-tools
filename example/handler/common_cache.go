package handler

import (
	"context"

	"github.com/byx-darwin/go-tools/go-common/cache"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterCacheRoutes 注册 cache 示例路由。
func RegisterCacheRoutes(h *server.Hertz) {
	h.GET("/common/cache", cacheHandler)
}

func cacheHandler(_ context.Context, c *app.RequestContext) {
	// LRU 缓存，容量 100。
	cch := cache.New[string, int](cache.LRU, 100).Build()

	// Set / Get。
	cch.Set("counter", 42)
	val, ok, _ := cch.Get("counter")

	// Delete。
	cch.Set("temp", 99)
	deleted := cch.Delete("temp")
	_, tempOK, _ := cch.Get("temp")

	results := map[string]any{
		"algorithm":     "LRU",
		"capacity":      100,
		"set_key":       "counter",
		"set_value":     42,
		"get_found":     ok,
		"get_value":     val,
		"deleted_key":   "temp",
		"delete_result": deleted, // 应为 true
		"after_delete":  tempOK,  // 应为 false
	}

	hertzresp.Success(c, results)
}
