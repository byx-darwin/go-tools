package handler

import (
	"context"
	"fmt"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
)

// RegisterErrorRoutes 注册 error 示例路由。
func RegisterErrorRoutes(h *server.Hertz) {
	h.GET("/common/error", errorHandler)
}

func errorHandler(_ context.Context, c *app.RequestContext) {
	// 使用预定义错误。
	err1 := goerror.ErrParamInvalid.Wrap(fmt.Errorf("username is required"))
	code1, public1 := goerror.Extract(err1)

	// 自定义错误码。
	err2 := goerror.Code(40001).Public("data_duplicate").Wrap(fmt.Errorf("duplicate key"))
	code2, public2 := goerror.Extract(err2)

	// HTTP 状态码映射。
	httpStatus1 := goerror.HTTPStatus(err1)
	httpStatus2 := goerror.HTTPStatus(err2)

	// 分类判断。
	results := map[string]any{
		"predefined": map[string]any{
			"code":       code1,
			"public_msg": public1,
			"http_status": httpStatus1,
			"is_client":  goerror.IsClientError(code1),
		},
		"custom": map[string]any{
			"code":       code2,
			"public_msg": public2,
			"http_status": httpStatus2,
			"is_business": goerror.IsBusinessErrorCode(code2),
		},
		"error_ranges": map[string]any{
			"framework":  "10000-10499",
			"middleware": "20000-20699",
			"project":    "40000-59999",
		},
	}

	hertzresp.Success(c, results)
}
