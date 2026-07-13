package log

// 预定义日志分类常量。
const (
	// CategoryAccess HTTP 访问日志。
	CategoryAccess = "access"

	// CategoryError 错误日志。
	CategoryError = "error"

	// CategoryBiz 业务逻辑日志。
	CategoryBiz = "biz"

	// CategoryRPC RPC 调用日志。
	CategoryRPC = "rpc"

	// CategoryDB 数据库操作日志。
	CategoryDB = "db"

	// CategoryPanic panic 恢复日志。
	CategoryPanic = "panic"

	// CategoryAudit 审计日志。
	CategoryAudit = "audit"

	// CategorySecurity 安全相关日志。
	CategorySecurity = "security"

	// CategoryApp 应用层日志。
	CategoryApp = "app"

	// CategoryCache 缓存操作日志。
	CategoryCache = "cache"

	// CategoryMQ 消息队列日志。
	CategoryMQ = "mq"
)
