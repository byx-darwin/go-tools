package hertz

import (
	"context"
	hertzConfig "gitee.com/byx_darwin/go-tools/config/hertz"
	"gitee.com/byx_darwin/go-tools/hertz/middleware"
	"gitee.com/byx_darwin/go-tools/hertz/registry/polaris"
	"gitee.com/byx_darwin/go-tools/tools/netutil"
	"gitee.com/byx_darwin/uptrace-opentelemetry/otelhertz"
	"github.com/cloudwego/hertz/pkg/app/middlewares/server/recovery"
	httpServer "github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/network/standard"
	"time"
)

func NewHTTPServer(ctx context.Context,
	serverConfig *hertzConfig.ServerConfig, log hlog.CtxLogger) (*httpServer.Hertz, error) {
	options := make([]config.Option, 0)
	internalIP, err := netutil.GetInternalIP()
	if err != nil {
		log.CtxFatalf(ctx, "GetInternalIP error:%v", err)
	}
	if serverConfig.HTTP.IdleTimeout == 0 {
		serverConfig.HTTP.IdleTimeout = 3000
	}
	options = append(options,
		httpServer.WithIdleTimeout(time.Duration(serverConfig.HTTP.IdleTimeout)*time.Millisecond))
	if serverConfig.HTTP.ExitWaitTime == 0 {
		serverConfig.HTTP.ExitWaitTime = 5000
	}
	tracer, cfg := otelhertz.NewServerTracer()
	if serverConfig.Jaeger.Enable {
		options = append(options, tracer)
	}
	if serverConfig.Registry.Enable {
		registryOptions, err := polaris.NewRegistry(ctx, serverConfig.Registry, log)
		if err != nil {
			log.CtxFatalf(ctx, "NewRegistry error:%v", err)
			return nil, err
		}
		options = append(options, registryOptions...)
	}
	options = append(options,
		httpServer.WithExitWaitTime(time.Duration(serverConfig.HTTP.ExitWaitTime)*time.Millisecond))
	options = append(options, httpServer.WithNetwork(serverConfig.HTTP.Network))

	address := ""
	if serverConfig.HTTP.Mode == 0 {
		address = internalIP + ":" + serverConfig.HTTP.Port
	} else {
		address = ":" + serverConfig.HTTP.Port
	}
	if serverConfig.HTTP.IsTransport {
		options = append(options, httpServer.WithTransport(standard.NewTransporter))
	}

	options = append(options, httpServer.WithHostPorts(address))
	hertz := httpServer.New(options...)
	if serverConfig.HTTP.IsCors {
		hertz.Use(middleware.Cors())
	}
	if serverConfig.HTTP.IsRecovery {
		hertz.Use(recovery.Recovery())
	}
	if serverConfig.Jaeger.Enable {
		hertz.Use(otelhertz.ServerMiddleware(cfg))
	}
	hertz.OnShutdown = append(hertz.OnShutdown, func(ctx context.Context) {
		hlog.CtxInfof(ctx, "exit timeout!")
		<-ctx.Done()
	})
	return hertz, nil
}
