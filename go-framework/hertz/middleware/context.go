// context.go 提供认证中间件的 context 注入/提取辅助函数。
//
// 使用 typed context key 避免与其他包的 key 冲突。
// Claims 和 Session 通过 c.Set/c.Get 在 Hertz RequestContext 中传递。
package middleware

import (
	"github.com/cloudwego/hertz/pkg/app"
)

// authCtxKey 认证上下文 key 类型。
type authCtxKey string

const (
	ctxKeyClaims  authCtxKey = "auth:claims"  // JWT Claims key
	ctxKeySession authCtxKey = "auth:session" // Session key
)

// SetClaims 将 JWT claims 注入 RequestContext。
// T 必须嵌入 jwt.RegisteredClaims。
func SetClaims[T any](c *app.RequestContext, claims *T) {
	c.Set(string(ctxKeyClaims), claims)
}

// GetClaims 从 RequestContext 提取 JWT claims。
// 返回 claims 指针和是否存在的布尔值。
func GetClaims[T any](c *app.RequestContext) (*T, bool) {
	v, ok := c.Get(string(ctxKeyClaims))
	if !ok {
		return nil, false
	}
	claims, ok := v.(*T)
	return claims, ok
}

// SetSession 将 Session 注入 RequestContext。
func SetSession(c *app.RequestContext, s any) {
	c.Set(string(ctxKeySession), s)
}

// GetSession 从 RequestContext 提取 Session。
// 返回 session 值和是否存在的布尔值。
func GetSession(c *app.RequestContext) (any, bool) {
	return c.Get(string(ctxKeySession))
}
