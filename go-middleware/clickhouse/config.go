// Package clickhouse 提供 ClickHouse 客户端配置和工厂方法。
package clickhouse

import "time"

// Config ClickHouse 连接配置
type Config struct {
	// DSN 连接字符串（clickhouse://user:password@host:port/database?options）
	DSN string `json:"dsn" yaml:"dsn"`

	// 以下为独立字段配置（与 DSN 二选一，优先 DSN）
	Addrs    []string `json:"addrs" yaml:"addrs"`       // 节点地址
	Database string   `json:"database" yaml:"database"` // 数据库名
	Username string   `json:"username" yaml:"username"` // 用户名
	Password string   `json:"password" yaml:"password"` // 密码

	// 连接配置
	DialTimeout      int `json:"dial_timeout" yaml:"dial_timeout"`           // 连接超时（秒）
	MaxOpenConns     int `json:"max_open_conns" yaml:"max_open_conns"`       // 最大打开连接数
	MaxIdleConns     int `json:"max_idle_conns" yaml:"max_idle_conns"`       // 最大空闲连接数
	ConnMaxLifetime  int `json:"conn_max_lifetime" yaml:"conn_max_lifetime"`  // 连接最大生命周期（秒）
	Compress         bool `json:"compress" yaml:"compress"`                  // 是否启用压缩

	// TLS 配置
	TLS struct {
		Enable             bool `json:"enable" yaml:"enable"`
		InsecureSkipVerify bool `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	} `json:"tls" yaml:"tls"`
}

// sec converts seconds to time.Duration.
func sec(v int) time.Duration {
	return time.Duration(v) * time.Second
}
