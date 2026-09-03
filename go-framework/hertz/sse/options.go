package sse

import "time"

// defaultHeartbeatInterval 默认心跳保活间隔。
const defaultHeartbeatInterval = 15 * time.Second

// config Writer 内部配置，由 Option 填充。
type config struct {
	heartbeatInterval time.Duration
	onRecover         func(rec any)
}

// defaultConfig 返回默认配置：heartbeatInterval=15s，onRecover=nil（仅记录日志）。
func defaultConfig() config {
	return config{heartbeatInterval: defaultHeartbeatInterval}
}

// Option 定义 sse.Writer 配置选项。
type Option func(*config)

// WithHeartbeatInterval 设置心跳保活间隔。<=0 禁用心跳 goroutine。默认 15s。
func WithHeartbeatInterval(d time.Duration) Option {
	return func(c *config) { c.heartbeatInterval = d }
}

// WithRecoverHandler 设置自定义 panic 上报回调（如埋点/告警），
// 在写入 event:error 之前调用。传入 nil 保持已有配置不变，默认仅记录结构化日志。
func WithRecoverHandler(fn func(rec any)) Option {
	return func(c *config) {
		if fn != nil {
			c.onRecover = fn
		}
	}
}
