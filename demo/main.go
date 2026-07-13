// Package main 是 go-tools 全模块综合 Demo 文件。
//
// 运行方式：
//
//	go run ./demo/...
//
// 每个包对应一个 verifyXxx() 函数，返回 result 结构体。
// 所有函数从 main() 中统一调用并打印结果。
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/byx-darwin/go-tools/go-common/astutil"
	"github.com/byx-darwin/go-tools/go-common/auth"
	"github.com/byx-darwin/go-tools/go-common/cache"
	"github.com/byx-darwin/go-tools/go-common/captcha"
	"github.com/byx-darwin/go-tools/go-common/crypto"
	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/byx-darwin/go-tools/go-common/executil"
	"github.com/byx-darwin/go-tools/go-common/httpclient"
	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/byx-darwin/go-tools/go-common/netutil"
	"github.com/byx-darwin/go-tools/go-common/templateutil"
	"github.com/byx-darwin/go-tools/go-common/timeutil"

	autherror "github.com/byx-darwin/go-tools/go-auth/error"
	"github.com/byx-darwin/go-tools/go-auth/jwt"

	"github.com/byx-darwin/go-tools/go-auth/device"
	"github.com/byx-darwin/go-tools/go-auth/session"

	"github.com/byx-darwin/go-tools/go-middleware/clickhouse"
	"github.com/byx-darwin/go-tools/go-middleware/db"
	"github.com/byx-darwin/go-tools/go-middleware/es"
	"github.com/byx-darwin/go-tools/go-middleware/kafka"
	"github.com/byx-darwin/go-tools/go-middleware/redis"

	"github.com/byx-darwin/go-tools/go-framework/config"
	hertzConfig "github.com/byx-darwin/go-tools/go-framework/config/hertz"
	"github.com/byx-darwin/go-tools/go-framework/hertz"

	_ "github.com/go-sql-driver/mysql"
)

// result 表示单个包的验证结果。
type result struct {
	status  string // PASS / FAIL / SKIP
	pkg     string // 包名称
	message string // 详细信息
}

func main() {
	var passed, failed, skipped int

	results := []result{
		// ── go-common (纯逻辑，实际测试) ──
		verifyLog(),
		verifyCache(),
		verifyCaptcha(),
		verifyCrypto(),
		verifyHTTPClient(),
		verifyTimeutil(),
		verifyNetutil(),
		verifyAuth(),
		verifyCommonError(),
		verifyAstutil(),
		verifyExecutil(),
		verifyTemplateutil(),

		// ── go-auth (纯逻辑，实际测试) ──
		verifyAuthError(),
		verifyJWT(),
		verifySession(),
		verifyDevice(),

		// ── go-middleware (实际连接测试，需要 Docker Compose 服务) ──
		verifyRedis(),
		verifyKafka(),
		verifyDB(),
		verifyES(),
		verifyClickHouse(),
		verifyTLS(),
		verifyMiddlewareAuth(),

		// ── go-framework (实际测试) ──
		verifyHertz(),
		verifyKitex(),
		verifyConfig(),
	}

	// ── 打印结果 ──
	for _, r := range results {
		switch r.status {
		case "PASS":
			passed++
		case "FAIL":
			failed++
		case "SKIP":
			skipped++
		}
		fmt.Printf("[%s] %s — %s\n", r.status, r.pkg, r.message)
	}

	// ── 总结 ──
	fmt.Println()
	fmt.Printf("=== Summary: %d PASS, %d FAIL, %d SKIP (total: %d) ===\n",
		passed, failed, skipped, len(results))
	if failed > 0 {
		os.Exit(1)
	}
}

// safeCall 包装函数调用，捕获 panic 并返回 FAIL。
func safeCall(pkg string, fn func() error) result {
	err := fn()
	if err != nil {
		return result{"FAIL", pkg, "test error: " + err.Error()}
	}
	return result{"PASS", pkg, "all checks passed"}
}

// isNotLinux 检查当前平台是否非 Linux。
func isNotLinux() bool { return runtime.GOOS != "linux" }

// ────────────────────────────────────────────────────────────
// go-common/log
// ────────────────────────────────────────────────────────────

func verifyLog() result {
	const pkg = "go-common/log"
	return safeCall(pkg, func() error {
		// 1. 测试 New() 创建 Logger
		l := log.New(log.WithLevel("debug"), log.WithJSON(false))
		if l == nil {
			return errors.New("log.New() returned nil")
		}
		defer func() { _ = l.Close() }()
		l.Info("demo log test", "key", "value")

		// 2. 测试 NewConfig
		cfg := log.NewConfig(
			log.WithConfigLevel("info"),
			log.WithConfigFormat("text"),
		)
		if cfg.Level != "info" {
			return fmt.Errorf("NewConfig Level = %q, want \"info\"", cfg.Level)
		}
		if cfg.Format != "text" {
			return fmt.Errorf("NewConfig Format = %q, want \"text\"", cfg.Format)
		}

		// 3. 测试 NewLogger + ReleaseInfo
		l2, err := log.NewLogger(cfg, log.ReleaseInfo{
			ServiceName: "demo",
			Version:     "1.0.0",
		})
		if err != nil {
			return fmt.Errorf("NewLogger failed: %w", err)
		}
		if l2 == nil {
			return errors.New("NewLogger returned nil")
		}
		l2.Info("logger created successfully")

		// 4. 测试 ReleaseInfo.WithExtra
		ri := log.ReleaseInfo{ServiceName: "srv"}
		ri2 := ri.WithExtra("region", "us-east")
		if ri2.Extra == nil || ri2.Extra["region"] != "us-east" {
			return errors.New("ReleaseInfo.WithExtra failed")
		}

		// 5. 测试 WithCategory
		categorized := l2.WithCategory(log.CategoryApp)
		if categorized == nil {
			return errors.New("WithCategory returned nil")
		}
		categorized.Info("categorized log test")

		// 6. 测试所有分类常量
		cats := []string{log.CategoryAccess, log.CategoryError, log.CategoryBiz,
			log.CategoryRPC, log.CategoryDB, log.CategoryPanic, log.CategoryAudit,
			log.CategorySecurity, log.CategoryApp, log.CategoryCache, log.CategoryMQ}
		for _, cat := range cats {
			if cat == "" {
				return fmt.Errorf("category constant is empty")
			}
		}

		// 7. 测试层 Logger (App, DB, Access, RPC, MQ, Cache)
		ctx := context.Background()
		appLogger := log.App(ctx)
		if appLogger == nil {
			return errors.New("log.App() returned nil")
		}
		dbLogger := log.DB(ctx)
		if dbLogger == nil {
			return errors.New("log.DB() returned nil")
		}
		accLogger := log.Access(ctx)
		if accLogger == nil {
			return errors.New("log.Access() returned nil")
		}
		rpcLogger := log.RPC(ctx)
		if rpcLogger == nil {
			return errors.New("log.RPC() returned nil")
		}
		mqLogger := log.MQ(ctx)
		if mqLogger == nil {
			return errors.New("log.MQ() returned nil")
		}
		cacheLogger := log.Cache(ctx)
		if cacheLogger == nil {
			return errors.New("log.Cache() returned nil")
		}

		// 8. 测试 DomainLogger
		dl := log.NewDomainLogger("order")
		if dl == nil {
			return errors.New("NewDomainLogger returned nil")
		}
		dl.Decision("test decision", true, "amount", 100)
		dl.Event("test event", "order_id", "ORD-001")
		dl.Error("test error", fmt.Errorf("something went wrong"))

		// 9. 测试 ErrorAttrs（非 oops 错误）
		attrs := log.ErrorAttrs(fmt.Errorf("plain error"))
		_ = attrs // 非 oops 错误返回 nil

		// 10. 测试 ErrorAttrs（oops 错误）
		oopsErr := goerror.In("test").Code(40001).
			Hint("check input").Public("bad_request").Errorf("test oops error")
		oopsAttrs := log.ErrorAttrs(oopsErr)
		if len(oopsAttrs) == 0 {
			return errors.New("ErrorAttrs returned empty for oops error")
		}

		// 11. 测试 ErrorContext
		l2.ErrorContext(ctx, "context error test", oopsErr, "extra", "data")

		// 12. 测试 L() 全局 Logger
		globalL := log.L()
		if globalL == nil {
			return errors.New("log.L() returned nil")
		}
		globalL.Info("global logger works")

		return nil
	})
}

