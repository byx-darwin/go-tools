package option

import (
	"context"
	"gitee.com/byx_darwin/go-tools/config/kitex"
	"gitee.com/byx_darwin/go-tools/kitex/discover/polaris"
	"gitee.com/byx_darwin/uptrace-opentelemetry/otelkitex"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/loadbalance"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/retry"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/transmeta"
	"github.com/cloudwego/kitex/transport"
	"time"
)

type ctxKey int

const (
	ctxConsistedKey ctxKey = iota
)

func NewClientOption(ctx context.Context,
	config *kitex.ClientConfig, log hlog.CtxLogger) ([]client.Option, error) {
	options := make([]client.Option, 0)
	if config.ClientOption.Resolver.Enable {
		resolver, err := polaris.NewResolver(ctx, config.ClientOption.Resolver.Space, log)
		if err != nil {
			log.CtxFatalf(ctx, "NewResolver error:%v", err)
		}
		options = append(options, resolver)
	} else {
		options = append(options, client.WithHostPorts(config.RPC.Intranet)) // 设置RPC客户端的地址
	}
	if config.ClientOption.Timeout.ConnectTimeOut > 0 {
		options = append(options, client.WithConnectTimeout(time.Duration(config.ClientOption.Timeout.ConnectTimeOut)*time.Millisecond))

	} else {
		options = append(options, client.WithConnectTimeout(50*time.Millisecond))
	}
	if config.ClientOption.Timeout.RPCTimeout > 0 {
		options = append(options, client.WithRPCTimeout(time.Duration(config.ClientOption.Timeout.RPCTimeout)*time.Millisecond))
	}
	if config.ClientOption.Jaeger.Enable {
		options = append(options, client.WithSuite(otelkitex.NewClientSuite()))
		options = append(options, client.WithClientBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: config.ClientOption.Resolver.Name}))
		log.CtxDebugf(ctx, "客户端配置链路已配置成功 ClientName：%v，Endpoint:%v", config.ClientOption.Resolver.Name, config.ClientOption.Jaeger.Endpoint)
	}

	//请求重试机制
	if config.ClientOption.Failure.Enable {
		failurePolicy := retry.NewFailurePolicy()
		failurePolicy.WithMaxRetryTimes(config.ClientOption.Failure.MaxRetryTimes)
		options = append(options, client.WithFailureRetry(failurePolicy))
		log.CtxDebugf(ctx, "客户端配置请求重试机制 ClientName:%s 已启用 重试次数：%v", config.ClientOption.Resolver.Name, config.ClientOption.Failure.MaxRetryTimes)
	}
	//多路复用
	if config.ClientOption.MuxConnNum == 0 {
		options = append(options, client.WithMuxConnection(2))
	} else {
		options = append(options, client.WithMuxConnection(config.ClientOption.MuxConnNum))
	}
	//负载均衡
	if config.ClientOption.LoadBalancer.Enable {
		options = append(options, client.WithLoadBalancer(loadbalance.NewConsistBalancer(
			loadbalance.NewConsistentHashOption(func(ctx context.Context, request interface{}) string {
				return ctx.Value(ctxConsistedKey).(string)
			}))))
		log.CtxDebugf(ctx, "客户端配置负载均衡 ClientName:%s 已启用 ：%v", config.ClientOption.Resolver.Name, config.ClientOption.LoadBalancer.Enable)
	}
	code := thrift.NewThriftCodec()
	options = append(options, client.WithPayloadCodec(code))
	options = append(options, client.WithMetaHandler(transmeta.ClientTTHeaderHandler))
	options = append(options, client.WithTransportProtocol(transport.TTHeaderFramed))
	options = append(options, client.WithClientBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: config.ClientOption.Resolver.Name})) // 设置RPC客户端的基本信息
	return options, nil

}
