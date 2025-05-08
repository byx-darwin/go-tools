package hertz

import (
	"context"
	"encoding/hex"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	oteltrace "go.opentelemetry.io/otel/trace"
	"strconv"
)

type Response struct {
	Code    int         `json:"code"`
	Msg     string      `json:"msg"`
	Data    interface{} `json:"data"`
	TraceID string      `json:"trace_id"`
}

const (
	RELOGIN = 302
	ERROR   = 500
	SUCCESS = 200
)

func Result(c context.Context,
	ctx *app.RequestContext,
	code int,
	data interface{}, msg string) {
	traceID := oteltrace.SpanContextFromContext(c).TraceID().String()
	if data == nil {
		ctx.JSON(consts.StatusOK, utils.H{
			"code":     code,
			"msg":      msg,
			"trace_id": traceID,
		})
	} else {
		ctx.JSON(consts.StatusOK, Response{
			code,
			msg,
			data,
			traceID,
		})
	}
	ctx.Abort()
}

func Ok(c context.Context,
	ctx *app.RequestContext) {
	Result(c, ctx, SUCCESS, nil, "操作成功")
}

func OkWithMessage(c context.Context,
	ctx *app.RequestContext, msg string) {
	Result(c, ctx, SUCCESS, nil, msg)
}
func OkWithData(c context.Context,
	ctx *app.RequestContext, data interface{}) {
	Result(c, ctx, SUCCESS, data, "操作成功")
}

func OkWithDetailed(c context.Context,
	ctx *app.RequestContext, data interface{}, message string) {
	Result(c, ctx, SUCCESS, data, message)
}

func Fail(c context.Context,
	ctx *app.RequestContext) {
	Result(c, ctx, ERROR, nil, "操作失败")
}

func FailWithMessage(c context.Context,
	ctx *app.RequestContext, message string) {
	Result(c, ctx, ERROR, nil, message)
}

func ReLoginWithMessage(c context.Context,
	ctx *app.RequestContext, message string) {
	Result(c, ctx, RELOGIN, nil, message)
}

func OkWithBody(data []byte, ctx *app.RequestContext, bodyLength int) {
	dst := make([]byte, hex.EncodedLen(len(data)))
	hex.Encode(dst, data)
	ctx.Header("X-Body-Length", strconv.Itoa(bodyLength))
	ctx.Data(consts.StatusOK, "application/json; charset=UTF-8", dst)
}