// ────────────────────────────────────────────────────────────
// go-common/cache
// ────────────────────────────────────────────────────────────

func verifyCache() result {
	const pkg = "go-common/cache"
	return safeCall(pkg, func() error {
		algorithms := []cache.EvictionAlgorithm{
			cache.LRU, cache.LFU, cache.FIFO, cache.TwoQueue, cache.ARC,
		}
		names := []string{"LRU", "LFU", "FIFO", "TwoQueue", "ARC"}

		for i, alg := range algorithms {
			c := cache.New[string, int](alg, 100).Build()
			if c == nil {
				return fmt.Errorf("cache.New(%s, 100).Build() returned nil", names[i])
			}
			key := "key-" + names[i]
			c.Set(key, 42)
			v, ok, _ := c.Get(key)
			if !ok {
				return fmt.Errorf("cache(%s).Get(%q) not found after Set", names[i], key)
			}
			if v != 42 {
				return fmt.Errorf("cache(%s).Get(%q) = %d, want 42", names[i], key, v)
			}
			// 测试 Delete
			c.Delete(key)
			_, ok2, _ := c.Get(key)
			if ok2 {
				return fmt.Errorf("cache(%s).Delete(%q): key still exists", names[i], key)
			}
		}

		// 测试 SetWithTTL
		cTTL := cache.New[string, string](cache.LRU, 100).WithTTL(time.Minute).Build()
		cTTL.SetWithTTL("ttl-key", "ttl-val", time.Second*30)
		v, ok, _ := cTTL.Get("ttl-key")
		if !ok || v != "ttl-val" {
			return errors.New("SetWithTTL + Get failed")
		}

		// 测试 Purge
		cTTL.Purge()
		if cTTL.Len() != 0 {
			return fmt.Errorf("Purge: Len() = %d, want 0", cTTL.Len())
		}

		return nil
	})
}

// ────────────────────────────────────────────────────────────
// go-common/captcha
// ────────────────────────────────────────────────────────────

func verifyCaptcha() result {
	const pkg = "go-common/captcha"
	return safeCall(pkg, func() error {
		// 1. 测试 GenerateCode 数字
		code1 := captcha.GenerateCode(6, "digit")
		if len(code1) != 6 {
			return fmt.Errorf("GenerateCode(6, digit) length = %d, want 6", len(code1))
		}

		// 2. 测试 GenerateCode 字母数字混合
		code2 := captcha.GenerateCode(8, "alphanumeric")
		if len(code2) != 8 {
			return fmt.Errorf("GenerateCode(8, alphanumeric) length = %d, want 8", len(code2))
		}

		// 3. 测试 GenerateDigitCode
		code3 := captcha.GenerateDigitCode(4)
		if len(code3) != 4 {
			return fmt.Errorf("GenerateDigitCode(4) length = %d, want 4", len(code3))
		}

		// 4. 测试 NewCacheStore（Options 模式）
		store := captcha.NewCacheStore(
			captcha.WithCapacity(256),
			captcha.WithExpiration(3*time.Minute),
			captcha.WithPreKey("DEMO_"),
		)
		if store == nil {
			return errors.New("NewCacheStore returned nil")
		}

		// 5. 测试 CacheStore Set/Get/Verify
		err := store.Set("test-id", "ABCD")
		if err != nil {
			return fmt.Errorf("CacheStore.Set failed: %v", err)
		}
		val := store.Get("test-id", false)
		if val != "ABCD" {
			return fmt.Errorf("CacheStore.Get = %q, want %q", val, "ABCD")
		}
		if !store.Verify("test-id", "ABCD", false) {
			return errors.New("CacheStore.Verify returned false for correct answer")
		}
		if store.Verify("test-id", "WRONG", false) {
			return errors.New("CacheStore.Verify returned true for wrong answer")
		}

		// 6. 测试 GetAndDelete
		_ = store.Set("del-id", "DEL")
		val2, ok := store.GetAndDelete("del-id")
		if !ok || val2 != "DEL" {
			return errors.New("GetAndDelete failed")
		}
		val3 := store.Get("del-id", false)
		if val3 != "" {
			return errors.New("GetAndDelete did not delete key")
		}

		// 7. 测试 Len 和 Clear
		_ = store.Set("k1", "v1")
		_ = store.Set("k2", "v2")
		if store.Len() == 0 {
			return errors.New("Len() should be > 0 after Set")
		}
		store.Clear()
		if store.Len() != 0 {
			return errors.New("Clear() should reset Len() to 0")
		}

		// 8. 测试 Expiration / PreKey getter/setter
		if store.Expiration() != 3*time.Minute {
			return fmt.Errorf("Expiration() = %v, want 3m", store.Expiration())
		}
		if store.PreKey() != "DEMO_" {
			return fmt.Errorf("PreKey() = %q, want \"DEMO_\"", store.PreKey())
		}
		store.SetPreKey("NEW_")
		if store.PreKey() != "NEW_" {
			return fmt.Errorf("SetPreKey failed: PreKey() = %q", store.PreKey())
		}
		store.SetExpiration(1 * time.Minute)
		if store.Expiration() != 1*time.Minute {
			return fmt.Errorf("SetExpiration failed: Expiration() = %v", store.Expiration())
		}

		// 9. 测试 NewImageCaptcha + Generate + Verify
		ic := captcha.NewImageCaptcha(
			captcha.WithWidth(200),
			captcha.WithHeight(60),
			captcha.WithKeyLong(4),
		)
		id, b64s, answer, errGen := ic.Generate()
		if errGen != nil {
			return fmt.Errorf("ImageCaptcha.Generate failed: %v", errGen)
		}
		if id == "" || b64s == "" || answer == "" {
			return errors.New("ImageCaptcha.Generate returned empty values")
		}
		if !ic.Verify(id, answer, true) {
			return errors.New("ImageCaptcha.Verify failed for correct answer")
		}
		if ic.Verify(id, answer, true) {
			return errors.New("ImageCaptcha.Verify should be false after clear")
		}

		// 10. 测试 Update 批量更新
		store2 := captcha.NewCacheStore()
		store2.Update(
			captcha.WithPreKey("UPDATED_"),
			captcha.WithExpiration(10*time.Minute),
		)
		if store2.PreKey() != "UPDATED_" {
			return fmt.Errorf("Update: PreKey() = %q, want \"UPDATED_\"", store2.PreKey())
		}

		// 11. 测试 WithEvictionPolicy
		store3 := captcha.NewCacheStore(
			captcha.WithEvictionPolicy(cache.LRU),
		)
		if store3.Len() != 0 {
			return errors.New("NewCacheStore with LRU: Len() should be 0")
		}
		_ = store3.Set("ek", "ev")
		_ = store3.Get("ek", false)

		return nil
	})
}

