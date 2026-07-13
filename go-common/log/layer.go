package log

import "context"

// App 返回应用层 Logger（自动注入 category="app"）。
//
// ctx 参数保留用于未来扩展，当前实现直接返回带 category 的 Logger。
// 调用方使用 InfoContext(ctx, msg, args...) 传入 ctx 以自动关联 trace_id。
func App(_ context.Context) *Logger {
	return L().WithCategory(CategoryApp)
}

// DB 返回基础设施层 Logger（自动注入 category="db"）。
func DB(_ context.Context) *Logger {
	return L().WithCategory(CategoryDB)
}

// Access 返回展示层 Logger（自动注入 category="access"）。
func Access(_ context.Context) *Logger {
	return L().WithCategory(CategoryAccess)
}

// RPC 返回 RPC 层 Logger（自动注入 category="rpc"）。
func RPC(_ context.Context) *Logger {
	return L().WithCategory(CategoryRPC)
}

// MQ 返回消息队列层 Logger（自动注入 category="mq"）。
func MQ(_ context.Context) *Logger {
	return L().WithCategory(CategoryMQ)
}

// Cache 返回缓存层 Logger（自动注入 category="cache"）。
func Cache(_ context.Context) *Logger {
	return L().WithCategory(CategoryCache)
}
