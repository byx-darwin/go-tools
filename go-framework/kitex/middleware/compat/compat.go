// Package compat 提供 go-framework 到 kitex SDK 的类型转换。
//
// 用法：
//
//	import (
//	    "github.com/cloudwego/kitex/pkg/endpoint"
//	    fwmw "gitee.com/byx_darwin/go-tools/go-framework/kitex/middleware/compat"
//	)
//	svr := xxxsvr.NewServer(handler,
//	    server.WithMiddleware(fwmw.AccessLog(logger)),
//	)
package compat

import (
	"context"

	"gitee.com/byx_darwin/go-tools/go-common/log"
	"gitee.com/byx_darwin/go-tools/go-framework/kitex/middleware"
	"github.com/cloudwego/kitex/pkg/endpoint"
)

// AccessLog 返回 kitex endpoint.Middleware 类型的 AccessLog。
func AccessLog(logger *log.Logger) endpoint.Middleware {
	mw := middleware.AccessLog(logger)
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return endpoint.Endpoint(mw(func(ctx context.Context, req, resp interface{}) error {
			return next(ctx, req, resp)
		}))
	}
}
