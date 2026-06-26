package handler

import (
	"context"
	"strconv"

	"github.com/byx-darwin/go-tools/go-auth/device"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/google/uuid"
)

// deviceStore Device 存储实例（由 main 通过 SetDeviceStore 注入）。
var deviceStore device.Store

// SetDeviceStore 注入 Device 存储（在 main 中调用）。
func SetDeviceStore(s device.Store) {
	deviceStore = s
}

// RegisterDeviceRoutes 注册 device 示例路由。
func RegisterDeviceRoutes(h *server.Hertz) {
	h.POST("/auth/device", deviceRegisterHandler)
	h.GET("/auth/device", deviceListHandler)
	h.DELETE("/auth/device", deviceRemoveHandler)
}

// deviceRegisterHandler 注册新设备并返回被踢出的设备列表。
//
// 请求体：{"user_id": "xxx", "device_id": "yyy"}（device_id 可选，缺省时自动生成）
// 查询参数：max_devices=N（默认 5）
// 响应：device_id、jti、kicked_devices。
func deviceRegisterHandler(ctx context.Context, c *app.RequestContext) {
	var req struct {
		UserID   string `json:"user_id" vd:"len($)>0"`
		DeviceID string `json:"device_id"`
	}
	if err := c.BindAndValidate(&req); err != nil {
		hertzresp.Error(ctx, c, err, "invalid request")
		return
	}

	// 自动生成 device_id（若未提供）。
	deviceID := req.DeviceID
	if deviceID == "" {
		deviceID = uuid.NewString()
	}

	// 生成 JTI（JWT ID，用于关联 Token）。
	jti := uuid.NewString()

	// 从查询参数获取 max_devices（默认 5）。
	maxDevices := 5
	if v := c.QueryArgs().Peek("max_devices"); len(v) > 0 {
		if n, err := strconv.Atoi(string(v)); err == nil && n > 0 {
			maxDevices = n
		}
	}

	kicked, err := deviceStore.AddDevice(ctx, req.UserID, deviceID, jti, maxDevices)
	if err != nil {
		hertzresp.Error(ctx, c, err, "register device failed")
		return
	}

	hertzresp.Success(c, map[string]any{
		"device_id":      deviceID,
		"jti":            jti,
		"kicked_devices": kicked,
	})
}

// deviceListHandler 列出用户的所有已注册设备。
//
// 查询参数：user_id=xxx
// 响应：devices 列表。
func deviceListHandler(ctx context.Context, c *app.RequestContext) {
	userID := string(c.QueryArgs().Peek("user_id"))
	if userID == "" {
		hertzresp.ErrorWithCode(ctx, c, 400, 10001, "missing user_id query param")
		return
	}

	devices, err := deviceStore.ListDevices(ctx, userID)
	if err != nil {
		hertzresp.Error(ctx, c, err, "list devices failed")
		return
	}

	hertzresp.Success(c, map[string]any{
		"devices": devices,
	})
}

// deviceRemoveHandler 移除指定设备。
//
// 查询参数：user_id=xxx&device_id=yyy
// 响应：删除结果。
func deviceRemoveHandler(ctx context.Context, c *app.RequestContext) {
	userID := string(c.QueryArgs().Peek("user_id"))
	deviceID := string(c.QueryArgs().Peek("device_id"))

	if userID == "" || deviceID == "" {
		hertzresp.ErrorWithCode(ctx, c, 400, 10001, "missing user_id or device_id query param")
		return
	}

	if err := deviceStore.RemoveDevice(ctx, userID, deviceID); err != nil {
		hertzresp.Error(ctx, c, err, "remove device failed")
		return
	}

	hertzresp.Success(c, map[string]any{
		"deleted": true,
	})
}
