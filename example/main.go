package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	hertzconfig "github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	klog "github.com/cloudwego/kitex/pkg/klog"

	"github.com/byx-darwin/go-tools/example/handler"
	demoservice "github.com/byx-darwin/go-tools/example/kitex_generated/demo/demoservice"
	examplemw "github.com/byx-darwin/go-tools/example/middleware"
	"github.com/byx-darwin/go-tools/example/rpc"
	"github.com/byx-darwin/go-tools/go-auth/device"
	"github.com/byx-darwin/go-tools/go-auth/session"
	"github.com/byx-darwin/go-tools/go-common/log"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	hertzlog "github.com/byx-darwin/go-tools/go-framework/hertz/log"
	"github.com/byx-darwin/go-tools/go-framework/hertz/observability"
	kitexlog "github.com/byx-darwin/go-tools/go-framework/kitex/log"
	kitexobs "github.com/byx-darwin/go-tools/go-framework/kitex/observability"
	mwauth "github.com/byx-darwin/go-tools/go-middleware/auth"
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

	// 5. 初始化运行时依赖（session / device store, config handler）
	deps := initDeps(cfg)

	// 6. 创建 Hertz HTTP server
	h := createHertzServer(cfg, deps, hertzObs)

	// 7. 启动 Kitex RPC server（goroutine）
	go startKitexServer(ctx, cfg, deps, kitexObs)

	// 8. 等待 Kitex 服务启动后创建客户端（延迟创建）
	go initRPCClient(ctx, cfg, deps, kitexObs)

	// 9. 启动 Hertz
	go h.Spin()

	// 10. 等待中断信号，优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.L().Info("shutting down servers...")
	cancel()
	h.Shutdown(context.Background())
}

// Deps 运行时依赖，聚合 session store / device store / 配置 等。
type Deps struct {
	// Config 全局配置引用。
	Config *AppConfig

	// SessionStore Session 存储（内存或 Redis 实现）。
	SessionStore session.Store

	// DeviceStore Device 存储（内存或 Redis 实现）。
	DeviceStore device.Store

	// RPCClient Kitex DemoService 客户端。
	RPCClient demoservice.Client

	// KitexObs Kitex 可观测性 Provider。
	KitexObs *kitexobs.Provider
}

func initDeps(cfg *AppConfig) *Deps {
	deps := &Deps{
		Config: cfg,
	}

	// 根据 store_mode 选择内存或 Redis 实现。
	switch cfg.StoreMode {
	case "redis":
		// Redis 实现需要 Redis 客户端，此处仅做示例演示，暂用内存。
		log.L().Warn("redis store not implemented, falling back to memory")
		fallthrough
	default:
		deps.SessionStore = mwauth.NewMemorySessionStore()
		deps.DeviceStore = mwauth.NewMemoryDeviceStore()
	}

	// 注入到 handler 包（供 auth handlers 使用）。
	handler.SetSessionStore(deps.SessionStore)
	handler.SetDeviceStore(deps.DeviceStore)

	// 注入 JWT 配置。
	handler.SetJWTConfig(
		cfg.JWT.Secret,
		cfg.JWT.Issuer,
		cfg.JWT.AccessExpiration.Duration,
		cfg.JWT.RefreshExpiration.Duration,
	)

	// 注入配置 handler 所需依赖。
	handler.SetCurrentConfig(cfg)
	handler.SetConfigPath("config.yaml")
	handler.SetConfigReloadFn(func(path string) (any, error) {
		return LoadConfig(path)
	})

	return deps
}

// initRPCClient 延迟初始化 Kitex RPC 客户端（等待服务启动后连接）。
func initRPCClient(ctx context.Context, cfg *AppConfig, deps *Deps, kitexObs *kitexobs.Provider) {
	// 等待 Kitex 服务启动（简单延迟）。
	select {
	case <-ctx.Done():
		return
	default:
	}

	rpcAddr := "localhost" + cfg.Server.RPCAddr
	client, err := rpc.NewDemoClient(rpcAddr, kitexObs)
	if err != nil {
		log.L().Warn("kitex client init failed, RPC routes will return 503", "error", err)
		return
	}

	deps.RPCClient = client
	handler.SetRPCClient(client)
	log.L().Info("kitex client initialized", "addr", rpcAddr)
}

