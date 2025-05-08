package hertz

import "gitee.com/byx_darwin/go-tools/config"

type ServerConfig struct {
	Registry *config.RegistryOption `json:"registry" yaml:"registry"`
	Jaeger   *config.JaegerOption   `json:"jaeger" yaml:"jaeger"`
	HTTP     *HTTPOption            `json:"http" yaml:"http"`
	Auth     *HTTPAuth              `json:"auth" yaml:"auth"`
}
type HTTPOption struct {
	Network      string `json:"network"  yaml:"network"`              //连接方式 (tcp udp unix)
	Address      string `json:"address"  yaml:"address"`              //地址
	ExitWaitTime int    `json:"exit_wait_time" yaml:"exit_wait_time"` // 优雅退出时间  单位ms
	IdleTimeout  int    `json:"idle_timeout" yaml:"idle_timeout"`     // 长连接请求链接空闲超时时间 单位ms
	IsTransport  bool   `json:"is_transport" yaml:"is_transport"`
	IsCors       bool   `json:"is_cors" yaml:"is_cors"`
	IsRecovery   bool   `json:"is_recovery" yaml:"is_recovery"`
}

type HTTPAuth struct {
	Enable bool   `json:"enable"  yaml:"enable"` //是否启用auth配置
	AK     string `json:"ak"  yaml:"ak"`
	SK     string `json:"sk"  yaml:"sk"`
	TeaKey string ` json:"tea_key"  yaml:"tea_key"`
}
