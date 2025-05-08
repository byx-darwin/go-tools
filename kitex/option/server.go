package option

import (
	"context"
	"gitee.com/byx_darwin/go-tools/config/kitex"
	"gitee.com/byx_darwin/go-tools/kitex/registry/polaris"
	"gitee.com/byx_darwin/go-tools/tools/netutil"
	"gitee.com/byx_darwin/uptrace-opentelemetry/otelkitex"
	"gitee.com/byx_darwin/uptrace-opentelemetry/otelplay"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/limit"
	"github.com/cloudwego/kitex/pkg/remote/codec/thrift"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/pkg/transmeta"
	"github.com/cloudwego/kitex/server"
	"net"
	"time"
)

func NewServerOption(ctx context.Context,
	config *kitex.ServerConfig, log klog.CtxLogger) ([]server.Option, error) {
	options := make([]server.Option, 0)
	internalIP, err := netutil.GetInternalIP()
	if err != nil {
		log.CtxFatalf(ctx, "GetInternalIP error:%v", err)
		return nil, err
	}
	address := config.RPC.Port
	if address == "" {
		address = internalIP + ":9000"
	} else if address[0] == ':' {
		address = internalIP + address
	} else if address[0] != ':' {
		address = internalIP + ":" + address
	}
	addr, err := net.ResolveTCPAddr(config.RPC.Network, address)
	if err != nil {
		log.CtxFatalf(ctx, "ResolveTCPAddr error addr:%v err:%v", addr, err)
		return nil, err
	}
	options = append(options, server.WithServiceAddr(addr))
	if config.Registry.Enable {
		registryOptions, err := polaris.NewRegistry(ctx, config.Registry.Space, config.Registry.Name, log)
		if err != nil {
			log.CtxFatalf(ctx, "NewRegistry error:%v", err)
			return nil, err
		}
		options = append(options, registryOptions...)
	}
	if config.Jaeger.Enable {
		shutdown := otelplay.ConfigureOpentelemetry(ctx, &otelplay.Config{
			ServiceDSN:     config.Jaeger.Endpoint,
			ServiceName:    config.Registry.Name,
			ServiceVersion: config.Registry.Version,
			Environment:    config.Registry.Env,
		})
		defer shutdown()
		options = append(options, server.WithSuite(otelkitex.NewServerSuite()))
		options = append(options, server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: config.Registry.Name}))
	}
	if config.Limit.Enable {
		options = append(options, server.WithLimit(&limit.Option{
			MaxConnections: config.Limit.MaxConnections,
			MaxQPS:         config.Limit.MaxQPS,
		}))
	}
	if config.Timeout.ReadWriteTimeout > 0 {
		options = append(options, server.WithReadWriteTimeout(time.Duration(config.Timeout.ReadWriteTimeout)*time.Millisecond))
	} else {
		options = append(options, server.WithReadWriteTimeout(5*time.Second))
	}
	if config.Timeout.ExitWaitTimeout > 0 {
		options = append(options, server.WithExitWaitTime(time.Duration(config.Timeout.ExitWaitTimeout)*time.Millisecond))
	} else {
		options = append(options, server.WithExitWaitTime(5*time.Second))
	}
	//启用多路复用
	options = append(options, server.WithMuxTransport())
	options = append(options, server.WithMetaHandler(transmeta.ServerTTHeaderHandler))
	code := thrift.NewThriftCodec()
	options = append(options, server.WithPayloadCodec(code))
	return options, nil

}
