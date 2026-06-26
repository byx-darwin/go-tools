package handler

import (
	"context"

	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/byx-darwin/go-tools/go-middleware/db"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// dbClient 数据库实例（由 main 通过 SetDBClient 注入）。
var dbClient *db.DB

// SetDBClient 注入数据库实例（在 main 中调用）。
func SetDBClient(d *db.DB) {
	dbClient = d
}

// RegisterDBRoutes 注册 db 示例路由。
func RegisterDBRoutes(h *server.Hertz) {
	h.GET("/middleware/db", dbHandler)
}

// dbHandler Ping 数据库，演示连通性。
//
// DB 未配置时返回 "db not configured"。
func dbHandler(ctx context.Context, c *app.RequestContext) {
	if dbClient == nil {
		hertzresp.Success(c, map[string]any{
			"status":  "not_configured",
			"message": "db not configured",
		})
		return
	}

	pingErr := dbClient.Ping(ctx)

	result := map[string]any{}
	if pingErr != nil {
		result["ping_error"] = pingErr.Error()
		result["healthy"] = false
	} else {
		result["healthy"] = true
		result["ping"] = "ok"
	}

	hertzresp.Success(c, result)
}
