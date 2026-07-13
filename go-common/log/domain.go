package log

import (
	"log/slog"
)

// DomainLogger 领域层日志端口接口。
//
// 领域层通过此接口记录业务日志，不依赖具体日志框架。
// 由应用层提供实现（domainLoggerAdapter）。
type DomainLogger interface {
	// Decision 记录业务决策。
	// accepted=true 输出 Info 级别，accepted=false 输出 Warn 级别。
	// 输出字段：log_type="decision", accepted=bool。
	Decision(msg string, accepted bool, args ...any)

	// Event 记录领域事件，Info 级别。
	// 输出字段：log_type="event"。
	Event(msg string, args ...any)

	// Error 记录业务异常，Error 级别。
	// 自动提取 oops 错误属性（error.code, error.domain 等）。
	Error(msg string, err error, args ...any)
}

// domainLoggerAdapter 实现 DomainLogger 接口，桥接到 log.Logger。
type domainLoggerAdapter struct {
	logger *Logger
	domain string
}

// NewDomainLogger 创建领域日志适配器。
//
// 注入 "domain" 字段到所有日志记录。
// domain 应为有意义的领域名称，如 "order"、"payment"。
//
// 用法：
//
//	svc := domain.NewOrderService(log.NewDomainLogger("order"))
func NewDomainLogger(domain string) DomainLogger {
	return &domainLoggerAdapter{
		logger: L(),
		domain: domain,
	}
}

// Decision 记录业务决策。
func (a *domainLoggerAdapter) Decision(msg string, accepted bool, args ...any) {
	logType := "decision"
	handler := NewDomainHandler(a.logger.Handler(), a.domain, logType)
	logger := slog.New(handler)

	allArgs := append([]any{"accepted", accepted}, args...)
	if accepted {
		logger.Info(msg, allArgs...)
	} else {
		logger.Warn(msg, allArgs...)
	}
}

// Event 记录领域事件。
func (a *domainLoggerAdapter) Event(msg string, args ...any) {
	logType := "event"
	handler := NewDomainHandler(a.logger.Handler(), a.domain, logType)
	logger := slog.New(handler)
	logger.Info(msg, args...)
}

// Error 记录业务异常。
func (a *domainLoggerAdapter) Error(msg string, err error, args ...any) {
	logType := "error"
	handler := NewDomainHandler(a.logger.Handler(), a.domain, logType)
	logger := slog.New(handler)

	allArgs := args
	if err != nil {
		allArgs = append(allArgs, "error", err.Error())
		allArgs = append(allArgs, ErrorAttrs(err)...)
	}
	logger.Error(msg, allArgs...)
}
