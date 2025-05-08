package middleware

import (
	"context"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type LocationFace interface {
	RegisterClientIp(c context.Context,
		ctx *app.RequestContext) (context.Context, error)
}

func RegisterLocation(locationFace LocationFace) app.HandlerFunc {
	return func(context context.Context, ctx *app.RequestContext) {
		ctx2, err := locationFace.RegisterClientIp(context, ctx)
		if err != nil {
			hlog.CtxErrorf(context, "register Client ip failure,err:%s", err.Error())
			ctx.AbortWithStatus(consts.StatusBadRequest)
			return
		}
		ctx.Next(ctx2)
	}
}
