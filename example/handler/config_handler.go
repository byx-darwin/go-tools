// Package handler 提供 config 配置验证和热重载的示例路由。
package handler

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"strings"
	"sync"

	"github.com/byx-darwin/go-tools/go-common/log"
	hertzresp "github.com/byx-darwin/go-tools/go-framework/hertz"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"gopkg.in/yaml.v3"
)

// configMu 保护运行时配置的读写（热重载安全）。
var configMu sync.RWMutex

// runtimeCfgPath 当前加载的配置文件路径。
var runtimeCfgPath = "config.yaml"

// SetConfigPath 设置运行时配置文件路径（由 main.go 初始化时调用）。
func SetConfigPath(path string) {
	runtimeCfgPath = path
}

// GetConfigMutex 返回配置读写锁（供 main.go 传入当前配置引用时使用）。
func GetConfigMutex() *sync.RWMutex {
	return &configMu
}

// RegisterConfigRoutes 注册 config 示例路由。
func RegisterConfigRoutes(h *server.Hertz) {
	h.GET("/config/load", configLoadHandler)
	h.GET("/config/duration", configDurationHandler)
	h.POST("/config/hot-reload", configHotReloadHandler)
	h.GET("/config/polaris", configPolarisHandler)
}

// configLoadHandler 返回当前配置（脱敏后）。
//
// GET /config/load
// 返回 JSON 格式的当前运行配置，敏感字段（secret/password/token）以 "***" 掩码显示。
func configLoadHandler(ctx context.Context, c *app.RequestContext) {
	configMu.RLock()
	defer configMu.RUnlock()

	// 使用反射脱敏：遍历 map 结构，将敏感字段值替换为 "***"。
	masked := maskSecrets(currentConfigSnapshot())
	hertzresp.Success(c, masked)
}

// configDurationHandler 演示 config.Duration 解析能力。
//
// GET /config/duration
// 展示 AppConfig 中使用 config.Duration 的字段及其解析后的 time.Duration 值。
func configDurationHandler(ctx context.Context, c *app.RequestContext) {
	configMu.RLock()
	defer configMu.RUnlock()

	snap := currentConfigSnapshot()
	demo := map[string]any{
		"jwt_access_expiration":  fmt.Sprintf("%v", snap["jwt.access_expiration"]),
		"jwt_refresh_expiration": fmt.Sprintf("%v", snap["jwt.refresh_expiration"]),
		"hertz_exit_wait_time":   fmt.Sprintf("%v", snap["hertz.http.exit_wait_time"]),
		"hertz_idle_timeout":     fmt.Sprintf("%v", snap["hertz.http.idle_timeout"]),
		"kitex_read_timeout":     fmt.Sprintf("%v", snap["kitex.timeout.read_write_timeout"]),
		"kitex_exit_wait":        fmt.Sprintf("%v", snap["kitex.timeout.exit_wait_timeout"]),
		"captcha_expires_time":   fmt.Sprintf("%v", snap["captcha.cache_expires_time"]),
		"metrics_interval":       fmt.Sprintf("%v", snap["observability.metrics_interval"]),
		"note":                   "config.Duration supports YAML strings like '30s', '5m', '24h', '7d'",
	}
	hertzresp.Success(c, demo)
}

// configHotReloadHandler 重新加载 config.yaml 并比较前后差异。
//
// POST /config/hot-reload
// 返回 before/after 快照及差异字段列表。
func configHotReloadHandler(ctx context.Context, c *app.RequestContext) {
	configMu.Lock()
	defer configMu.Unlock()

	before := currentConfigSnapshot()

	newCfg, err := reloadConfig(runtimeCfgPath)
	if err != nil {
		hertzresp.Error(ctx, c, fmt.Errorf("reload config: %w", err), "配置重载失败")
		return
	}

	after := configToSnapshot(newCfg)
	diffs := diffSnapshots(before, after)

	// 更新全局配置引用（通过 SetCurrentConfig）。
	SetCurrentConfig(newCfg)

	log.L().Info("config hot-reloaded", "path", runtimeCfgPath, "diff_count", len(diffs))

	hertzresp.Success(c, map[string]any{
		"message":  "config reloaded successfully",
		"path":     runtimeCfgPath,
		"diffs":    diffs,
		"has_diff": len(diffs) > 0,
	})
}

