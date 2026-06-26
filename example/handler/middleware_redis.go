package handler

import (
	"context"

	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/byx-darwin/go-tools/go-middleware/redis"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// redisClient Redis 客户端实例（由 main 通过 SetRedisClient 注入）。
var redisClient redis.Client

// SetRedisClient 注入 Redis 客户端（在 main 中调用）。
func SetRedisClient(c redis.Client) {
	redisClient = c
}

// RegisterRedisRoutes 注册 redis 示例路由。
func RegisterRedisRoutes(h *server.Hertz) {
	h.GET("/middleware/redis", redisHandler)
}

// redisHandler SET → GET → DEL 演示。
//
// Redis 未配置时返回 "redis not configured"。
func redisHandler(ctx context.Context, c *app.RequestContext) {
	if redisClient == nil {
		hertzresp.Success(c, map[string]any{
			"status":  "not_configured",
			"message": "redis not configured",
		})
		return
	}

	key := "example:go-tools"
	value := "hello-middleware"

	setErr := redisClient.Set(ctx, key, value, 0).Err()
	getResult := redisClient.Get(ctx, key)
	delErr := redisClient.Del(ctx, key).Err()

	result := map[string]any{
		"key":     key,
		"value":   value,
		"set_err": setErr,
		"del_err": delErr,
	}
	if getErr := getResult.Err(); getErr != nil {
		result["get_err"] = getErr
	} else {
		result["get_value"] = getResult.Val()
	}

	hertzresp.Success(c, result)
}
