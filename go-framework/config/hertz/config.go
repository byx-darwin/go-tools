// Package config 提供 Hertz 服务的配置类型。
package hertz

import (
	"github.com/byx-darwin/go-tools/go-framework/config"
	"time"
)

// ServerConfig Hertz HTTP 服务配置
type ServerConfig struct {
	Registry config.RegistryOption `json:"registry" yaml:"registry"`
	Jaeger   *config.JaegerOption  `json:"jaeger" yaml:"jaeger"`
	HTTP     *HTTPOption           `json:"http" yaml:"http"`
	Auth     *HTTPAuth             `json:"auth" yaml:"auth"`
}

// HTTPOption HTTP 服务选项
type HTTPOption struct {
	Network      string        `json:"network"  yaml:"network"`
	Port         string        `json:"port"  yaml:"port"`
	Mode         int           `json:"mode"  yaml:"mode"` // 0:内网 1:外网
	ExitWaitTime time.Duration `json:"exit_wait_time" yaml:"exit_wait_time"`
	IdleTimeout  time.Duration `json:"idle_timeout" yaml:"idle_timeout"`
	IsTransport  bool          `json:"is_transport" yaml:"is_transport"`
	IsCors       bool          `json:"is_cors" yaml:"is_cors"`
	IsRecovery   bool          `json:"is_recovery" yaml:"is_recovery"`
}

// HTTPAuth 鉴权配置
type HTTPAuth struct {
	Enable bool   `json:"enable"  yaml:"enable"`
	AK     string `json:"ak"  yaml:"ak"`
	SK     string `json:"sk"  yaml:"sk"`
	TeaKey string `json:"tea_key"  yaml:"tea_key"`
}
