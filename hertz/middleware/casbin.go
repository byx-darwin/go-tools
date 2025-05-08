package middleware

import (
	"context"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"strconv"
)

type CasbinFace interface {
	Enforce(ctx context.Context,
		roleID int64, path string, method string) (bool, error)
	GetRoleID(ctx *app.RequestContext) int64
}

func CasbinHandler(casbinFace CasbinFace, log hlog.CtxLogger) app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		roleId := casbinFace.GetRoleID(ctx)
		path := string(ctx.Path())
		method := string(ctx.Method())
		ok, err := casbinFace.Enforce(c, roleId, path, method)
		if err != nil {
			log.CtxErrorf(c, "casbin enforce,err:%s", err.Error())
			ctx.AbortWithStatus(consts.StatusBadRequest)
			return
		}
		if !ok {
			hlog.CtxErrorf(c, "roleId:%s,path:%s,method:%s,no permission",
				strconv.Itoa(int(roleId)), path, method)
			ctx.AbortWithStatus(consts.StatusForbidden)
			return
		}
		ctx.Next(c)
	}
}
