package auth

import (
	"context"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/cloudwego/kitex/pkg/endpoint"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
	gojwt "github.com/byx-darwin/go-tools/go-auth/jwt"
	"github.com/byx-darwin/go-tools/go-auth/revocation"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
)

// jwtConfig 存储 JWTAuthServer 配置选项。
type jwtConfig struct {
	revocationChecker revocation.Checker
	verifyOptions     []gojwt.Option
}

// JWTOption 定义 JWTAuthServer 配置选项函数。
type JWTOption func(*jwtConfig)

// WithRevocationChecker 设置撤销检查器。验证签名成功后，若配置了
// checker，会额外查询撤销表；命中则返回 autherror.ErrTokenRevoked。
// 未设置时行为与不启用撤销检查完全一致。语义与
// go-framework/hertz/middleware.WithRevocationChecker 一致。
func WithRevocationChecker(checker revocation.Checker) JWTOption {
	return func(c *jwtConfig) { c.revocationChecker = checker }
}

// WithVerifyOptions 透传 go-auth/jwt 的 Verify 选项（如
// gojwt.WithExpectedIssuer）给内部 gojwt.Verify 调用。语义与
// go-framework/hertz/middleware.WithVerifyOptions 一致。
func WithVerifyOptions(opts ...gojwt.Option) JWTOption {
	return func(c *jwtConfig) { c.verifyOptions = append(c.verifyOptions, opts...) }
}

func applyJWTOptions(opts []JWTOption) jwtConfig {
	var cfg jwtConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// JWTAuthServer 返回 Kitex Server 端 JWT 鉴权中间件。
// 从 incoming metainfo（persistent key）读取 token，验证签名后将 claims
// 与原始 token 注入 ctx，供业务代码与 JWTAuthClient（透传到下游）使用。
// T 必须嵌入 jwt.RegisteredClaims。
//
// 使用方式：
//
//	server.WithMiddleware(auth.JWTAuthServer[UserClaims](secret))
//	claims, ok := auth.GetClaims[UserClaims](ctx)
func JWTAuthServer[T any](secret []byte, opts ...JWTOption) endpoint.Middleware {
	cfg := applyJWTOptions(opts)

	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			token, ok := metainfo.GetPersistentValue(ctx, metaKeyJWTToken)
			if !ok || token == "" {
				return bizError(frameworkerror.ErrTokenMissing.Errorf("missing jwt token in metainfo"))
			}

			claims, err := gojwt.Verify[T](token, secret, cfg.verifyOptions...)
			if err != nil {
				return bizError(err)
			}

			if cfg.revocationChecker != nil {
				if jti, ok := gojwt.ExtractJTI(claims); ok {
					revoked, rerr := cfg.revocationChecker.IsRevoked(ctx, jti)
					if rerr != nil {
						return bizError(autherror.ErrJWTVerifyFailed.Wrap(rerr))
					}
					if revoked {
						return bizError(autherror.ErrTokenRevoked.Errorf("token revoked"))
					}
				}
			}

			ctx = SetClaims(ctx, claims)
			ctx = SetJWTToken(ctx, token)
			return next(ctx, req, resp)
		}
	}
}

// JWTAuthClient 返回 Kitex Client 端 JWT 身份注入中间件。
// 优先复用 ctx 中已由 JWTAuthServer 校验通过的原始 token（多跳场景，
// 如 B 调 C 复用 A 传给 B 的身份，不重新签发）；ctx 中没有时调用
// tokenProvider 获取 token。token 通过 metainfo.WithPersistentValue
// 写入 outgoing metadata，随调用链自动持久透传到所有下游，无需业务代码
// 在中间服务手动转发。
//
// tokenProvider 允许为 nil（例如纯透传场景、ctx 中必然已有 token）；
// ctx 与 provider 均未提供 token 时返回错误。
//
// 使用方式：
//
//	client.WithMiddleware(auth.JWTAuthClient[UserClaims](func(ctx context.Context) (string, bool) {
//	    return getTokenFromSomewhere(ctx)
//	}))
func JWTAuthClient[T any](tokenProvider func(ctx context.Context) (string, bool)) endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			token, ok := GetJWTToken(ctx)
			if !ok || token == "" {
				if tokenProvider == nil {
					return bizError(frameworkerror.ErrTokenMissing.Errorf("no token in context and no tokenProvider configured"))
				}
				token, ok = tokenProvider(ctx)
				if !ok || token == "" {
					return bizError(frameworkerror.ErrTokenMissing.Errorf("tokenProvider returned no token"))
				}
			}

			ctx = metainfo.WithPersistentValue(ctx, metaKeyJWTToken, token)
			return next(ctx, req, resp)
		}
	}
}
