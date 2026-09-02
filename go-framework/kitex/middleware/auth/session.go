package auth

import (
	"context"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/kitex/pkg/endpoint"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
	"github.com/byx-darwin/go-tools/go-auth/session"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
)

// SessionAuthServer 返回 Kitex Server 端 Session 鉴权中间件。
// 从 incoming metainfo（persistent key）读取 Session ID，校验通过后将
// Session 与 Session ID 注入 ctx，供业务代码与 SessionAuthClient
// （透传到下游）使用。
//
// 使用方式：
//
//	server.WithMiddleware(auth.SessionAuthServer(sessionStore))
//	s, ok := auth.GetSession(ctx)
func SessionAuthServer(store session.Store) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			sessionID, ok := metainfo.GetPersistentValue(ctx, metaKeySessionID)
			if !ok || sessionID == "" {
				return bizError(autherror.ErrSessionInvalid.Errorf("missing session id in metainfo"))
			}

			s, err := store.Get(ctx, sessionID)
			if err != nil {
				return bizError(frameworkerror.ErrSystem.Wrap(err))
			}
			if s == nil {
				return bizError(autherror.ErrSessionInvalid.Errorf("session not found"))
			}

			ctx = SetSession(ctx, s)
			ctx = SetSessionID(ctx, sessionID)
			return next(ctx, req, resp)
		}
	}
}

// SessionAuthClient 返回 Kitex Client 端 Session 身份注入中间件。
// 优先复用 ctx 中已由 SessionAuthServer 校验通过的 Session ID；ctx 中
// 没有时调用 sessionIDProvider 获取。Session ID 通过
// metainfo.WithPersistentValue 写入，随调用链自动持久透传到所有下游。
//
// sessionIDProvider 允许为 nil；ctx 与 provider 均未提供时返回错误。
func SessionAuthClient(sessionIDProvider func(ctx context.Context) (string, bool)) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			sessionID, ok := GetSessionID(ctx)
			if !ok || sessionID == "" {
				if sessionIDProvider == nil {
					return bizError(autherror.ErrSessionInvalid.Errorf("no session id in context and no sessionIDProvider configured"))
				}
				sessionID, ok = sessionIDProvider(ctx)
				if !ok || sessionID == "" {
					return bizError(autherror.ErrSessionInvalid.Errorf("sessionIDProvider returned no session id"))
				}
			}

			ctx = metainfo.WithPersistentValue(ctx, metaKeySessionID, sessionID)
			return next(ctx, req, resp)
		}
	}
}