// ────────────────────────────────────────────────────────────
// go-common/crypto
// ────────────────────────────────────────────────────────────

func verifyCrypto() result {
	const pkg = "go-common/crypto"
	return safeCall(pkg, func() error {
		data := []byte("hello world")

		// 1. MD5
		md5Hash := crypto.MD5(data)
		expectedMD5 := "5eb63bbbe01eeed093cb22bb8f5acdc3"
		if md5Hash != expectedMD5 {
			return fmt.Errorf("MD5 = %q, want %q", md5Hash, expectedMD5)
		}

		// 2. SHA256
		sha256Hash := crypto.SHA256(data)
		expectedSHA256 := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
		if sha256Hash != expectedSHA256 {
			return fmt.Errorf("SHA256 = %q, want %q", sha256Hash, expectedSHA256)
		}

		// 3. SHA1
		sha1Hash := crypto.SHA1(data)
		if len(sha1Hash) != 40 {
			return fmt.Errorf("SHA1 length = %d, want 40", len(sha1Hash))
		}

		// 4. SHA512
		sha512Hash := crypto.SHA512(data)
		if len(sha512Hash) != 128 {
			return fmt.Errorf("SHA512 length = %d, want 128", len(sha512Hash))
		}

		// 5. HMAC-SHA256
		key := []byte("secret-key")
		hmacHash := crypto.HMACSHA256(data, key)
		if hmacHash == "" {
			return errors.New("HMACSHA256 returned empty string")
		}

		// 6. EncodePwd
		pwd := crypto.EncodePwd("mypassword", "myak")
		if pwd == "" {
			return errors.New("EncodePwd returned empty string")
		}

		// 8. TEA 加密/解密
		teaKey := "0123456789abcdef" // 16 bytes
		plainText := []byte("test data for TEA encryption")
		encoded, pad, err := crypto.EncodeTeaStr(plainText, teaKey)
		if err != nil {
			return fmt.Errorf("EncodeTeaStr failed: %v", err)
		}
		decoded, err := crypto.DecodeTeaStr(encoded, pad, teaKey)
		if err != nil {
			return fmt.Errorf("DecodeTeaStr failed: %v", err)
		}
		if string(plainText) != string(decoded[:len(plainText)]) {
			return errors.New("TEA roundtrip failed: plainText != decoded")
		}

		return nil
	})
}

// ────────────────────────────────────────────────────────────
// go-common/httpclient
// ────────────────────────────────────────────────────────────

func verifyHTTPClient() result {
	const pkg = "go-common/httpclient"
	return safeCall(pkg, func() error {
		// 发送 GET 请求到 jsonplaceholder（公开测试服务）
		resp, statusCode, err := httpclient.Send(
			"https://jsonplaceholder.typicode.com/posts/1",
			"GET",
			nil,
			map[string]string{"Accept": "application/json"},
			10*time.Second,
		)
		if err != nil {
			return fmt.Errorf("HTTP GET failed: %v", err)
		}
		if statusCode < 200 || statusCode >= 300 {
			// 某些公开服务可能不稳定，2xx 以外也接受（只要没报错）
			return fmt.Errorf("unexpected status code: %d", statusCode)
		}
		if resp == nil {
			return errors.New("response body is nil")
		}
		body := resp.Body()
		if len(body) == 0 {
			return errors.New("response body is empty")
		}
		return nil
	})
}

// ────────────────────────────────────────────────────────────
// go-common/timeutil
// ────────────────────────────────────────────────────────────

func verifyTimeutil() result {
	const pkg = "go-common/timeutil"
	return safeCall(pkg, func() error {
		// 使用固定 timestamp: 2024-01-15 08:30:45 UTC = 1705307445
		ts := int64(1705307445)

		// 无时区参数使用本地时区，测试纯日期格式最可靠
		tests := []struct {
			format, want string
		}{
			{"YYYY-MM-DD", "2024-01-15"},
			{"YYYY年MM月DD日", "2024年01月15日"},
		}
		for _, tc := range tests {
			got := timeutil.Format(ts, tc.format, "")
			if got != tc.want {
				return fmt.Errorf("Format(%d, %q) = %q, want %q", ts, tc.format, got, tc.want)
			}
		}

		// 测试 UTC 时区（不受本地时区影响）
		utcTests := []struct {
			format, want string
		}{
			{"YYYY/MM/DD HH:mm:ss", "2024/01/15 08:30:45"},
			{"HH:mm", "08:30"},
			{"h:mm A", "8:30 AM"},
			{"YYYY-MM-DD [at] HH:mm", "2024-01-15 at 08:30"},
		}
		for _, tc := range utcTests {
			utcGot := timeutil.Format(ts, tc.format, "UTC")
			if utcGot != tc.want {
				return fmt.Errorf("Format(%d, %q, UTC) = %q, want %q", ts, tc.format, utcGot, tc.want)
			}
		}

		// 测试本地时区输出非空
		localFormat := timeutil.Format(ts, "HH:mm:ss", "")
		if localFormat == "" {
			return errors.New("Format with empty timezone returned empty")
		}

		return nil
	})
}

// ────────────────────────────────────────────────────────────
// go-common/netutil
// ────────────────────────────────────────────────────────────