// createHertzServer 创建 Hertz HTTP 服务。
func createHertzServer(cfg *AppConfig, deps *Deps, provider *observability.Provider) *server.Hertz {
	opts := []hertzconfig.Option{
		server.WithHostPorts(cfg.Server.HTTPAddr),
	}

	var h *server.Hertz
	if provider != nil {
		tracer, _ := provider.ServerTracer()
		opts = append(opts, server.WithTracer(tracer))
		h = server.New(opts...)
		h.Use(observability.TracerServerMiddleware(cfg.Observability))
	} else {
		h = server.New(opts...)
	}

	// 注册全局中间件（AccessLog、Cors）。
	mwDeps := &examplemw.Deps{
		SessionStore: deps.SessionStore,
		DeviceStore:  deps.DeviceStore,
		JWTSecret:    []byte(cfg.JWT.Secret),
	}
	examplemw.RegisterMiddleware(h, mwDeps)

	// 注册健康检查路由（Task 21）。
	registerHealthRoutes(h)

	// 注册 go-common 示例路由。
	registerCommonRoutes(h)

	// 注册 go-auth 示例路由（JWT / Session / Device）。
	registerAuthRoutes(h)

	// 注册 go-middleware 示例路由（Redis / Kafka / DB / ES / ClickHouse）。
	registerMiddlewareRoutes(h)

	// 注册 config 示例路由（Task 19）。
	registerConfigRoutes(h)

	// 注册 RPC 示例路由（Task 20：Hertz → Kitex）。
	registerRPCRoutes(h)

	// 注册受保护的路由组。
	examplemw.RegisterProtectedRoutes(h, mwDeps)

	return h
}

// registerHealthRoutes 注册健康检查路由。
func registerHealthRoutes(h *server.Hertz) {
	h.GET("/health", func(_ context.Context, c *app.RequestContext) {
		hertzresp.Success(c, map[string]any{"status": "ok"})
	})
}

// registerCommonRoutes 注册所有 go-common 包的示例路由。
func registerCommonRoutes(h *server.Hertz) {
	handler.RegisterCryptoRoutes(h)
	handler.RegisterCacheRoutes(h)
	handler.RegisterErrorRoutes(h)
	handler.RegisterNetutilRoutes(h)
	handler.RegisterTimeutilRoutes(h)
	handler.RegisterCaptchaRoutes(h)
	handler.RegisterLogRoutes(h)
	handler.RegisterHTTPClientRoutes(h)
	handler.RegisterTemplateRoutes(h)
	handler.RegisterExecutilRoutes(h)
	handler.RegisterAstutilRoutes(h)
	handler.RegisterAkskRoutes(h)
}

// registerAuthRoutes 注册 go-auth 包的示例路由（JWT / Session / Device）。
func registerAuthRoutes(h *server.Hertz) {
	handler.RegisterJWTRoutes(h)
	handler.RegisterSessionRoutes(h)
	handler.RegisterDeviceRoutes(h)
}

// registerMiddlewareRoutes 注册 go-middleware 包的示例路由（Redis / Kafka / DB / ES / ClickHouse）。
func registerMiddlewareRoutes(h *server.Hertz) {
	handler.RegisterRedisRoutes(h)
	handler.RegisterKafkaRoutes(h)
	handler.RegisterDBRoutes(h)
	handler.RegisterESRoutes(h)
	handler.RegisterClickHouseRoutes(h)
}

// registerConfigRoutes 注册 go-framework/config 示例路由。
func registerConfigRoutes(h *server.Hertz) {
	handler.RegisterConfigRoutes(h)
}

// registerRPCRoutes 注册 Hertz → Kitex RPC 示例路由。
func registerRPCRoutes(h *server.Hertz) {
	handler.RegisterRPCRoutes(h)
}

// startKitexServer 启动 Kitex RPC 服务。
func startKitexServer(ctx context.Context, cfg *AppConfig, _ *Deps, kitexObs *kitexobs.Provider) {
	if err := rpc.StartServer(ctx, cfg.Server.RPCAddr, kitexObs); err != nil {
		log.L().Error("kitex server error", "error", err)
	}
}
