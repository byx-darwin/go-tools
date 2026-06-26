package handler

import (
	"context"
	"io"

	elasticsearchv8 "github.com/elastic/go-elasticsearch/v8"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// esClient Elasticsearch 客户端实例（由 main 通过 SetESClient 注入）。
var esClient *elasticsearchv8.Client

// SetESClient 注入 ES 客户端（在 main 中调用）。
func SetESClient(c *elasticsearchv8.Client) {
	esClient = c
}

// RegisterESRoutes 注册 es 示例路由。
func RegisterESRoutes(h *server.Hertz) {
	h.GET("/middleware/es", esHandler)
}

// esHandler 调用 ES ClusterHealth API 演示连通性。
//
// ES 未配置时返回 "es not configured"。
func esHandler(_ context.Context, c *app.RequestContext) {
	if esClient == nil {
		hertzresp.Success(c, map[string]any{
			"status":  "not_configured",
			"message": "es not configured",
		})
		return
	}

	res, err := esClient.Cluster.Health()
	result := map[string]any{}

	if err != nil {
		result["health_error"] = err.Error()
	} else {
		defer func() { _ = res.Body.Close() }()
		body, _ := io.ReadAll(res.Body)
		result["status_code"] = res.StatusCode
		result["body"] = string(body)
		if res.IsError() {
			result["is_error"] = true
		}
	}

	hertzresp.Success(c, result)
}
