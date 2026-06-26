package handler

import (
	"context"

	"github.com/byx-darwin/go-tools/go-common/astutil"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterAstutilRoutes 注册 astutil 示例路由。
func RegisterAstutilRoutes(h *server.Hertz) {
	h.GET("/common/astutil", astutilHandler)
}

func astutilHandler(ctx context.Context, c *app.RequestContext) {
	src := []byte(`package example

import "fmt"

func Hello() {
	fmt.Println("hello")
}

func World() {
	fmt.Println("world")
}
`)

	file, err := astutil.ParseSource(src)
	if err != nil {
		hertzresp.Error(ctx, c, err, "parse source failed")
		return
	}

	// 查找所有函数。
	funcs := file.FindFunctions()
	funcNames := make([]string, 0, len(funcs))
	for _, fn := range funcs {
		funcNames = append(funcNames, fn.Name.Name)
	}

	// 查找所有 import。
	imports := file.FindImports()
	importPaths := make([]string, 0, len(imports))
	for _, imp := range imports {
		importPaths = append(importPaths, imp.Path.Value)
	}

	// 按名称查找特定函数。
	helloFn := file.FindFunction("Hello")
	helloExists := helloFn != nil

	// 格式化输出（展示 Format 方法）。
	formatted, _ := file.Format()

	results := map[string]any{
		"functions":   funcNames,
		"imports":     importPaths,
		"hello_found": helloExists,
		"formatted_lines": len(formatted),
	}

	hertzresp.Success(c, results)
}
