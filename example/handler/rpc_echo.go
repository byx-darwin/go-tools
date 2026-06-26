package handler

import (
	"context"

	demo "github.com/byx-darwin/go-tools/example/kitex_generated/demo"
	"github.com/byx-darwin/go-tools/example/kitex_generated/demo/demoservice"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// rpcClient Kitex DemoService 客户端引用（由 main.go 注入）。
var rpcClient demoservice.Client

// SetRPCClient 注入 Kitex RPC 客户端。
func SetRPCClient(c demoservice.Client) {
	rpcClient = c
}

// RegisterRPCRoutes 注册 RPC 示例路由（Hertz → Kitex 调用）。
func RegisterRPCRoutes(h *server.Hertz) {
	h.GET("/rpc/echo", rpcEchoHandler)
	h.GET("/rpc/health", rpcHealthHandler)
}

// rpcEchoHandler 通过 Kitex 客户端调用 Echo RPC。
//
// GET /rpc/echo?message=hello
// 将 HTTP 请求转发到 Kitex RPC 服务，返回 Echo 响应。
func rpcEchoHandler(ctx context.Context, c *app.RequestContext) {
	if rpcClient == nil {
		hertzresp.ErrorWithCode(ctx, c, 503, 10001, "RPC client not initialized")
		return
	}

	msg := c.Query("message")
	if msg == "" {
		msg = "hello from hertz"
	}

	resp, err := rpcClient.Echo(ctx, &demo.EchoRequest{Message: msg})
	if err != nil {
		hertzresp.Error(ctx, c, err, "RPC Echo 调用失败")
		return
	}

	hertzresp.Success(c, map[string]any{
		"message": resp.GetMessage(),
		"service": resp.GetService(),
		"source":  "kitex_rpc",
	})
}

// rpcHealthHandler 通过 Kitex 客户端调用 Health RPC。
//
// GET /rpc/health
// 调用 Kitex RPC 服务的健康检查接口。
func rpcHealthHandler(ctx context.Context, c *app.RequestContext) {
	if rpcClient == nil {
		hertzresp.ErrorWithCode(ctx, c, 503, 10001, "RPC client not initialized")
		return
	}

	resp, err := rpcClient.Health(ctx, &demo.HealthRequest{})
	if err != nil {
		hertzresp.Error(ctx, c, err, "RPC Health 调用失败")
		return
	}

	hertzresp.Success(c, map[string]any{
		"healthy": resp.GetHealthy(),
		"version": resp.GetVersion(),
		"source":  "kitex_rpc",
	})
}
