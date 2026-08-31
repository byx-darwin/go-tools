package redis

import (
	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// Redis 错误码 20101-20105。
const (
	// CodeConnect Redis 连接/Ping 失败
	CodeConnect = 20101
	// CodeInstrument Redis OpenTelemetry 埋点初始化失败
	CodeInstrument = 20102
	// CodeLockAcquire 分布式锁加锁失败
	CodeLockAcquire = 20103
	// CodeLockRelease 分布式锁释放失败（未持有或已过期）
	CodeLockRelease = 20104
	// CodeLimiterEval 限流器 Lua 脚本执行失败
	CodeLimiterEval = 20105
)

// 预定义 Redis 错误构造器。
var (
	// ErrConnect Redis 连接/Ping 失败
	ErrConnect = goerror.Code(CodeConnect).Public("redis_connect_error")
	// ErrInstrument Redis OpenTelemetry 埋点初始化失败
	ErrInstrument = goerror.Code(CodeInstrument).Public("redis_instrument_error")
	// ErrLockAcquire 分布式锁加锁失败
	ErrLockAcquire = goerror.Code(CodeLockAcquire).Public("redis_lock_acquire_error")
	// ErrLockRelease 分布式锁释放失败
	ErrLockRelease = goerror.Code(CodeLockRelease).Public("redis_lock_release_error")
	// ErrLimiterEval 限流器 Lua 脚本执行失败
	ErrLimiterEval = goerror.Code(CodeLimiterEval).Public("redis_limiter_eval_error")
)

// init 注册 Redis 错误码的细粒度 HTTP 状态码映射。
func init() {
	goerror.RegisterHTTPStatuses(map[int]int{
		CodeConnect:     503,
		CodeInstrument:  500,
		CodeLockAcquire: 409,
		CodeLockRelease: 409,
		CodeLimiterEval: 500,
	})
}
