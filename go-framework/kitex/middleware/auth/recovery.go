package auth

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/cloudwego/kitex/pkg/endpoint"

	"github.com/byx-darwin/go-tools/go-common/log"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
)

// Recovery 返回 Kitex Server 端 panic 恢复中间件。
// 捕获 handler 链路中的 panic，结构化记录日志（含 stack），转换为
// frameworkerror.ErrSystem 并通过 bizError 包装返回，避免进程崩溃。
// 仅提供 Server 端实现：Client 端是 RPC 发起方，panic 通常发生在业务
// handler 内部（Server 侧），Client 端无需 recovery。
//
// 使用方式：
//
//	server.WithMiddleware(auth.Recovery())
func Recovery() endpoint.Middleware {
	rpcLog := log.L().WithCategory(log.CategoryRPC)

	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) (err error) {
			defer func() {
				if r := recover(); r != nil {
					stack := debug.Stack()
					rpcLog.ErrorContext(ctx, "rpc panic recovered", fmt.Errorf("%v", r),
						"panic", fmt.Sprintf("%v", r),
						"stack", string(stack),
					)
					err = bizError(frameworkerror.ErrSystem.Errorf("rpc panic recovered: %v", r))
				}
			}()

			return next(ctx, req, resp)
		}
	}
}