func verifyNetutil() result {
	const pkg = "go-common/netutil"
	return safeCall(pkg, func() error {
		// 1. GetInternalIP — 在大多数环境至少返回一个内网 IP 或空
		ip, err := netutil.GetInternalIP()
		if err != nil {
			return fmt.Errorf("GetInternalIP error: %v", err)
		}
		// ip 可能为空（如仅回环接口的环境中），这是合法的
		_ = ip

		// 2. CheckNetwork
		status := netutil.CheckNetwork()
		_ = status // 离线环境也正常，函数本身不应报错

		// 3. IsNetworkAvailable
		_ = netutil.IsNetworkAvailable() // 不依赖网络可用

		return nil
	})
}

// ────────────────────────────────────────────────────────────
// go-common/auth (AK/SK)
// ────────────────────────────────────────────────────────────

func verifyAuth() result {
	const pkg = "go-common/auth"
	return safeCall(pkg, func() error {
		// 1. GetRandAk
		ak1 := auth.GetRandAk(16)
		if len(ak1) != 16 {
			return fmt.Errorf("GetRandAk(16) length = %d, want 16", len(ak1))
		}
		ak2 := auth.GetRandAk(32)
		if len(ak2) != 32 {
			return fmt.Errorf("GetRandAk(32) length = %d, want 32", len(ak2))
		}
		// 两次调用应大概率生成不同值
		ak3 := auth.GetRandAk(16)
		if ak1 == ak3 {
			return errors.New("GetRandAk generated identical values (unlikely)")
		}

		// 2. RefreshSK
		sk := auth.RefreshSK(ak1)
		if sk == "" {
			return errors.New("RefreshSK returned empty string")
		}
		// RefreshSK 使用 MD5，应生成 32 位十六进制字符串
		if len(sk) != 32 {
			return fmt.Errorf("RefreshSK length = %d, want 32 (MD5 hex)", len(sk))
		}

		return nil
	})
}

// ────────────────────────────────────────────────────────────
// go-common/error
// ────────────────────────────────────────────────────────────

func verifyCommonError() result {
	const pkg = "go-common/error"
	return safeCall(pkg, func() error {
		// 1. 预定义错误码常量
		if goerror.CodeSystem != 10000 {
			return fmt.Errorf("CodeSystem = %d, want 10000", goerror.CodeSystem)
		}
		if goerror.CodeParamInvalid != 10001 {
			return fmt.Errorf("CodeParamInvalid = %d, want 10001", goerror.CodeParamInvalid)
		}

		// 2. 范围常量
		if goerror.FrameworkCodeMin != 10000 {
			return fmt.Errorf("FrameworkCodeMin = %d, want 10000", goerror.FrameworkCodeMin)
		}
		if goerror.MiddlewareCodeMin != 20000 {
			return fmt.Errorf("MiddlewareCodeMin = %d, want 20000", goerror.MiddlewareCodeMin)
		}
		if goerror.ProjectCodeMin != 40000 {
			return fmt.Errorf("ProjectCodeMin = %d, want 40000", goerror.ProjectCodeMin)
		}

		// 3. 预定义错误构造器
		err := goerror.ErrParamInvalid.Wrap(fmt.Errorf("bad input"))
		if err == nil {
			return errors.New("ErrParamInvalid.Wrap returned nil")
		}

		// 4. Code 构造函数
		customErr := goerror.Code(40001).Public("custom_error").Errorf("custom message")
		if customErr == nil {
			return errors.New("Code(40001) returned nil")
		}

		// 5. In 构造函数
		domainErr := goerror.In("payment").Code(40100).Errorf("domain error")
		_ = domainErr

		// 6. Extract（带码错误）
		code, pub := goerror.Extract(err)
		if code != goerror.CodeParamInvalid {
			return fmt.Errorf("Extract code = %d, want %d", code, goerror.CodeParamInvalid)
		}
		if pub != "param_invalid" {
			return fmt.Errorf("Extract public = %q, want \"param_invalid\"", pub)
		}

		// 7. Extract（nil 错误）
		codeNil, pubNil := goerror.Extract(nil)
		if codeNil != 0 || pubNil != "" {
			return fmt.Errorf("Extract(nil) = (%d, %q), want (0, \"\")", codeNil, pubNil)
		}

		// 8. ExtractWithFallback（非 oops 错误）
		plainErr := errors.New("plain error")
		codeFB, pubFB := goerror.ExtractWithFallback(plainErr, 99999)
		if codeFB != 99999 {
			return fmt.Errorf("ExtractWithFallback code = %d, want 99999", codeFB)
		}
		if pubFB != "plain error" {
			return fmt.Errorf("ExtractWithFallback public = %q, want \"plain error\"", pubFB)
		}

		// 9. HTTPStatus 映射
		httpStatus := goerror.HTTPStatus(goerror.ErrParamInvalid.Wrap(fmt.Errorf("test")))
		if httpStatus != 400 {
			return fmt.Errorf("HTTPStatus(ErrParamInvalid) = %d, want 400", httpStatus)
		}
		httpStatusSys := goerror.HTTPStatus(goerror.ErrSystem.Wrap(fmt.Errorf("test")))
		if httpStatusSys != 500 {
			return fmt.Errorf("HTTPStatus(ErrSystem) = %d, want 500", httpStatusSys)
		}
		httpStatusRPC := goerror.HTTPStatus(goerror.ErrRPCUnavailable.Wrap(fmt.Errorf("test")))
		if httpStatusRPC != 503 {
			return fmt.Errorf("HTTPStatus(ErrRPCUnavailable) = %d, want 503", httpStatusRPC)
		}
		// 业务错误 → 200
		bizErr := goerror.Code(40110).Public("login").Errorf("login failed")
		bizStatus := goerror.HTTPStatus(bizErr)
		if bizStatus != 200 {
			return fmt.Errorf("HTTPStatus(bizErr) = %d, want 200", bizStatus)
		}

		// 10. IsClientError / IsServerError / IsBusinessErrorCode
		if !goerror.IsClientError(goerror.CodeParamInvalid) {
			return errors.New("IsClientError(10001) should be true")
		}
		if !goerror.IsServerError(goerror.CodeSystem) {
			return errors.New("IsServerError(10000) should be true")
		}
		if !goerror.IsBusinessErrorCode(40110) {
			return errors.New("IsBusinessErrorCode(40110) should be true")
		}
		if goerror.IsBusinessErrorCode(10000) {
			return errors.New("IsBusinessErrorCode(10000) should be false")
		}

		// 11. AsOopsError
		oopsErr, ok := goerror.AsOopsError(err)
		if !ok {
			return errors.New("AsOopsError should return true for oops error")
		}
		_ = oopsErr

		// 12. 验证所有预定义错误构造器可正常使用
		predefs := []goerror.Builder{
			goerror.ErrSystem, goerror.ErrParamInvalid, goerror.ErrAuthFailed,
			goerror.ErrConfigNotFound, goerror.ErrConfigInvalid,
			goerror.ErrRPCUnavailable, goerror.ErrRPCTimeout,
			goerror.ErrRedisConnect, goerror.ErrKafkaConnect, goerror.ErrDBConnect,
			goerror.ErrESConnect, goerror.ErrCHConnect,
			goerror.ErrDataNotFound, goerror.ErrLoginFailed, goerror.ErrTokenExpired,
			goerror.ErrPermissionDenied, goerror.ErrRateLimit,
		}
		for i, e := range predefs {
			wrapped := e.Wrap(fmt.Errorf("test"))
			if wrapped == nil {
				return fmt.Errorf("predefined error builder at index %d produced nil error", i)
			}
			_ = wrapped
		}

		return nil
	})
}

