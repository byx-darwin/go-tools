// Package middleware 提供 Hertz HTTP 中间件和受保护路由的注册逻辑。
package middleware

import (
	"github.com/cloudwego/hertz/pkg/app/server"

	"github.com/byx-darwin/go-tools/example/handler"
	"github.com/byx-darwin/go-tools/go-auth/device"
	"github.com/byx-darwin/go-tools/go-auth/session"
	hertzmw "github.com/byx-darwin/go-tools/go-framework/hertz/middleware"
)

// Deps 中间件注册所需的运行时依赖。
type Deps struct {
	// SessionStore Session 存储（内存或 Redis 实现）。
	SessionStore session.Store

	// DeviceStore Device 存储（内存或 Redis 实现）。
	DeviceStore device.Store

	// JWTSecret JWT 签名密钥（字节切片）。
	JWTSecret []byte
}

// RegisterMiddleware 注册全局中间件（AccessLog、Cors）。
func RegisterMiddleware(h *server.Hertz, _ *Deps) {
	h.Use(hertzmw.AccessLog())
	h.Use(hertzmw.Cors())
}

// RegisterProtectedRoutes 注册受保护的路由组。
//
// 路由组：
//   - /protected/jwt     — JWT 认证保护
//   - /protected/session — Session 认证保护
//   - /protected/device  — JWT + Device 认证保护
func RegisterProtectedRoutes(h *server.Hertz, deps *Deps) {
	// JWT 保护路由组。
	jwtGroup := h.Group("/protected/jwt")
	jwtGroup.Use(hertzmw.JWTAuth[handler.AppClaims](deps.JWTSecret))
	jwtGroup.GET("", handler.HandleProtectedJWT)

	// Session 保护路由组。
	sessionGroup := h.Group("/protected/session")
	sessionGroup.Use(hertzmw.SessionAuth(deps.SessionStore))
	sessionGroup.GET("", handler.HandleProtectedSession)

	// Device 保护路由组（需配合 JWT 使用）。
	deviceGroup := h.Group("/protected/device")
	deviceGroup.Use(hertzmw.JWTAuth[handler.DeviceAppClaims](deps.JWTSecret))
	deviceGroup.Use(hertzmw.DeviceAuth(deps.DeviceStore, handler.ExtractDeviceClaims))
	deviceGroup.GET("", handler.HandleProtectedDevice)
}
