package kitex

import (
	"gitee.com/byx_darwin/go-tools/go-framework/config"
	"time"
)

// ClientConfig Kitex RPC 客户端配置
type ClientConfig struct {
	RPC          *RPCServerOption `json:"rpc_server" yaml:"rpc_server"`
	ClientOption *ClientOption    `json:"client" yaml:"client"`
}

// RPCServerOption 远程 RPC 服务信息
type RPCServerOption struct {
	Name     string `json:"name"  yaml:"name"`
	Intranet string `json:"intranet"   yaml:"intranet"`
}

// ClientOption 客户端选项
type ClientOption struct {
	Resolver     ResolverOption      `json:"resolver" yaml:"resolver"`
	MuxConnNum   int                 `json:"mux_conn_num" yaml:"mux_conn_num"`
	Timeout      ClientTimeout       `json:"timeout" yaml:"timeout"`
	Jaeger       config.JaegerOption `json:"jaeger" yaml:"jaeger"`
	Failure      FailureRetry        `json:"failure_retry" yaml:"failure_retry"`
	LoadBalancer LoadBalancer        `json:"load_balancer" yaml:"load_balancer"`
	CBSuite      CBSuite             `json:"circuit_breaker" yaml:"circuit_breaker"`
}

// ResolverOption 服务发现
type ResolverOption struct {
	Enable  bool   `json:"enable" yaml:"enable"`
	Space   string `json:"space"  yaml:"space"`
	Name    string `json:"name"  yaml:"name"`
	Version string `json:"version"  yaml:"version"`
	Env     string `json:"env" yaml:"env"`
}

// FailureRetry 重试机制
type FailureRetry struct {
	Enable        bool `json:"enable"  yaml:"enable"`
	MaxRetryTimes int  `json:"max_retry_times" yaml:"max_retry_times"`
}

// LoadBalancer 负载均衡
type LoadBalancer struct {
	Enable bool `json:"enable"  yaml:"enable"`
}

// CBSuite 熔断器
type CBSuite struct {
	Enable bool `json:"enable"  yaml:"enable"`
}

// ClientTimeout 超时控制（Duration per D2）
type ClientTimeout struct {
	RPCTimeout     time.Duration `json:"rpc_timeout"  yaml:"rpc_timeout"`
	ConnectTimeOut time.Duration `json:"connect_timeout"  yaml:"connect_timeout"`
}