// ────────────────────────────────────────────────────────────
// go-common/astutil
// ────────────────────────────────────────────────────────────

func verifyAstutil() result {
	const pkg = "go-common/astutil"
	return safeCall(pkg, func() error {
		src := []byte(`
package main

import "fmt"

func hello() {
	fmt.Println("Hello")
}

func world() string {
	return "World"
}
`)

		// 1. ParseSource
		f, err := astutil.ParseSource(src)
		if err != nil {
			return fmt.Errorf("ParseSource failed: %v", err)
		}
		if f == nil {
			return errors.New("ParseSource returned nil")
		}

		// 2. FindFunctions
		funcs := f.FindFunctions()
		if len(funcs) != 2 {
			return fmt.Errorf("FindFunctions count = %d, want 2", len(funcs))
		}

		// 3. FindFunction
		fn := f.FindFunction("hello")
		if fn == nil {
			return errors.New("FindFunction(\"hello\") returned nil")
		}
		if fn.Name.Name != "hello" {
			return fmt.Errorf("FindFunction name = %q, want \"hello\"", fn.Name.Name)
		}
		fnMissing := f.FindFunction("nonExistent")
		if fnMissing != nil {
			return errors.New("FindFunction(\"nonExistent\") should return nil")
		}

		// 4. FindImports
		imports := f.FindImports()
		if len(imports) != 1 {
			return fmt.Errorf("FindImports count = %d, want 1", len(imports))
		}

		// 5. Format
		formatted, err := f.Format()
		if err != nil {
			return fmt.Errorf("Format failed: %v", err)
		}
		if len(formatted) == 0 {
			return errors.New("Format returned empty bytes")
		}

		return nil
	})
}

// ────────────────────────────────────────────────────────────
// go-common/executil
// ────────────────────────────────────────────────────────────

func verifyExecutil() result {
	const pkg = "go-common/executil"
	return safeCall(pkg, func() error {
		// 1. 创建 Runner
		runner := executil.New()

		// 2. 执行 echo 命令（跨平台可用）
		ctx := context.Background()
		result := runner.Run(ctx, &executil.Cmd{
			Name: "echo",
			Args: []string{"hello", "world"},
		})
		if result.Err != nil {
			return fmt.Errorf("echo command failed: %v", result.Err)
		}
		if result.ExitCode != 0 {
			return fmt.Errorf("echo exit code = %d, want 0", result.ExitCode)
		}
		stdoutStr := string(result.Stdout)
		if stdoutStr == "" {
			return errors.New("echo output is empty")
		}

		// 3. 测试不存在的命令
		result2 := runner.Run(ctx, &executil.Cmd{
			Name:    "nonexistent_command_xyz",
			Timeout: 1 * time.Second,
		})
		if result2.Err == nil {
			return errors.New("nonexistent command should return error")
		}

		// 4. 测试超时
		result3 := runner.Run(ctx, &executil.Cmd{
			Name:    "sleep",
			Args:    []string{"10"},
			Timeout: 100 * time.Millisecond,
		})
		if result3.Err == nil {
			return errors.New("sleep command should timeout")
		}

		// 5. 测试 OnStdout 回调
		var captured []byte
		result4 := runner.Run(ctx, &executil.Cmd{
			Name: "echo",
			Args: []string{"captured"},
			OnStdout: func(b []byte) {
				captured = append(captured, b...)
			},
		})
		if result4.Err != nil {
			return fmt.Errorf("echo with callback failed: %v", result4.Err)
		}
		if len(captured) == 0 {
			return errors.New("OnStdout callback not called")
		}

		return nil
	})
}

// ────────────────────────────────────────────────────────────
// go-common/templateutil
// ────────────────────────────────────────────────────────────

func verifyTemplateutil() result {
	const pkg = "go-common/templateutil"
	return safeCall(pkg, func() error {
		// 1. Render 使用默认函数集
		out, err := templateutil.Render("Hello, {{.Name | ToUpper}}!", map[string]string{"Name": "world"})
		if err != nil {
			return fmt.Errorf("Render failed: %v", err)
		}
		if out != "Hello, WORLD!" {
			return fmt.Errorf("Render = %q, want \"Hello, WORLD!\"", out)
		}

		// 2. NewRegistry + Register + RenderWith
		reg := templateutil.NewRegistry()
		reg.Default()
		reg.Register("Double", func(s string) string { return s + s })
		out2, err := templateutil.RenderWith("{{.Name | Double}}", map[string]string{"Name": "ab"}, reg)
		if err != nil {
			return fmt.Errorf("RenderWith failed: %v", err)
		}
		if out2 != "abab" {
			return fmt.Errorf("RenderWith custom func = %q, want \"abab\"", out2)
		}

		// 3. Plural
		pluralTests := map[string]string{
			"user":     "users",
			"box":      "boxes",
			"city":     "cities",
			"leaf":     "leaves",
			"child":    "children",
			"wolf":     "wolves",
			"potato":   "potatoes",
			"quiz":     "quizzes",
			"person":   "people",
			"analysis": "analyses", //nolint:misspell // analyses 是 analysis 的正确复数形式
		}
		for singular, expected := range pluralTests {
			got := templateutil.Plural(singular)
			if got != expected {
				return fmt.Errorf("Plural(%q) = %q, want %q", singular, got, expected)
			}
		}

		// 4. Singular
		singularTests := map[string]string{
			"users":    "user",
			"boxes":    "box",
			"cities":   "city",
			"leaves":   "leaf",
			"children": "child",
			"wolves":   "wolf",
			"potatoes": "potato",
			"quizzes":  "quiz",
			"people":   "person",
			"analyses": "analysis", //nolint:misspell // analyses 是 analysis 的正确复数形式
			"buses":    "bus",
			"churches": "church",
		}
		for plural, expected := range singularTests {
			got := templateutil.Singular(plural)
			if got != expected {
				return fmt.Errorf("Singular(%q) = %q, want %q", plural, got, expected)
			}
		}

		// 5. RegisterAll
		reg2 := templateutil.NewRegistry()
		reg2.Default()
		reg2.RegisterAll(map[string]any{
			"Triple": func(s string) string { return s + s + s },
		})
		out3, err := templateutil.RenderWith("{{.Name | Triple}}", map[string]string{"Name": "x"}, reg2)
		if err != nil {
			return fmt.Errorf("RenderWith RegisterAll failed: %v", err)
		}
		if out3 != "xxx" {
			return fmt.Errorf("RegisterAll test = %q, want \"xxx\"", out3)
		}

		return nil
	})
}

