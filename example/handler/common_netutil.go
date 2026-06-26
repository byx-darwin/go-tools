package handler

import (
	"context"

	"github.com/byx-darwin/go-tools/go-common/netutil"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterNetutilRoutes 注册 netutil 示例路由。
func RegisterNetutilRoutes(h *server.Hertz) {
	h.GET("/common/netutil", netutilHandler)
}

func netutilHandler(_ context.Context, c *app.RequestContext) {
	internalIP, err := netutil.GetInternalIP()
	isAvailable := netutil.IsNetworkAvailable()

	result := map[string]any{
		"internal_ip":      internalIP,
		"ip_error":         errToString(err),
		"network_available": isAvailable,
	}

	hertzresp.Success(c, result)
}

// errToString 将 error 转为字符串，nil 返回空串。
func errToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
