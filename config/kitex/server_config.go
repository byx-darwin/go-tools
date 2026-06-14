package kitex

import "gitee.com/byx_darwin/go-tools/config"

type ServerConfig struct {
	RPC      *RPCOption             `json:"rpc" yaml:"rpc"`
	Registry *config.RegistryOption `json:"registry" yaml:"registry"`
	Jaeger   *config.JaegerOption   `json:"jaeger" yaml:"jaeger"`
	Limit    *LimitOption           `json:"limit" yaml:"limit"`
	Timeout  *ServerTimeout         `json:"timeout" yaml:"timeout"`
}

// RPCOption RPC 服务地址端口配置
type RPCOption struct {
	Port    string `json:"port" yaml:"port"`        //端口
	Network string `json:"network"  yaml:"network"` //连接方式 (tcp udp)
	Mode    int    `json:"mode"  yaml:"mode"`       //运行模式 0:本地模式 1:局域网模式
}

// LimitOption 限流器配置
type LimitOption struct {
	Enable         bool `json:"enable" yaml:"enable"`                    //是否启用限流器
	MaxConnections int  `json:"max_connections"  yaml:"max_connections"` // 最大连接数
	MaxQPS         int  `json:"max_qps"  yaml:"max_qps"`                 //最大qps
}

// ServerTimeout 超时控制
type ServerTimeout struct {
	ReadWriteTimeout int `json:"read_write_timeout"  yaml:"read_write_timeout"` //读写超时 (单位ms 默认5s)
	ExitWaitTimeout  int `json:"connect_timeout"  yaml:"connect_timeout"`       //Server 在收到退出信号时的等待时间 (单位ms 默认5s)
}
