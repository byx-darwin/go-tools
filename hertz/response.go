package hertz

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"

	"gitee.com/byx_darwin/go-tools/kitex/rpc_error"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

const (
	RELOGIN = 302
	ERROR   = 500
	SUCCESS = 200
)

func Result(c context.Context,
	ctx *app.RequestContext,
	httpCode int,
	code int,
	data interface{}, msg string) {
	traceID := oteltrace.SpanContextFromContext(c).TraceID().String()
	ctx.Header("X-Trace-ID", traceID)
	if data == nil {
		ctx.JSON(httpCode, utils.H{
			"code": code,
			"msg":  msg,
		})
	} else {
		ctx.JSON(httpCode, Response{
			code,
			msg,
			data,
		})
	}
	ctx.Abort()
}

func ReplyWithBindErr(ctx context.Context, c *app.RequestContext, err error) {
	Result(ctx, c, consts.StatusForbidden, ERROR, nil, err.Error())
}

func ReplyWithReLogin(ctx context.Context, c *app.RequestContext) {
	Result(ctx, c, consts.StatusOK, RELOGIN, nil, "重新登录")
}

func ReplyWithServerError(ctx context.Context, c *app.RequestContext,
	format string, log hlog.CtxLogger, err error) {
	path := string(c.Path())
	log.CtxErrorf(ctx, fmt.Sprintf("(%s)%s", path, format), err.Error())
	Result(ctx, c, consts.StatusInternalServerError, ERROR, nil, "内部错误")
}

func ReplyWithErr(ctx context.Context, c *app.RequestContext,
	format string, log hlog.CtxLogger, err error) {
	if err != nil {
		path := string(c.Path())
		log.CtxErrorf(ctx, fmt.Sprintf("(%s)%s", path, format), err.Error())
		Result(ctx, c, consts.StatusInternalServerError, ERROR, nil, "内部错误")
		return
	}
	Result(ctx, c, consts.StatusOK, SUCCESS, nil, "ok")
}

func ReplyWithOk(ctx context.Context, c *app.RequestContext, publicMsg string, log hlog.CtxLogger, err error) {
	if err != nil {
		errType, privateErrMsg := rpc_error.ParseBizStatusError(err)
		if errType == rpc_error.ErrorTypeDataRepeat {
			Result(ctx, c, consts.StatusOK, ERROR, nil, publicMsg)
		} else {
			path := string(c.Path())
			log.CtxErrorf(ctx, fmt.Sprintf("(%s)%s", path, privateErrMsg), err.Error())
			Result(ctx, c, consts.StatusInternalServerError, ERROR, nil, "内部错误")
		}
		return
	}
	Result(ctx, c, consts.StatusOK, SUCCESS, nil, "ok")
}
func ReplyWithTeaBody(ctx context.Context, c *app.RequestContext, data []byte, bodyLength int) {
	dst := make([]byte, hex.EncodedLen(len(data)))
	hex.Encode(dst, data)
	c.Header("X-Body-Length", strconv.Itoa(bodyLength))
	traceID := oteltrace.SpanContextFromContext(ctx).TraceID().String()
	c.Header("X-Trace-ID", traceID)
	c.Data(consts.StatusOK, "text/plain", dst)

}
