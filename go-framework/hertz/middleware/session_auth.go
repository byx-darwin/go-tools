package middleware

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/byx-darwin/go-tools/go-auth/session"
)

const (
	// headerSessionID Session ID 请求头名称。
	headerSessionID = "X-Session-Id"
	// cookieSessionID Session ID Cookie 名称。
	cookieSessionID = "session_id"
)

// SessionAuth 返回 Session 认证中间件。
// 优先从 X-Session-Id 头解析 Session ID，其次从 session_id Cookie 解析。
// 验证通过后将 Session 注入 RequestContext。
//
// 使用方式：
//
//	engine.Use(middleware.SessionAuth(sessionStore))
//	s, ok := middleware.GetSession(c)
func SessionAuth(store session.Store) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		sessionID := extractSessionID(c)
		if sessionID == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		s, err := store.Get(ctx, sessionID)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if s == nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		SetSession(c, s)
		c.Next(ctx)
	}
}

// extractSessionID 从请求中提取 Session ID。
// 优先级：X-Session-Id 头 > session_id Cookie。
func extractSessionID(c *app.RequestContext) string {
	// 优先从 Header 获取。
	if id := string(c.Request.Header.Peek(headerSessionID)); id != "" {
		return id
	}
	// 其次从 Cookie 获取。
	if id := string(c.Cookie(cookieSessionID)); id != "" {
		return id
	}
	return ""
}
