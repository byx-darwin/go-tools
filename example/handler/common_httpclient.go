package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/byx-darwin/go-tools/go-common/httpclient"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/valyala/fasthttp"
)

// RegisterHTTPClientRoutes 注册 httpclient 示例路由。
func RegisterHTTPClientRoutes(h *server.Hertz) {
	h.GET("/common/httpclient", httpclientHandler)
}

func httpclientHandler(_ context.Context, c *app.RequestContext) {
	// 单次请求。
	headers := map[string]string{"Accept": "application/json"}
	resp, statusCode, err := httpclient.Send(
		"https://httpbin.org/get", "GET", nil, headers, 5*time.Second,
	)

	result := map[string]any{}

	if err != nil {
		result["send_error"] = err.Error()
	} else {
		// 使用完后释放 fasthttp 响应到对象池。
		if resp != nil {
			defer fasthttp.ReleaseResponse(resp)
		}
		bodyLen := len(resp.Body())
		result["send"] = map[string]any{
			"status_code": statusCode,
			"body_length": bodyLen,
		}
	}

	// SendWithRetry 演示（不实际发送，仅展示 API）。
	result["send_with_retry_api"] = fmt.Sprintf(
		"httpclient.SendWithRetry(url, method, body, headers, sleep=%v, timeout=%v, retry=%d)",
		time.Second, 5*time.Second, 3,
	)

	hertzresp.Success(c, result)
}
