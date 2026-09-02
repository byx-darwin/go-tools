// Package auth 提供 Kitex RPC 鉴权中间件。
//
// # 快速开始
//
// JWT 鉴权（Server 端校验 + Client 端注入）：
//
//	// 服务端（被调用方）
//	svr := xxxsvr.NewServer(handler,
//	    server.WithMiddleware(auth.JWTAuthServer[UserClaims](secret)),
//	)
//	claims, ok := auth.GetClaims[UserClaims](ctx)
//
//	// 客户端（调用方）
//	cli, _ := xxxcli.NewClient("target-service",
//	    client.WithMiddleware(auth.JWTAuthClient[UserClaims](func(ctx context.Context) (string, bool) {
//	        return getTokenFromSomewhere(ctx)
//	    })),
//	)
//
// Session / Device 鉴权用法对称，分别见 SessionAuthServer/SessionAuthClient
// 与 DeviceAuthServer/DeviceAuthClient。
//
// Panic 恢复：
//
//	server.WithMiddleware(auth.Recovery())
//
// # 身份透传
//
// 三种鉴权中间件均使用 github.com/bytedance/gopkg/cloud/metainfo 的
// WithPersistentValue/GetPersistentValue 在 RPC 调用链路中传递身份。
// Persistent 语义保证 A→B→C 调用链中，B 收到 A 传来的身份后，B 调 C 时
// 该身份自动继续透传，无需业务代码在 B 中手动转发。
//
// # 错误处理
//
// 鉴权失败统一通过 go-framework/kitex/rpcerror.OopsStatusAdapter 包装为
// Kitex BizStatusErrorIface 返回，错误码复用 go-auth/error（Token/Session/
// Device 相关）与 go-framework/error（CodeTokenMissing 等）。
package auth
