package handler

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// chClient ClickHouse 客户端实例（由 main 通过 SetClickHouseClient 注入）。
var chClient clickhouse.Conn

// SetClickHouseClient 注入 ClickHouse 客户端（在 main 中调用）。
func SetClickHouseClient(c clickhouse.Conn) {
	chClient = c
}

// RegisterClickHouseRoutes 注册 clickhouse 示例路由。
func RegisterClickHouseRoutes(h *server.Hertz) {
	h.GET("/middleware/clickhouse", clickhouseHandler)
}

// clickhouseHandler 执行 SELECT 1 演示连通性。
//
// ClickHouse 未配置时返回 "clickhouse not configured"。
func clickhouseHandler(ctx context.Context, c *app.RequestContext) {
	if chClient == nil {
		hertzresp.Success(c, map[string]any{
			"status":  "not_configured",
			"message": "clickhouse not configured",
		})
		return
	}

	var result any
	queryErr := chClient.QueryRow(ctx, "SELECT 1").Scan(&result)

	resp := map[string]any{}
	if queryErr != nil {
		resp["query_error"] = queryErr.Error()
	} else {
		resp["result"] = result
		resp["healthy"] = true
	}

	hertzresp.Success(c, resp)
}
