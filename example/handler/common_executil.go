package handler

import (
	"context"
	"time"

	"github.com/byx-darwin/go-tools/go-common/executil"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterExecutilRoutes 注册 executil 示例路由。
func RegisterExecutilRoutes(h *server.Hertz) {
	h.GET("/common/executil", executilHandler)
}

func executilHandler(ctx context.Context, c *app.RequestContext) {
	runner := executil.New()

	// 执行 echo 命令。
	res := runner.Run(ctx, &executil.Cmd{
		Name:    "echo",
		Args:    []string{"hello from executil"},
		Timeout: 5 * time.Second,
	})

	result := map[string]any{
		"stdout":    string(res.Stdout),
		"stderr":    string(res.Stderr),
		"exit_code": res.ExitCode,
		"error":     errToString(res.Err),
	}

	// 演示 date 命令。
	dateRes := runner.Run(ctx, &executil.Cmd{
		Name:    "date",
		Args:    []string{"+%Y-%m-%d"},
		Timeout: 5 * time.Second,
	})
	result["date_stdout"] = string(dateRes.Stdout)

	hertzresp.Success(c, result)
}
