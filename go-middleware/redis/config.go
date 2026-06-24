// Package redis 提供 Redis 客户端配置与工厂方法。
//
// 配置字段与 ncgo 模板对齐，时间单位统一使用 time.Duration（D2 决策）。
package redis

import "time"

// Config Redis 客户端配置（与 ncgo redis.yaml 对齐）
type Config struct {
	// Addrs 节点地址（单节点取第一个，哨兵/集群取全部）
	Addrs []string `json:"addrs" yaml:"addrs"`

	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	DB       int    `json:"db" yaml:"db"`

	// Sentinel 配置
	MasterName       string `json:"master_name" yaml:"master_name"`
	SentinelUsername string `json:"sentinel_username" yaml:"sentinel_username"`
	SentinelPassword string `json:"sentinel_password" yaml:"sentinel_password"`

	// Protocol 协议版本（2=RESP2, 3=RESP3）
	Protocol   int    `json:"protocol" yaml:"protocol"`
	ClientName string `json:"client_name" yaml:"client_name"`

	// 连接池
	PoolSize     int `json:"pool_size" yaml:"pool_size"`
	MinIdleConns int `json:"min_idle_conns" yaml:"min_idle_conns"`

	// 超时（time.Duration per D2）
	DialTimeout  time.Duration `json:"dial_timeout" yaml:"dial_timeout"`
	ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
	PoolTimeout  time.Duration `json:"pool_timeout" yaml:"pool_timeout"`

	// 连接生命周期
	ConnMaxIdleTime    time.Duration `json:"conn_max_idle_time" yaml:"conn_max_idle_time"`
	ConnMaxLifetime    time.Duration `json:"conn_max_lifetime" yaml:"conn_max_lifetime"`
	IdleCheckFrequency time.Duration `json:"idle_check_frequency" yaml:"idle_check_frequency"`

	// 重试
	MaxRetries      int           `json:"max_retries" yaml:"max_retries"`
	MinRetryBackoff time.Duration `json:"min_retry_backoff" yaml:"min_retry_backoff"`
	MaxRetryBackoff time.Duration `json:"max_retry_backoff" yaml:"max_retry_backoff"`
}