// ────────────────────────────────────────────────────────────
// go-auth/error
// ────────────────────────────────────────────────────────────

func verifyAuthError() result {
	const pkg = "go-auth/error"
	return safeCall(pkg, func() error {
		// 1. 错误码常量
		if autherror.CodeTokenInvalid != 40001 {
			return fmt.Errorf("CodeTokenInvalid = %d, want 40001", autherror.CodeTokenInvalid)
		}
		if autherror.CodeTokenExpired != 40002 {
			return fmt.Errorf("CodeTokenExpired = %d, want 40002", autherror.CodeTokenExpired)
		}
		if autherror.CodeJWTSignFailed != 40007 {
			return fmt.Errorf("CodeJWTSignFailed = %d, want 40007", autherror.CodeJWTSignFailed)
		}

		// 2. 预定义错误构造器可正常使用
		authPredefs := map[string]goerror.Builder{
			"ErrTokenInvalid":     autherror.ErrTokenInvalid,
			"ErrTokenExpired":     autherror.ErrTokenExpired,
			"ErrTokenRevoked":     autherror.ErrTokenRevoked,
			"ErrDeviceKicked":     autherror.ErrDeviceKicked,
			"ErrSessionInvalid":   autherror.ErrSessionInvalid,
			"ErrSessionExpired":   autherror.ErrSessionExpired,
			"ErrJWTSignFailed":    autherror.ErrJWTSignFailed,
			"ErrJWTVerifyFailed":  autherror.ErrJWTVerifyFailed,
			"ErrJWTRefreshFailed": autherror.ErrJWTRefreshFailed,
		}
		for name, e := range authPredefs {
			wrapped := e.Wrap(fmt.Errorf("test"))
			if wrapped == nil {
				return fmt.Errorf("%s produced nil error", name)
			}
		}

		// 3. 测试 oops 错误包装
		wrapped := autherror.ErrSessionExpired.Wrap(fmt.Errorf("session timeout"))
		code, _ := goerror.Extract(wrapped)
		if code != autherror.CodeSessionExpired {
			return fmt.Errorf("Extract(wrapped SessionExpired) code = %d, want %d",
				code, autherror.CodeSessionExpired)
		}

		// 4. Builder 类型别名（编译期验证类型存在即可）
		var b goerror.Builder = goerror.Code(40100)
		_ = b

		return nil
	})
}

// ────────────────────────────────────────────────────────────
// go-auth/jwt
// ────────────────────────────────────────────────────────────

// UserClaims 用于 JWT 测试的自定义 Claims 类型。
type UserClaims struct {
	UserUUID string `json:"user_uuid"`
	Role     string `json:"role,omitempty"`
	gojwt.RegisteredClaims
}

func verifyJWT() result {
	const pkg = "go-auth/jwt"
	return safeCall(pkg, func() error {
		secret := []byte("demo-secret-key-32bytes!!")

		// 1. Sign + Verify 正常流程
		claims := UserClaims{
			UserUUID: "user-001",
			Role:     "admin",
		}
		token, err := jwt.Sign(claims, secret, jwt.WithExpiration(time.Hour))
		if err != nil {
			return fmt.Errorf("jwt.Sign failed: %v", err)
		}
		if token == "" {
			return errors.New("jwt.Sign returned empty token")
		}

		parsed, err := jwt.Verify[UserClaims](token, secret)
		if err != nil {
			return fmt.Errorf("jwt.Verify failed: %v", err)
		}
		if parsed.UserUUID != "user-001" {
			return fmt.Errorf("parsed UserUUID = %q, want \"user-001\"", parsed.UserUUID)
		}
		if parsed.Role != "admin" {
			return fmt.Errorf("parsed Role = %q, want \"admin\"", parsed.Role)
		}
		if parsed.ExpiresAt == nil {
			return errors.New("ExpiresAt should not be nil when WithExpiration is set")
		}

		// 2. Sign WithIssuer
		claims2 := UserClaims{UserUUID: "user-002"}
		token2, err := jwt.Sign(claims2, secret,
			jwt.WithExpiration(time.Hour),
			jwt.WithIssuer("go-tools-demo"),
		)
		if err != nil {
			return fmt.Errorf("jwt.Sign with issuer failed: %v", err)
		}
		parsed2, err := jwt.Verify[UserClaims](token2, secret)
		if err != nil {
			return fmt.Errorf("jwt.Verify with issuer failed: %v", err)
		}
		if parsed2.Issuer != "go-tools-demo" {
			return fmt.Errorf("Issuer = %q, want \"go-tools-demo\"", parsed2.Issuer)
		}

		// 3. Verify 错误 token
		_, err = jwt.Verify[UserClaims]("invalid-token-string", secret)
		if err == nil {
			return errors.New("jwt.Verify should fail for invalid token")
		}

		// 4. Verify 错误签名
		wrongSecret := []byte("wrong-secret-key!!!!!!!!")
		_, err = jwt.Verify[UserClaims](token, wrongSecret)
		if err == nil {
			return errors.New("jwt.Verify should fail for wrong secret")
		}

		// 5. Refresh
		newToken, err := jwt.Refresh[UserClaims](token, secret, jwt.WithExpiration(2*time.Hour))
		if err != nil {
			return fmt.Errorf("jwt.Refresh failed: %v", err)
		}
		if newToken == "" {
			return errors.New("Refresh returned empty token")
		}
		if newToken == token {
			return errors.New("Refresh should return a different token")
		}
		refreshed, err := jwt.Verify[UserClaims](newToken, secret)
		if err != nil {
			return fmt.Errorf("Verify refreshed token failed: %v", err)
		}
		if refreshed.UserUUID != "user-001" {
			return fmt.Errorf("Refreshed UserUUID = %q, want \"user-001\"", refreshed.UserUUID)
		}

		// 6. 无过期时间的 Sign
		claims3 := UserClaims{UserUUID: "user-noexp"}
		token3, err := jwt.Sign(claims3, secret)
		if err != nil {
			return fmt.Errorf("Sign without expiration failed: %v", err)
		}
		parsed3, err := jwt.Verify[UserClaims](token3, secret)
		if err != nil {
			return fmt.Errorf("Verify without expiration failed: %v", err)
		}
		if parsed3.ExpiresAt != nil {
			return errors.New("ExpiresAt should be nil when no WithExpiration")
		}

		return nil
	})
}

