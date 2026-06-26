package handler

import (
	"context"
	"time"

	"github.com/byx-darwin/go-tools/go-auth/session"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/google/uuid"
)

// sessionStore Session 存储实例（由 main 通过 SetSessionStore 注入）。
var sessionStore session.Store

// SetSessionStore 注入 Session 存储（在 main 中调用）。
func SetSessionStore(s session.Store) {
	sessionStore = s
}

// RegisterSessionRoutes 注册 session 示例路由。
func RegisterSessionRoutes(h *server.Hertz) {
	h.POST("/auth/session", sessionCreateHandler)
	h.GET("/auth/session", sessionGetHandler)
	h.DELETE("/auth/session", sessionDeleteHandler)
}

// sessionCreateHandler 创建新 Session，返回 session_id。
//
// 请求体：{"user_id": "xxx", "data": {"key": "value"}}（可选）
// 响应：session_id、expires_at。
func sessionCreateHandler(ctx context.Context, c *app.RequestContext) {
	var req struct {
		UserID string         `json:"user_id" vd:"len($)>0"`
		Data   map[string]any `json:"data"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		hertzresp.Error(ctx, c, err, "invalid request")
		return
	}

	// 生成 session ID（UUID v4）。
	sessionID := uuid.NewString()

	// 默认 30 分钟过期。
	expiresAt := time.Now().Add(30 * time.Minute)

	sess := &session.Session{
		ID:        sessionID,
		UserUUID:  req.UserID,
		Data:      req.Data,
		ExpiresAt: expiresAt,
	}

	if err := sessionStore.Save(ctx, sess); err != nil {
		hertzresp.Error(ctx, c, err, "save session failed")
		return
	}

	hertzresp.Success(c, map[string]any{
		"session_id": sessionID,
		"expires_at": expiresAt,
	})
}

// sessionGetHandler 根据 X-Session-Id 查询 Session。
//
// 请求头：X-Session-Id: xxx
// 响应：session 信息（id、user_uuid、data、expires_at）。
func sessionGetHandler(ctx context.Context, c *app.RequestContext) {
	sessionID := string(c.Request.Header.Peek("X-Session-Id"))
	if sessionID == "" {
		hertzresp.ErrorWithCode(ctx, c, 400, 10001, "missing X-Session-Id header")
		return
	}

	sess, err := sessionStore.Get(ctx, sessionID)
	if err != nil {
		hertzresp.Error(ctx, c, err, "get session failed")
		return
	}
	if sess == nil {
		hertzresp.ErrorWithCode(ctx, c, 404, 40005, "session not found")
		return
	}

	hertzresp.Success(c, map[string]any{
		"session_id": sess.ID,
		"user_uuid":  sess.UserUUID,
		"data":       sess.Data,
		"expires_at": sess.ExpiresAt,
	})
}

// sessionDeleteHandler 删除指定 Session。
//
// 请求头：X-Session-Id: xxx
// 响应：删除结果。
func sessionDeleteHandler(ctx context.Context, c *app.RequestContext) {
	sessionID := string(c.Request.Header.Peek("X-Session-Id"))
	if sessionID == "" {
		hertzresp.ErrorWithCode(ctx, c, 400, 10001, "missing X-Session-Id header")
		return
	}

	// 先检查是否存在。
	exists, err := sessionStore.Exists(ctx, sessionID)
	if err != nil {
		hertzresp.Error(ctx, c, err, "check session failed")
		return
	}
	if !exists {
		hertzresp.ErrorWithCode(ctx, c, 404, 40005, "session not found")
		return
	}

	if err := sessionStore.Delete(ctx, sessionID); err != nil {
		hertzresp.Error(ctx, c, err, "delete session failed")
		return
	}

	hertzresp.Success(c, map[string]any{
		"deleted": true,
	})
}
