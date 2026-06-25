package log

import "sync"

var (
	defaultLogger *Logger
	defaultMu     sync.RWMutex
)

// Init 初始化全局 Logger。
func Init(cfg Config, release ReleaseInfo) error {
	logger, err := NewLogger(cfg, release)
	if err != nil {
		return err
	}
	SetDefault(logger)
	return nil
}

// L 获取全局 Logger。如果未初始化，返回默认 Logger。
func L() *Logger {
	defaultMu.RLock()
	l := defaultLogger
	defaultMu.RUnlock()

	if l != nil {
		return l
	}

	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultLogger != nil { // double-check
		return defaultLogger
	}

	// 返回默认 Logger（stdout, info level, json format）
	defaultCfg := NewConfig()
	l, _ = NewLogger(defaultCfg, ReleaseInfo{})
	defaultLogger = l
	return l
}

// SetDefault 设置全局 Logger。
func SetDefault(l *Logger) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultLogger = l
}

// Close 关闭全局 Logger。
func Close() error {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultLogger != nil {
		err := defaultLogger.Close()
		defaultLogger = nil
		return err
	}
	return nil
}