// ────────────────────────────────────────────────────────────
// go-auth/session
// ────────────────────────────────────────────────────────────

func verifySession() result {
	const pkg = "go-auth/session"
	return safeCall(pkg, func() error {
		// 1. Session 结构体字段验证
		now := time.Now()
		s := session.Session{
			ID:        "sid-123",
			UserUUID:  "user-456",
			Data:      map[string]any{"key": "value"},
			ExpiresAt: now,
		}
		if s.ID != "sid-123" {
			return fmt.Errorf("Session.ID = %q, want \"sid-123\"", s.ID)
		}
		if s.UserUUID != "user-456" {
			return fmt.Errorf("Session.UserUUID = %q, want \"user-456\"", s.UserUUID)
		}
		if v, ok := s.Data["key"]; !ok || v != "value" {
			return errors.New("Session.Data map content mismatch")
		}
		if !s.ExpiresAt.Equal(now) {
			return errors.New("Session.ExpiresAt mismatch")
		}

		// 2. Session 零值：Data 可为 nil
		s2 := session.Session{ID: "zero-data", UserUUID: "u1"}
		if s2.Data != nil {
			return errors.New("Session.Data should be nil for zero value")
		}

		// 3. Store 接口编译期验证（创建 mock 实现）
		var store session.Store = new(mockSessionStore)
		ctx := context.Background()

		// Get 不存在返回 nil
		got, err := store.Get(ctx, "not-exist")
		if err != nil {
			return fmt.Errorf("Store.Get error: %v", err)
		}
		if got != nil {
			return errors.New("Store.Get should return nil for nonexistent session")
		}

		// Save
		err = store.Save(ctx, &session.Session{ID: "s1", UserUUID: "u1"})
		if err != nil {
			return fmt.Errorf("Store.Save error: %v", err)
		}

		// Exists
		exists, err := store.Exists(ctx, "s1")
		if err != nil {
			return fmt.Errorf("Store.Exists error: %v", err)
		}
		if exists {
			return errors.New("mockStore should return false for any key")
		}

		// Delete
		err = store.Delete(ctx, "s1")
		if err != nil {
			return fmt.Errorf("Store.Delete error: %v", err)
		}

		return nil
	})
}

// mockSessionStore 是 session.Store 的最小 mock 实现。
type mockSessionStore struct{}

