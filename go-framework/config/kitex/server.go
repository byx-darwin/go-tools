// Package kitex 提供 Kitex 服务的配置类型。
package kitex

import (
	"time"

	"github.com/byx-darwin/go-tools/go-framework/config"
)

// ServerConfig Kitex RPC 服务配置
type ServerConfig struct {
	RPC      *RPCOption             `json:"rpc" yaml:"rpc"`
	Registry *config.RegistryOption `json:"registry" yaml:"registry"`
	Jaeger   *config.JaegerOption   `json:"jaeger" yaml:"jaeger"`
	Limit    *LimitOption           `json:"limit" yaml:"limit"`
	Timeout  *ServerTimeout         `json:"timeout" yaml:"timeout"`
}

// RPCOption RPC 服务地址端口
type RPCOption struct {
	Port    string `json:"port" yaml:"port"`
	Network string `json:"network"  yaml:"network"`
}

// LimitOption 限流器
type LimitOption struct {
	Enable         bool `json:"enable" yaml:"enable"`
	MaxConnections int  `json:"max_connections"  yaml:"max_connections"`
	MaxQPS         int  `json:"max_qps"  yaml:"max_qps"`
}

// ServerTimeout 超时控制（Duration per D2）
type ServerTimeout struct {
	ReadWriteTimeout time.Duration `json:"read_write_timeout"  yaml:"read_write_timeout"`
	ExitWaitTimeout  time.Duration `json:"exit_wait_timeout"  yaml:"exit_wait_timeout"`
}
