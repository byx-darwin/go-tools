package handler

import (
	"context"

	"github.com/byx-darwin/go-tools/go-common/templateutil"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterTemplateRoutes 注册 templateutil 示例路由。
func RegisterTemplateRoutes(h *server.Hertz) {
	h.GET("/common/template", templateHandler)
}

func templateHandler(ctx context.Context, c *app.RequestContext) {
	data := map[string]any{
		"Name":  "user_profile",
		"Table": "User",
	}

	// 使用默认函数集渲染。
	rendered, err := templateutil.Render("Hello {{.Name}}! Table: {{ToLower .Table}}, Snake: {{ToSnake .Table}}", data)
	if err != nil {
		hertzresp.Error(ctx, c, err, "template render failed")
		return
	}

	// Plural / Singular 演示。
	plural := templateutil.Plural("child")
	singular := templateutil.Singular("cities")

	// 自定义 Registry 演示。
	reg := templateutil.NewRegistry()
	reg.Register("shout", func(s string) string { return s + "!!!" })
	customOut, _ := templateutil.RenderWith("{{shout .Name}}", data, reg)

	results := map[string]any{
		"rendered":      rendered,
		"plural":        plural,
		"singular":      singular,
		"custom_render": customOut,
	}

	hertzresp.Success(c, results)
}