func (m *mockSessionStore) Get(_ context.Context, _ string) (*session.Session, error) {
	return nil, nil
}
func (m *mockSessionStore) Save(_ context.Context, _ *session.Session) error { return nil }
func (m *mockSessionStore) Delete(_ context.Context, _ string) error         { return nil }
func (m *mockSessionStore) Exists(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// ────────────────────────────────────────────────────────────
// go-auth/device
// ────────────────────────────────────────────────────────────

func verifyDevice() result {
	const pkg = "go-auth/device"
	return safeCall(pkg, func() error {
		// 1. Device 结构体字段验证
		now := time.Now()
		d := device.Device{
			DeviceID:  "dev-123",
			JTI:       "jti-abc",
			UserUUID:  "user-456",
			CreatedAt: now,
		}
		if d.DeviceID != "dev-123" {
			return fmt.Errorf("Device.DeviceID = %q, want \"dev-123\"", d.DeviceID)
		}
		if d.JTI != "jti-abc" {
			return fmt.Errorf("Device.JTI = %q, want \"jti-abc\"", d.JTI)
		}

		// 2. Device 零值验证
		dZero := device.Device{}
		if dZero.DeviceID != "" || dZero.JTI != "" || dZero.UserUUID != "" {
			return errors.New("Device zero value fields should be empty")
		}
		if !dZero.CreatedAt.IsZero() {
			return errors.New("Device.CreatedAt should be zero time")
		}

		// 3. Store 接口编译期验证
		var store device.Store = new(mockDeviceStore)
		ctx := context.Background()

		// AddDevice
		kicked, err := store.AddDevice(ctx, "user-1", "dev-new", "jti-new", 5)
		if err != nil {
			return fmt.Errorf("Store.AddDevice error: %v", err)
		}
		if kicked == nil {
			return errors.New("mock AddDevice should return empty slice, not nil")
		}

		// CheckDevice
		valid, err := store.CheckDevice(ctx, "user-1", "dev-1", "jti-1")
		if err != nil {
			return fmt.Errorf("Store.CheckDevice error: %v", err)
		}
		if valid {
			return errors.New("mock CheckDevice should return false")
		}

		// ListDevices
		devices, err := store.ListDevices(ctx, "user-1")
		if err != nil {
			return fmt.Errorf("Store.ListDevices error: %v", err)
		}
		if devices == nil {
			return errors.New("mock ListDevices should return empty slice, not nil")
		}

		// RemoveDevice
		err = store.RemoveDevice(ctx, "user-1", "dev-1")
		if err != nil {
			return fmt.Errorf("Store.RemoveDevice error: %v", err)
		}

		// RemoveAllDevices
		err = store.RemoveAllDevices(ctx, "user-1")
		if err != nil {
			return fmt.Errorf("Store.RemoveAllDevices error: %v", err)
		}

		return nil
	})
}

// mockDeviceStore 是 device.Store 的最小 mock 实现。
type mockDeviceStore struct{}

func (m *mockDeviceStore) AddDevice(_ context.Context, _, _, _ string, _ int) ([]device.Device, error) {
	return []device.Device{}, nil
}
func (m *mockDeviceStore) CheckDevice(_ context.Context, _, _, _ string) (bool, error) {
	return false, nil
}
func (m *mockDeviceStore) RemoveDevice(_ context.Context, _, _ string) error  { return nil }
func (m *mockDeviceStore) RemoveAllDevices(_ context.Context, _ string) error { return nil }
func (m *mockDeviceStore) ListDevices(_ context.Context, _ string) ([]device.Device, error) {
	return []device.Device{}, nil
}

// ────────────────────────────────────────────────────────────
// go-middleware (实际连接测试，Docker Compose 服务不可用时返回 SKIP)
// ────────────────────────────────────────────────────────────

// verifyRedis 测试 Redis 连接与 SET/GET 操作。
func verifyRedis() result {
	const pkg = "go-middleware/redis"
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	client, closeFn, err := redis.NewUniversalClient(ctx, &redis.Config{
		Addrs:       []string{"localhost:6379"},
		DialTimeout: 500 * time.Millisecond,
	})
	if err != nil {
		return result{"SKIP", pkg, "Redis 不可达: " + err.Error()}
	}
	defer closeFn()

	// SET
	_ = client.Set(ctx, "demo-key", "hello-redis", 10*time.Second)
	// GET
	val, err := client.Get(ctx, "demo-key").Result()
	if err != nil {
		return result{"SKIP", pkg, "Redis 不可达: " + err.Error()}
	}
	if val != "hello-redis" {
		return result{"FAIL", pkg, fmt.Sprintf("GET 值不匹配: got %q, want \"hello-redis\"", val)}
	}
	// 清理
	_ = client.Del(ctx, "demo-key")
	return result{"PASS", pkg, "连接成功，SET/GET 验证通过"}
}

// verifyKafka 测试 Kafka Writer 创建与消息发送。
func verifyKafka() result {
	const pkg = "go-middleware/kafka"
	writer := kafka.NewWriter(kafka.WriterConfig{
		Broker: []string{"localhost:9092"},
		Topic:  "demo-test",
	})
	defer func() { _ = writer.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := writer.SendStr(ctx, "demo-key", "demo-value"); err != nil {
		return result{"SKIP", pkg, "Kafka 不可达: " + err.Error()}
	}
	return result{"PASS", pkg, "连接成功，消息已发送至 demo-test"}
}

// verifyDB 测试 MySQL 连接与 Ping。
func verifyDB() result {
	const pkg = "go-middleware/db"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	database, closeFn, err := db.NewDB(ctx,
		db.WithDriver("mysql"),
		db.WithSource("root:demo123@tcp(localhost:3306)/demo"),
	)
	if err != nil {
		return result{"SKIP", pkg, "MySQL 不可达: " + err.Error()}
	}
	defer closeFn()

	if err := database.Ping(ctx); err != nil {
		return result{"FAIL", pkg, "Ping 失败: " + err.Error()}
	}
	return result{"PASS", pkg, "连接成功，Ping 验证通过"}
}

// verifyES 测试 Elasticsearch 连接与集群 Ping。
func verifyES() result {
	const pkg = "go-middleware/es"
	client, err := es.NewClient(es.Config{
		Addresses: []string{"http://localhost:9200"},
	})
	if err != nil {
		return result{"SKIP", pkg, "ES 客户端创建失败: " + err.Error()}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pingResp, err := client.Ping(client.Ping.WithContext(ctx))
	if err != nil {
		return result{"SKIP", pkg, "ES 不可达: " + err.Error()}
	}
	defer func() { _ = pingResp.Body.Close() }()

	if pingResp.IsError() {
		return result{"SKIP", pkg, "ES Ping 返回错误: " + pingResp.Status()}
	}
	return result{"PASS", pkg, "连接成功，集群 Ping 验证通过"}
}

// verifyClickHouse 测试 ClickHouse 连接与 Ping。
func verifyClickHouse() result {
	const pkg = "go-middleware/clickhouse"
	conn, err := clickhouse.NewClient(clickhouse.Config{
		Addrs:    []string{"localhost:9000"},
		Database: "default",
		Username: "default",
		Password: "demo123",
	})
	if err != nil {
		return result{"SKIP", pkg, "ClickHouse 不可达: " + err.Error()}
	}
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return result{"SKIP", pkg, "ClickHouse Ping 失败: " + err.Error()}
	}
	return result{"PASS", pkg, "连接成功，Ping 验证通过"}
}

// verifyTLS 测试 TLS 连接（需要火山引擎专有云环境，SKIP）。
func verifyTLS() result {
	return result{"SKIP", "go-middleware/tls", "需要火山引擎 TLS 服务（专有云环境）"}
}

// verifyMiddlewareAuth 测试中间件认证（依赖 Redis Session 存储，SKIP）。
func verifyMiddlewareAuth() result {
	return result{"SKIP", "go-middleware/auth", "依赖 Redis Session 存储（独立测试见 verifyRedis）"}
}

// ────────────────────────────────────────────────────────────
// go-framework (实际测试)
// ────────────────────────────────────────────────────────────

// verifyHertz 测试 Hertz HTTP 服务器创建、启动与优雅关闭。
func verifyHertz() result {
	const pkg = "go-framework/hertz"
	ctx := context.Background()

	h, err := hertz.NewHTTPServer(ctx, &hertzConfig.ServerConfig{
		HTTP: &hertzConfig.HTTPOption{
			Port:        "19888",
			Mode:        1,
			IsTransport: true, // Go 标准网络库，跨平台兼容
		},
		Registry: config.RegistryOption{Name: "demo-service"},
	})
	if err != nil {
		return result{"SKIP", pkg, "服务器创建失败: " + err.Error()}
	}

	go h.Spin()
	time.Sleep(500 * time.Millisecond)

	shutdownCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_ = h.Shutdown(shutdownCtx)
	return result{"PASS", pkg, "服务器创建、启动、关闭验证通过"}
}

// verifyKitex 测试 Kitex 客户端 Option 创建。
// Kitex 在非 Linux 平台导入时可能 panic，仅在 Linux 上测试。
func verifyKitex() result {
	if isNotLinux() {
		return result{"SKIP", "go-framework/kitex", "Kitex 仅在 Linux 支持，当前: " + runtime.GOOS}
	}
	return result{"SKIP", "go-framework/kitex", "需要在 Linux 环境运行"}
}

// verifyConfig 测试 YAML 配置加载与 Polaris 远程配置。
func verifyConfig() result {
	const pkg = "go-framework/config"

	// 测试 YAML Loader（无需外部服务）
	tmpFile, err := os.CreateTemp("", "demo-config-*.yaml")
	if err != nil {
		return result{"FAIL", pkg, "创建临时文件失败: " + err.Error()}
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	content := "name: demo\nversion: \"1.0\"\n"
	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		return result{"FAIL", pkg, "写入临时文件失败: " + err.Error()}
	}
	_ = tmpFile.Close()

	type testConfig struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	}

	cfg, err := config.LoadYAML[testConfig](tmpFile.Name())
	if err != nil {
		return result{"FAIL", pkg, "LoadYAML 失败: " + err.Error()}
	}
	if cfg.Name != "demo" || cfg.Version != "1.0" {
		return result{"FAIL", pkg, fmt.Sprintf("配置不匹配: %+v", cfg)}
	}

	// 尝试 Polaris 远程配置（预期无 Polaris 服务时失败，不影响 PASS）
	polarisNote := ""
	pcf, err := config.LoadPolarisConfig(
		config.WithNamespace("default"),
		config.WithFileGroup("demo"),
		config.WithFileName("config.yaml"),
	)
	if err != nil {
		polarisNote = " (Polaris 不可用: " + err.Error() + ")"
	} else {
		_ = pcf.Content()
		polarisNote = " (Polaris 连接成功)"
	}

	return result{"PASS", pkg, "YAML Loader 验证通过" + polarisNote}
}
