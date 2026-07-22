package error

import (
	"fmt"
	"sync"
)

// httpStatusRegistry 存储各模块注册的"错误码 → HTTP 状态码"细粒度映射。
// 仅预期在包 init() 阶段写入，运行期只读。
var (
	httpStatusMu       sync.RWMutex
	httpStatusRegistry = map[int]int{}
)

// RegisterHTTPStatuses 注册错误码到 HTTP 状态码的细粒度映射。
// 预期在各模块的包 init() 中调用（如 go-framework/error、go-middleware/clickhouse）。
// 重复注册同一错误码会 panic，以在启动期暴露配置错误。
func RegisterHTTPStatuses(m map[int]int) {
	httpStatusMu.Lock()
	defer httpStatusMu.Unlock()
	for code, status := range m {
		if _, exists := httpStatusRegistry[code]; exists {
			panic(fmt.Sprintf("go-common/error: duplicate HTTP status registration for code %d", code))
		}
		httpStatusRegistry[code] = status
	}
}

// lookupHTTPStatus 查询错误码的已注册 HTTP 状态码。
func lookupHTTPStatus(code int) (int, bool) {
	httpStatusMu.RLock()
	defer httpStatusMu.RUnlock()
	status, ok := httpStatusRegistry[code]
	return status, ok
}
