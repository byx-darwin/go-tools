package middleware

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"

	"github.com/byx-darwin/go-tools/go-auth/session"
)

const (
	// headerSessionID Session ID 请求头默认名称。
	headerSessionID = "X-Session-Id"
	// cookieSessionID Session ID Cookie 默认名称。
	cookieSessionID = "session_id"
)

// sessionAuthConfig 存储 SessionAuth 配置选项。
type sessionAuthConfig struct {
	headerName string
	cookieName string
}

// SessionAuthOption 定义 SessionAuth 配置选项函数。
type SessionAuthOption func(*sessionAuthConfig)

// WithSessionHeader 设置 Session ID 请求头名称。空字符串忽略，保持默认 X-Session-Id。
func WithSessionHeader(name string) SessionAuthOption {
	return func(c *sessionAuthConfig) {
		if name != "" {
			c.headerName = name
		}
	}
}

// WithSessionCookie 设置 Session ID Cookie 名称。空字符串忽略，保持默认 session_id。
func WithSessionCookie(name string) SessionAuthOption {
	return func(c *sessionAuthConfig) {
		if name != "" {
			c.cookieName = name
		}
	}
}

// SessionAuth 返回 Session 认证中间件。
// 优先从 Header（默认 X-Session-Id，可用 WithSessionHeader 定制）解析 Session ID，
// 其次从 Cookie（默认 session_id，可用 WithSessionCookie 定制）解析。
// 验证通过后将 Session 注入 RequestContext。
//
// 使用方式：
//
//	engine.Use(middleware.SessionAuth(sessionStore))
//	s, ok := middleware.GetSession(c)
//
//	// 自定义 Header/Cookie 名称：
//	engine.Use(middleware.SessionAuth(sessionStore,
//	    middleware.WithSessionHeader("X-My-Session"),
//	    middleware.WithSessionCookie("my_sid")))
func SessionAuth(store session.Store, opts ...SessionAuthOption) app.HandlerFunc {
	cfg := sessionAuthConfig{headerName: headerSessionID, cookieName: cookieSessionID}
	for _, opt := range opts {
		opt(&cfg)
	}

	return func(ctx context.Context, c *app.RequestContext) {
		sessionID := extractSessionID(c, cfg.headerName, cfg.cookieName)
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
// 优先级：headerName 请求头 > cookieName Cookie。
func extractSessionID(c *app.RequestContext, headerName, cookieName string) string {
	// 优先从 Header 获取。
	if id := string(c.Request.Header.Peek(headerName)); id != "" {
		return id
	}
	// 其次从 Cookie 获取。
	if id := string(c.Cookie(cookieName)); id != "" {
		return id
	}
	return ""
}
