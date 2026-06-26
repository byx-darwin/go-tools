// Package compat 提供 go-framework 到 kitex SDK 的类型转换。
//
// 用法：
//
//	import (
//	    "github.com/cloudwego/kitex/pkg/endpoint"
//	    fwmw "github.com/byx-darwin/go-tools/go-framework/kitex/middleware/compat"
//	)
//	svr := xxxsvr.NewServer(handler,
//	    server.WithMiddleware(fwmw.AccessLog()),
//	)
package compat

import (
	"context"

	"github.com/byx-darwin/go-tools/go-framework/kitex/middleware"
	"github.com/cloudwego/kitex/pkg/endpoint"
)

// AccessLog 返回 kitex endpoint.Middleware 类型的 AccessLog。
func AccessLog() endpoint.Middleware {
	mw := middleware.AccessLog()
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return endpoint.Endpoint(mw(func(ctx context.Context, req, resp interface{}) error {
			return next(ctx, req, resp)
		}))
	}
}
