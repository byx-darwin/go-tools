// Package db 提供数据库配置结构体。
package db

// Config 数据库配置结构体
type Config struct {
	// 数据库驱动类型 (mysql, postgres, sqlite3, mongodb)
	Driver string `json:"driver"  yaml:"driver"`
	// 数据库连接字符串
	Source string `json:"source" yaml:"source"`
	// 数据库名称
	Name string `json:"name" yaml:"name"`
	// 表前缀
	TablePrefix string `json:"table_prefix" yaml:"table_prefix"`
	// 是否开启SQL debug日志
	SqlLog bool `json:"sql_log" yaml:"sql_log"`
	// 最大打开连接数
	MaxOpenCons int `json:"max_open_cons"  yaml:"max_open_cons"`
	// 最大空闲连接数
	MaxIdleCons int `json:"max_idle_cons"  yaml:"max_idle_cons"`
	// 连接最大生命周期（秒）
	ConMaxLifetime int `json:"con_max_lifetime" yaml:"con_max_lifetime"`
	// 空闲连接最大存活时间（秒）
	MaxIdleTime int `json:"max_idle_time" yaml:"max_idle_time"`
}
