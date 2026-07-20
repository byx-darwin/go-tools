package db

// Option 定义 NewDB 的配置选项函数。
type Option func(*dbConfig)

// dbConfig 是 NewDB 的内部配置聚合。
type dbConfig struct {
	driver string
	source string
	pool   *Config
}

// WithDriver 设置数据库驱动名称（mysql, postgres, sqlite3 等）。
func WithDriver(driver string) Option {
	return func(c *dbConfig) {
		if driver != "" {
			c.driver = driver
		}
	}
}

// WithSource 设置数据库连接字符串（DSN）。
func WithSource(source string) Option {
	return func(c *dbConfig) {
		if source != "" {
			c.source = source
		}
	}
}

// WithPoolConfig 设置连接池配置。
func WithPoolConfig(cfg *Config) Option {
	return func(c *dbConfig) {
		if cfg != nil {
			c.pool = cfg
		}
	}
}
