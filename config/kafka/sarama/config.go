package sarama

// Config 定义了 Kafka 客户端配置的结构体
type Config struct {
	TLSOption  TLSOption  `json:"tls_option"  yaml:"tls_option"`    // TLS 配置选项
	CAOption   CAOption   `json:"ca_option" yaml:"ca_option"`       // CA 证书配置选项
	SASLOption SASLOption `json:"sasl_option"   yaml:"sasl_option"` // SASL 认证配置选项
	NetOption  NetOption  `json:"net_option"  yaml:"net_option"`    // 网络相关配置选项
	Timeout    int        `json:"timeout"  yaml:"timeout"`          // 超时时间(秒)
	Broker     []string   `json:"broker"  yaml:"broker"`            // Kafka broker 地址列表
}

// TLSOption 定义了 TLS 安全传输层配置
type TLSOption struct {
	Enable   bool   `json:"enable"  yaml:"enable"`       // 是否启用 TLS
	CertPath string `json:"cert_path"  yaml:"cert_path"` // 证书文件路径
	KeyPath  string `json:"key_path"  yaml:"key_path"`   // 私钥文件路径
}

// NetOption 定义了网络连接相关配置
type NetOption struct {
	MaxOpenRequests int `json:"max_open_requests"  yaml:"max_open_requests"` // 最大并发请求数
	DialTimeout     int `json:"dial_timeout"  yaml:"dial_timeout"`           // 连接超时时间(秒)
	ReadTimeout     int `json:"read_timeout"  yaml:"read_timeout"`           // 读取超时时间(秒)
	WriteTimeout    int `json:"write_timeout"  yaml:"write_timeout"`         // 写入超时时间(秒)
}

// CAOption 定义了 CA 证书配置
type CAOption struct {
	Enable bool   `json:"enable"  yaml:"enable"`   // 是否启用 CA 证书
	CAPath string `json:"ca_path"  yaml:"ca_path"` // CA 证书文件路径
}

// SASLOption 定义了 SASL 认证配置
type SASLOption struct {
	Enable   bool   `json:"enable"  yaml:"enable"`    // 是否启用 SASL 认证
	User     string `json:"user"  yaml:"user"`        // SASL 用户名
	Password string `json:"password" yaml:"password"` // SASL 密码
}