// configPolarisHandler 尝试从 Polaris 加载配置。
//
// GET /config/polaris
// 如果 Polaris 未启用，返回提示信息。
func configPolarisHandler(ctx context.Context, c *app.RequestContext) {
	configMu.RLock()
	defer configMu.RUnlock()

	snap := currentConfigSnapshot()
	enabled, _ := snap["polaris.enabled"].(bool)
	if !enabled {
		hertzresp.Success(c, map[string]any{
			"status":  "not_enabled",
			"message": "Polaris config center is not enabled in config.yaml. Set polaris.enabled=true to enable.",
		})
		return
	}

	// Polaris 已启用时返回配置信息（实际调用需要 Polaris SDK 环境）。
	hertzresp.Success(c, map[string]any{
		"status":     "enabled",
		"namespace":  snap["polaris.namespace"],
		"file_group": snap["polaris.file_group"],
		"file_name":  snap["polaris.file_name"],
		"message":    "Polaris is enabled. Remote config loading requires a running Polaris server.",
	})
}

// currentConfig 当前运行时配置引用（由 main.go 初始化）。
var currentConfig any

// SetCurrentConfig 设置当前运行时配置引用。
func SetCurrentConfig(cfg any) {
	currentConfig = cfg
}

// currentConfigSnapshot 将当前配置转为扁平 map（读取时调用，需持有读锁）。
func currentConfigSnapshot() map[string]any {
	if currentConfig == nil {
		return map[string]any{}
	}
	return configToSnapshot(currentConfig)
}

// configToSnapshot 使用 YAML round-trip 将任意配置结构转为 map。
func configToSnapshot(cfg any) map[string]any {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return map[string]any{"error": err.Error()}
	}
	return m
}

// reloadConfig 从文件重新加载配置。
func reloadConfig(path string) (any, error) {
	// 使用反射调用 LoadConfig 函数（避免 import cycle）。
	// 这里直接返回 nil 让 main.go 提供重载回调。
	if configReloadFn != nil {
		return configReloadFn(path)
	}
	return nil, fmt.Errorf("config reload function not registered")
}

// configReloadFn 配置重载回调（由 main.go 注册）。
var configReloadFn func(path string) (any, error)

// SetConfigReloadFn 注册配置重载函数。
func SetConfigReloadFn(fn func(path string) (any, error)) {
	configReloadFn = fn
}

// maskSecrets 将 map 中敏感字段的值替换为 "***"。
func maskSecrets(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		if isSensitiveKey(k) {
			result[k] = "***"
		} else if sub, ok := v.(map[string]any); ok {
			result[k] = maskSecrets(sub)
		} else {
			result[k] = v
		}
	}
	return result
}

// isSensitiveKey 判断 key 是否包含敏感词。
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, s := range []string{"secret", "password", "token", "sk", "tea_key", "app_key"} {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// diffSnapshots 比较两个 map 的差异，返回变化字段列表。
func diffSnapshots(before, after map[string]any) []map[string]any {
	var diffs []map[string]any
	// 使用扁平化后的 key 比较。
	flatBefore := flattenMap(before, "")
	flatAfter := flattenMap(after, "")

	allKeys := make(map[string]struct{})
	for k := range flatBefore {
		allKeys[k] = struct{}{}
	}
	for k := range flatAfter {
		allKeys[k] = struct{}{}
	}

	for k := range allKeys {
		vb, okB := flatBefore[k]
		va, okA := flatAfter[k]
		if !okB {
			diffs = append(diffs, map[string]any{"key": k, "type": "added", "new": va})
		} else if !okA {
			diffs = append(diffs, map[string]any{"key": k, "type": "removed", "old": vb})
		} else if !reflect.DeepEqual(vb, va) {
			diffs = append(diffs, map[string]any{"key": k, "type": "changed", "old": vb, "new": va})
		}
	}
	return diffs
}

// flattenMap 将嵌套 map 扁平化为 "a.b.c" 形式的 key-value。
func flattenMap(m map[string]any, prefix string) map[string]any {
	result := make(map[string]any)
	for k, v := range m {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}
		if sub, ok := v.(map[string]any); ok {
			maps.Copy(result, flattenMap(sub, fullKey))
		} else {
			result[fullKey] = v
		}
	}
	return result
}
