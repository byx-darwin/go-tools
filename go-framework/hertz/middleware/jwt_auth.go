package middleware

import (
	"context"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
	gojwt "github.com/byx-darwin/go-tools/go-auth/jwt"
	"github.com/byx-darwin/go-tools/go-auth/revocation"
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// jwtAuthConfig 存储 JWTAuth 配置选项。
type jwtAuthConfig struct {
	revocationChecker revocation.Checker
}

// JWTAuthOption 定义 JWTAuth 配置选项函数。
type JWTAuthOption func(*jwtAuthConfig)

// WithRevocationChecker 设置撤销检查器。
// 验证签名成功后，若配置了 checker，会额外查询撤销表；命中则返回 ErrTokenRevoked。
// 未设置时行为与不启用撤销检查完全一致。
//
// 安全告诫（fail-open）：撤销检查依赖 Claims 中的 jti（JWT ID）。若 Claims
// 未嵌入 jti（gojwt.ExtractJTI 返回 ok=false），本中间件会跳过撤销检查、
// 放行请求——即没有 jti 的 token 永远无法被撤销。业务方必须确保签发 token
// 时设置了 jti（RegisteredClaims.ID），否则该 token 的撤销能力形同虚设。
func WithRevocationChecker(checker revocation.Checker) JWTAuthOption {
	return func(c *jwtAuthConfig) { c.revocationChecker = checker }
}

// applyJWTAuthOptions 应用选项并返回配置快照。
func applyJWTAuthOptions(opts []JWTAuthOption) jwtAuthConfig {
	var cfg jwtAuthConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// JWTAuth 返回 JWT 认证中间件。
// 从 Authorization Bearer 头解析 token，验证签名，将 claims 注入 RequestContext。
// T 必须嵌入 jwt.RegisteredClaims。可通过 WithRevocationChecker 启用撤销检查。
//
// 使用方式：
//
//	engine.Use(middleware.JWTAuth[UserClaims](secret))
//	claims, ok := middleware.GetClaims[UserClaims](c)
func JWTAuth[T any](secret []byte, opts ...JWTAuthOption) app.HandlerFunc {
	cfg := applyJWTAuthOptions(opts)

	return func(ctx context.Context, c *app.RequestContext) {
		token := extractBearerToken(c)
		if token == "" {
			writeAuthError(c, autherror.ErrTokenInvalid.Errorf("missing bearer token"))
			return
		}

		claims, err := gojwt.Verify[T](token, secret)
		if err != nil {
			writeAuthError(c, err)
			return
		}

		if cfg.revocationChecker != nil {
			if jti, ok := gojwt.ExtractJTI(claims); ok {
				revoked, rerr := cfg.revocationChecker.IsRevoked(ctx, jti)
				if rerr != nil {
					writeAuthError(c, autherror.ErrJWTVerifyFailed.Wrap(rerr))
					return
				}
				if revoked {
					writeAuthError(c, autherror.ErrTokenRevoked.Errorf("token revoked"))
					return
				}
			}
		}

		SetClaims(c, claims)
		c.Next(ctx)
	}
}

// writeAuthError 中断请求并记录错误。不依赖 go-framework/hertz 包——
// hertz 包（server.go）已经 import 了 hertz/middleware（挂载 middleware.Cors()），
// 若 middleware 反过来 import hertz 会形成循环 import，编译失败。
// AbortWithError 是 Hertz 框架自带方法，立即写出正确的 HTTP 状态码
// （goerror.HTTPStatus(err)）并把 err 压入 c.Errors；未配置
// hertz.Responder.Middleware() 时 body 为空，状态码已经正确（与当前
// AbortWithStatus 行为等价，无回归）；配置了的话，Responder.Middleware()
// 会在链路收尾时用完整的内容协商重写响应体。
//
// 防御性兜底：本包内所有调用方传入的 err 都是已注册 HTTP 状态码的认证错误
// （401/403/500），goerror.HTTPStatus 不会走到"无错误码 → 200"的默认分支；
// 但若未来某处不慎传入未注册错误码的错误，这里仍强制回落到 401，避免鉴权
// 失败被误报为成功。
func writeAuthError(c *app.RequestContext, err error) {
	status := goerror.HTTPStatus(err)
	if status < 400 {
		status = 401
	}
	_ = c.AbortWithError(status, err)
}

// extractBearerToken 从 Authorization 头提取 Bearer token。
func extractBearerToken(c *app.RequestContext) string {
	auth := string(c.Request.Header.Peek("Authorization"))
	if auth == "" {
		return ""
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return ""
	}

	token := strings.TrimSpace(auth[len(prefix):])
	if token == "" {
		return ""
	}

	return token
}
