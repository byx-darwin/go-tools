// Package option 提供 Kitex RPC 服务端和客户端的 Option 工厂。
//
// 注意：因 kitex SDK 和 otel 依赖存在 genproto 冲突，本文件暂用 build ignore 隔离。
// 当上游修复后，删除第一行的 //go:build ignore 即可启用。
//
// 提供的工厂函数：
//   - NewServerOption(ctx, cfg, log) → []server.Option  (Kitex 服务端)
//   - NewClientOption(ctx, cfg, log) → []client.Option  (Kitex 客户端)
//
// 功能覆盖：
//   - 服务地址解析（内网 IP + 端口）
//   - Polaris 服务注册与发现
//   - Jaeger 链路追踪（OTel）
//   - 限流、超时、长连接池
//   - 传输协议 TTHeaderStreaming（同时兼容 unary 和 streaming）
//   - 负载均衡（一致性哈希）
//   - 失败重试
package option

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/byx-darwin/go-tools/go-common/netutil"
	"github.com/byx-darwin/go-tools/go-framework/config/kitex"
	"github.com/byx-darwin/go-tools/go-framework/kitex/rpcerror"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/connpool"
	"github.com/cloudwego/kitex/pkg/limit"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	remoteConnpool "github.com/cloudwego/kitex/pkg/remote/connpool"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/transmeta"
	"github.com/cloudwego/kitex/server"
	"github.com/cloudwego/kitex/transport"
)

// ── Server ──

// NewServerOption 创建 Kitex 服务端 Option 列表。
func NewServerOption(ctx context.Context, cfg *kitex.ServerConfig) ([]server.Option, error) {
	if cfg == nil {
		return nil, rpcerror.ErrConfigInvalid.With("step", "NewServerOption").Wrap(
			errors.New("server config is nil"))
	}
	if cfg.RPC == nil {
		cfg.RPC = &kitex.RPCOption{Network: "tcp"}
	}
	if cfg.RPC.Network == "" {
		cfg.RPC.Network = "tcp"
	}

	internalIP, err := netutil.GetInternalIP()
	if err != nil {
		return nil, rpcerror.ErrSystem.With("step", "get_internal_ip").Wrap(err)
	}

	addr := resolveAddr(internalIP, cfg.RPC.Port)
	tcpAddr, err := net.ResolveTCPAddr(cfg.RPC.Network, addr)
	if err != nil {
		return nil, rpcerror.ErrConfigInvalid.With("addr", addr).With("network", cfg.RPC.Network).Wrap(err)
	}

	options := []server.Option{
		server.WithServiceAddr(tcpAddr),
		server.WithMetaHandler(transmeta.ServerTTHeaderHandler),
		server.WithPayloadCodec(thrift.NewThriftCodec()),
	}
	if cfg.Limit != nil && cfg.Limit.Enable {
		options = append(options, server.WithLimit(&limit.Option{
			MaxConnections: cfg.Limit.MaxConnections,
			MaxQPS:         cfg.Limit.MaxQPS,
		}))
	}

	if cfg.Timeout != nil && cfg.Timeout.ReadWriteTimeout > 0 {
		options = append(options, server.WithReadWriteTimeout(cfg.Timeout.ReadWriteTimeout))
	} else {
		options = append(options, server.WithReadWriteTimeout(5*time.Second))
	}

	if cfg.Timeout != nil && cfg.Timeout.ExitWaitTimeout > 0 {
		options = append(options, server.WithExitWaitTime(cfg.Timeout.ExitWaitTimeout))
	} else {
		options = append(options, server.WithExitWaitTime(5*time.Second))
	}

	return options, nil
}

func resolveAddr(ip, port string) string {
	if port == "" {
		return ip + ":9000"
	}
	if port[0] == ':' {
		return ip + port
	}
	return ip + ":" + port
}

// ── Client ──

// NewClientOption 创建 Kitex 客户端 Option 列表。
func NewClientOption(ctx context.Context, cfg *kitex.ClientConfig) ([]client.Option, error) {
	if cfg == nil || cfg.ClientOption == nil {
		return nil, rpcerror.ErrConfigInvalid.With("step", "NewClientOption").Wrap(
			errors.New("client config is nil"))
	}
	co := cfg.ClientOption

	options := []client.Option{
		client.WithPayloadCodec(thrift.NewThriftCodec()),
		client.WithMetaHandler(transmeta.ClientTTHeaderHandler),
		client.WithTransportProtocol(transport.TTHeaderStreaming),
	}

	if cfg.RPC != nil && cfg.RPC.Intranet != "" && !co.Resolver.Enable {
		options = append(options, client.WithHostPorts(cfg.RPC.Intranet))
	}

	if co.Timeout.ConnectTimeOut > 0 {
		options = append(options, client.WithConnectTimeout(co.Timeout.ConnectTimeOut))
	} else {
		options = append(options, client.WithConnectTimeout(50*time.Millisecond))
	}

	if co.Timeout.RPCTimeout > 0 {
		options = append(options, client.WithRPCTimeout(co.Timeout.RPCTimeout))
	}

	// 长连接池（替代已废弃的 Mux Connection）
	options = append(options, client.WithConnPool(remoteConnpool.NewLongPool(
		co.Resolver.Name,
		connpool.IdleConfig{
			MinIdlePerAddress: co.ConnPool.MinIdlePerAddress,
			MaxIdlePerAddress: co.ConnPool.MaxIdlePerAddress,
			MaxIdleGlobal:     co.ConnPool.MaxIdleGlobal,
			MaxIdleTimeout:    co.ConnPool.MaxIdleTimeout,
		},
	)))

	if co.Failure.Enable {
		fp := retry.NewFailurePolicy()
		fp.WithMaxRetryTimes(co.Failure.MaxRetryTimes)
		options = append(options, client.WithFailureRetry(fp))
	}

	if co.LoadBalancer.Enable {
		options = append(options, client.WithLoadBalancer(loadbalance.NewConsistBalancer(
			loadbalance.NewConsistentHashOption(func(ctx context.Context, request any) string {
				if s, ok := request.(interface{ Key() string }); ok {
					return s.Key()
				}
				return ""
			}))))
	}

	options = append(options, client.WithClientBasicInfo(&rpcinfo.EndpointBasicInfo{
		ServiceName: co.Resolver.Name,
	}))

	return options, nil
}
