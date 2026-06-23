// Package middleware 提供 Kitex RPC 访问日志中间件。
//
// 注意：为避免 kitex SDK 的 genproto 冲突，本包不直接 import kitex。
// Middleware 函数签名与 kitex endpoint.Middleware 兼容：
//
//	type Middleware func(Endpoint) Endpoint
//	type Endpoint func(ctx context.Context, req, resp interface{}) error
//
// 导入 kitex 后可直接赋值：
//
//	var mw endpoint.Middleware = middleware.AccessLog(logger)
package middleware

import (
	"context"
	"time"

	"gitee.com/byx_darwin/go-tools/go-common/log"
)

// Endpoint Kitex RPC 端点函数（兼容 kitex endpoint.Endpoint）。
type Endpoint func(ctx context.Context, req, resp interface{}) error

// Middleware Kitex RPC 中间件（兼容 kitex endpoint.Middleware）。
type Middleware func(Endpoint) Endpoint

// AccessLog 返回 Kitex server-side AccessLog 中间件。
//
// 用法:
//
//	import "github.com/cloudwego/kitex/pkg/endpoint"
//	var mw endpoint.Middleware = endpoint.Middleware(middleware.AccessLog(logger))
func AccessLog(logger *log.Logger) Middleware {
	return func(next Endpoint) Endpoint {
		return func(ctx context.Context, req, resp interface{}) error {
			start := time.Now()
			err := next(ctx, req, resp)
			latency := time.Since(start)

			if err != nil {
				logger.ErrorContext(ctx, "rpc_access",
					"latency_ms", latency.Milliseconds(),
					"error", err.Error(),
				)
			} else {
				logger.InfoContext(ctx, "rpc_access",
					"latency_ms", latency.Milliseconds(),
				)
			}
			return err
		}
	}
}
