package kitex

import "gitee.com/byx_darwin/go-tools/config"

type ClientConfig struct {
	RPC          *RPCServerOption `json:"rpc_server" yaml:"rpc_server"`
	ClientOption *ClientOption    `json:"client" yaml:"client"`
}

type ClientOption struct {
	Resolver     ResolverOption      `json:"resolver" yaml:"resolver"`
	MuxConnNum   int                 `json:"mux_conn_num" yaml:"mux_conn_num"`
	Timeout      ClientTimeout       `json:"timeout" yaml:"timeout"`
	Jaeger       config.JaegerOption `json:"jaeger" yaml:"jaeger"`
	Failure      FailureRetry        `json:"failure_retry" yaml:"failure_retry"`
	LoadBalancer LoadBalancer        `json:"load_balancer" yaml:"load_balancer"`
	CBSuite      CBSuite             `json:"circuit_breaker" yaml:"circuit_breaker"`
}

type RPCServerOption struct {
	Name     string `json:"name"  yaml:"name"`
	Intranet string `json:"intranet"   yaml:"intranet"`
}

type ResolverOption struct {
	Enable  bool   `json:"enable" yaml:"enable"`    //是否启用服务发现
	Space   string `json:"space"  yaml:"space"`     //服务空间名称
	Name    string `json:"name"  yaml:"name"`       //服务名称
	Version string `json:"version"  yaml:"version"` //版本信息
	Env     string `json:"env" yaml:"env"`          //环境信息
}

// FailureRetry 重试机制
type FailureRetry struct {
	Enable        bool `json:"enable"  yaml:"enable"`                  //是否启用请求重试机制
	MaxRetryTimes int  `json:"max_retry_times" yaml:"max_retry_times"` //重试次数
}

// LoadBalancer 负载均衡
type LoadBalancer struct {
	Enable bool `json:"enable"  yaml:"enable"` //是否启用负载均衡
}

// CBSuite 熔断器
type CBSuite struct {
	Enable bool `json:"enable"  yaml:"enable"` //是否启用熔断器
}

// ClientTimeout 超时控制
type ClientTimeout struct {
	RPCTimeout     int `json:"rpc_timeout"  yaml:"rpc_timeout"`         //rpc超时时间 (单位ms 默认不限制)
	ConnectTimeOut int `json:"connect_timeout"  yaml:"connect_timeout"` //连接超时控制 (单位ms 默认50ms)
}
