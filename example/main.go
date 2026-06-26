package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/byx-darwin/go-tools/go-common/log"
	hertzlog "github.com/byx-darwin/go-tools/go-framework/hertz/log"
	"github.com/byx-darwin/go-tools/go-framework/hertz/observability"
	kitexlog "github.com/byx-darwin/go-tools/go-framework/kitex/log"
	kitexobs "github.com/byx-darwin/go-tools/go-framework/kitex/observability"

	"github.com/cloudwego/hertz/pkg/app/server"
	hertzconfig "github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	klog "github.com/cloudwego/kitex/pkg/klog"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. 加载配置
	cfg, err := LoadConfig("config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	// 2. 初始化日志
	if err := log.Init(cfg.Log, log.ReleaseInfo{
		ServiceName: "go-tools-example",
		Version:     "v1.0.0",
		Environment: "development",
	}); err != nil {
		fmt.Fprintf(os.Stderr, "init log: %v\n", err)
		os.Exit(1)
	}
	defer log.Close()

	// 替换 Hertz 和 Kitex 默认日志为统一 Logger
	hlog.SetLogger(hertzlog.NewHertzAdapter(log.L()))
	klog.SetLogger(kitexlog.NewKitexAdapter(log.L()))

	// 3. 初始化 Hertz 可观测性（OTel Tracing + Metrics）
	hertzObs, err := observability.NewProvider(ctx, cfg.Observability)
	if err != nil {
		log.L().Warn("hertz observability init failed, continuing without tracing", "error", err)
	}
	defer func() {
		if hertzObs != nil {
			_ = hertzObs.Shutdown()
		}
	}()

	// 4. 初始化 Kitex 可观测性（OTel Tracing + Metrics）
	kitexObs, err := kitexobs.NewProvider(ctx, cfg.Observability)
	if err != nil {
		log.L().Warn("kitex observability init failed, continuing without tracing", "error", err)
	}
	defer func() {
		if kitexObs != nil {
			_ = kitexObs.Shutdown()
		}
	}()

	// 5. 初始化运行时依赖（session / device store, middleware clients）
	deps := initDeps(cfg)

	// 6. 创建 Hertz HTTP server
	h := createHertzServer(cfg, deps, hertzObs)

	// 7. 创建 Kitex RPC server（goroutine 中运行，Task 20 实现）
	go startKitexServer(ctx, cfg, deps, kitexObs)

	// 8. 启动 Hertz
	go h.Spin()

	// 9. 等待中断信号，优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.L().Info("shutting down servers...")
	cancel()
	h.Shutdown(context.Background())
}

// Deps 运行时依赖，后续任务填充（session store / device store / middleware clients）。
type Deps struct{}

func initDeps(_ *AppConfig) *Deps {
	return &Deps{}
}

// createHertzServer 创建 Hertz HTTP 服务。
//
// 路由和中间件在后续任务中注册；当前仅接入 OTel 链路追踪。
func createHertzServer(cfg *AppConfig, _ *Deps, provider *observability.Provider) *server.Hertz {
	opts := []hertzconfig.Option{
		server.WithHostPorts(cfg.Server.HTTPAddr),
	}
	if provider != nil {
		tracer, _ := provider.ServerTracer()
		opts = append(opts, server.WithTracer(tracer))
		h := server.New(opts...)
		h.Use(observability.TracerServerMiddleware(cfg.Observability))
		return h
	}
	return server.New(opts...)
}

// startKitexServer 占位 goroutine，Task 20 实现完整 RPC 服务。
//
// 当前仅阻塞等待 context 取消。
func startKitexServer(ctx context.Context, _ *AppConfig, _ *Deps, _ *kitexobs.Provider) {
	<-ctx.Done()
}
