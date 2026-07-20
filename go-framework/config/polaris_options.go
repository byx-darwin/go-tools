package config

import "github.com/polarismesh/polaris-go/pkg/model"

// PolarisOption 定义 LoadPolarisConfig 的配置选项函数。
type PolarisOption func(*polarisConfig)

// polarisConfig 是 LoadPolarisConfig 的内部配置聚合。
type polarisConfig struct {
	namespace string
	group     string
	fileName  string
	listener  func(event model.ConfigFileChangeEvent)
}

// WithNamespace 设置 Polaris 命名空间。
func WithNamespace(ns string) PolarisOption {
	return func(c *polarisConfig) {
		if ns != "" {
			c.namespace = ns
		}
	}
}

// WithFileGroup 设置 Polaris 文件组。
func WithFileGroup(group string) PolarisOption {
	return func(c *polarisConfig) {
		if group != "" {
			c.group = group
		}
	}
}

// WithFileName 设置 Polaris 文件名。
func WithFileName(name string) PolarisOption {
	return func(c *polarisConfig) {
		if name != "" {
			c.fileName = name
		}
	}
}

// WithChangeListener 设置配置变更监听器。
func WithChangeListener(listener func(event model.ConfigFileChangeEvent)) PolarisOption {
	return func(c *polarisConfig) {
		if listener != nil {
			c.listener = listener
		}
	}
}
