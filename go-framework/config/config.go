// Package config 提供 go-framework 的通用配置类型。
package config

import "time"

// JaegerOption 链路追踪配置
type JaegerOption struct {
	Enable   bool   `json:"enable" yaml:"enable"`
	Endpoint string `json:"endpoint"  yaml:"endpoint"`
}

// RegistryOption 服务注册中心配置
type RegistryOption struct {
	Enable  bool   `json:"enable" yaml:"enable"`
	Space   string `json:"space"  yaml:"space"`
	Name    string `json:"name"  yaml:"name"`
	Version string `json:"version"  yaml:"version"`
	Env     string `json:"env" yaml:"env"`
	Network string `json:"network"  yaml:"network"`
	Address string `json:"address"  yaml:"address"`
}

// Loader 通用 YAML 配置加载器接口
type Loader interface {
	Load(path string, v interface{}) error
}

// Duration 封装 time.Duration，支持 YAML 字符串格式 (e.g., "30s", "5m")
type Duration struct {
	time.Duration
}

// UnmarshalYAML 实现 yaml.Unmarshaler，解析 "30s" 等格式
func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}
