// Package es 提供 Elasticsearch 客户端配置和工厂方法。
package es

// Config Elasticsearch 连接配置
type Config struct {
	// Addresses ES 节点地址列表
	Addresses []string `json:"addresses" yaml:"addresses"`

	// Username 用户名
	Username string `json:"username" yaml:"username"`
	// Password 密码
	Password string `json:"password" yaml:"password"`
	// APIKey API 密钥（与 Username/Password 二选一）
	APIKey string `json:"api_key" yaml:"api_key"`

	// CloudID Elastic Cloud ID
	CloudID string `json:"cloud_id" yaml:"cloud_id"`

	// TLS 配置
	TLS struct {
		// Enable 是否启用 TLS
		Enable bool `json:"enable" yaml:"enable"`
		// InsecureSkipVerify 跳过证书验证
		InsecureSkipVerify bool `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
	} `json:"tls" yaml:"tls"`

	// 连接池配置
	MaxRetries          int `json:"max_retries" yaml:"max_retries"`                     // 最大重试次数
	MaxIdleConnsPerHost int `json:"max_idle_conns_per_host" yaml:"max_idle_conns_per_host"` // 每个 host 最大空闲连接
}
