// Package hertz 提供 Hertz HTTP 服务工厂。
package hertz

import (
	"context"
	"time"

	"github.com/byx-darwin/go-tools/go-common/netutil"
	"github.com/byx-darwin/go-tools/go-framework/config"
	hertzConfig "github.com/byx-darwin/go-tools/go-framework/config/hertz"
	"github.com/byx-darwin/go-tools/go-framework/hertz/middleware"
	"github.com/byx-darwin/go-tools/go-framework/hertz/observability"
	"github.com/cloudwego/hertz/pkg/app/middlewares/server/recovery"
	httpServer "github.com/cloudwego/hertz/pkg/app/server"
	hertz "github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/network/standard"
)

// NewHTTPServer 创建 Hertz HTTP 服务。
func NewHTTPServer(ctx context.Context, serverConfig *hertzConfig.ServerConfig) (*httpServer.Hertz, error) {
	options := make([]hertz.Option, 0)

	internalIP, err := netutil.GetInternalIP()
	if err != nil {
		return nil, err
	}

	if serverConfig.HTTP == nil {
		serverConfig.HTTP = &hertzConfig.HTTPOption{}
	}

	// 使用局部变量填充默认值，不修改传入的 config
	idleTimeout := serverConfig.HTTP.IdleTimeout
	if idleTimeout == 0 {
		idleTimeout = 3 * time.Second
	}
	exitWaitTime := serverConfig.HTTP.ExitWaitTime
	if exitWaitTime == 0 {
		exitWaitTime = 5 * time.Second
	}

	options = append(options,
		httpServer.WithExitWaitTime(exitWaitTime),
		httpServer.WithIdleTimeout(idleTimeout),
		httpServer.WithNetwork(serverConfig.HTTP.Network),
	)

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
	h := httpServer.New(options...)

	// 中间件
	if serverConfig.HTTP.IsCors {
		h.Use(middleware.Cors())
	}
	if serverConfig.HTTP.IsRecovery {
		h.Use(recovery.Recovery())
	}

	// OTel 链路追踪（使用自有 observability provider）
	if serverConfig.Jaeger != nil && serverConfig.Jaeger.Enable {
		provider, err := observability.NewProvider(ctx, config.ObservabilityConfig{
			Enabled:     true,
			Endpoint:    serverConfig.Jaeger.Endpoint,
			ServiceName: serverConfig.Registry.Name,
		})
		if err != nil {
			return nil, err
		}
		h.Use(provider.ServerMiddleware())
		h.OnShutdown = append(h.OnShutdown, func(ctx context.Context) { _ = provider.Shutdown() })
	}

	h.OnShutdown = append(h.OnShutdown, func(ctx context.Context) {
		<-ctx.Done()
	})
	return h, nil
}
