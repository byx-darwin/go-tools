package kitex

import (
	"time"

	"github.com/byx-darwin/go-tools/go-framework/config"
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
	Resolver ResolverOption `json:"resolver" yaml:"resolver"`
	// Deprecated: MuxConnNum is unused — Kitex has deprecated Mux Connection
	// ("no longer being maintained"); client.WithMuxConnection was removed from
	// go-framework/kitex/option. Field kept for YAML backward compatibility.
	MuxConnNum   int                 `json:"mux_conn_num" yaml:"mux_conn_num"`
	Timeout      ClientTimeout       `json:"timeout" yaml:"timeout"`
	Jaeger       config.JaegerOption `json:"jaeger" yaml:"jaeger"`
	Failure      FailureRetry        `json:"failure_retry" yaml:"failure_retry"`
	LoadBalancer LoadBalancer        `json:"load_balancer" yaml:"load_balancer"`
	CBSuite      CBSuite             `json:"circuit_breaker" yaml:"circuit_breaker"`
	ConnPool     ConnPool            `json:"conn_pool" yaml:"conn_pool"`
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

// ConnPool 长连接池配置（替代已废弃的 Mux Connection）。
// 对应 Kitex pkg/remote/connpool.NewLongPool + client.WithConnPool。
//
// 官方最佳实践（cloudwego.io）：
//   - 一般不需要调整连接池大小，不当调整反而可能降低性能。
//   - MaxIdleGlobal 应满足：MaxIdleGlobal ≥ 下游实例数 × MaxIdlePerAddress。
//   - v0.7.2 起 MaxIdleGlobal 可不设置（默认无上限），由 Kitex 自动管理。
//
// YAML 示例：
//
//	client:
//	  conn_pool:
//	    min_idle_per_address: 1     # 每个后端地址最少保持的空闲连接数（上限 5，默认 0）
//	    max_idle_per_address: 10    # 每个后端地址最多缓存的空闲连接数（默认 1）
//	    max_idle_global: 1000       # 全局最大空闲连接总数（默认无上限，建议 ≥ 实例数 × max_idle_per_address）
//	    max_idle_timeout: 30s       # 空闲连接最大存活时间，到期回收（最小 2s，默认 30s）
type ConnPool struct {
	// MinIdlePerAddress 每个后端地址最少保持的空闲连接数。
	// 连接池会预热并保持至少这么多连接，避免突发请求时临时建连。
	// 取值范围 0-5，超过 5 会被截断为 5。默认 0（不预热）。
	MinIdlePerAddress int `json:"min_idle_per_address" yaml:"min_idle_per_address"`

	// MaxIdlePerAddress 每个后端地址最多缓存的空闲连接数。
	// 高并发场景可适当调大，减少建连开销。默认 1。
	MaxIdlePerAddress int `json:"max_idle_per_address" yaml:"max_idle_per_address"`

	// MaxIdleGlobal 全局所有地址合计最大空闲连接总数。
	// 防止连接数随服务实例数线性膨胀。
	// 建议值：MaxIdleGlobal ≥ 下游实例数 × MaxIdlePerAddress。
	// 默认无上限（v0.7.2+），可不设置让 Kitex 自动管理。
	MaxIdleGlobal int `json:"max_idle_global" yaml:"max_idle_global"`

	// MaxIdleTimeout 空闲连接的最大存活时间，超时后自动回收。
	// 最小值 2s，默认 30s。使用 time.Duration，YAML 支持 30s / 1m 等格式。
	MaxIdleTimeout time.Duration `json:"max_idle_timeout" yaml:"max_idle_timeout"`
}
