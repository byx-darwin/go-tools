package log

// ReleaseInfo 发布信息，用于在日志中注入服务元数据。
type ReleaseInfo struct {
	// ServiceName 服务名称。
	ServiceName string `yaml:"service_name" json:"service_name"`

	// Version 服务版本。
	Version string `yaml:"version" json:"version"`

	// GitSHA Git 提交哈希。
	GitSHA string `yaml:"git_sha" json:"git_sha"`

	// BuildTime 构建时间。
	BuildTime string `yaml:"build_time" json:"build_time"`

	// Environment 运行环境（如 production, staging, development）。
	Environment string `yaml:"environment" json:"environment"`

	// Extra 自定义扩展字段。
	Extra map[string]string `yaml:"extra" json:"extra"`
}

// WithExtra 添加自定义扩展字段，返回新的 ReleaseInfo。
func (r ReleaseInfo) WithExtra(key, value string) ReleaseInfo {
	if r.Extra == nil {
		r.Extra = make(map[string]string)
	}
	r.Extra[key] = value
	return r
}
