package hertz

// blank-import go-framework/error，确保框架错误码 → HTTP 状态码的
// 细粒度映射注册表在任何使用 hertz 包的应用中生效
// （即使应用未直接使用 frameworkerror 符号）。
import (
	_ "github.com/byx-darwin/go-tools/go-framework/error"
)
