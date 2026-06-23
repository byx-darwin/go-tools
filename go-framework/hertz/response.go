package hertz

import (
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
)

// Response 统一 JSON 响应体
type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

// Result 写入统一 JSON 响应
func Result(c *app.RequestContext, httpCode int, code int, data any, msg string) {
	if data == nil {
		c.JSON(httpCode, utils.H{"code": code, "msg": msg})
	} else {
		c.JSON(httpCode, Response{code, msg, data})
	}
}

// OK 成功响应（200 OK）
func OK(c *app.RequestContext, data any) {
	Result(c, 200, 0, data, "ok")
}

// Err 错误响应（500 Internal Server Error）
func Err(c *app.RequestContext, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	Result(c, 500, 500, nil, msg)
}

// ErrWithCode 指定 HTTP 状态码的错误响应
func ErrWithCode(c *app.RequestContext, httpCode, bizCode int, msg string) {
	Result(c, httpCode, bizCode, nil, msg)
}
