package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// LoadYAML 从 YAML 文件加载配置（泛型）。
//
//	cfg, err := config.LoadYAML[MyConfig]("/path/to/config.yaml")
func LoadYAML[T any](path string) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg T
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// MustLoadYAML 加载配置，失败则 panic。
func MustLoadYAML[T any](path string) *T {
	cfg, err := LoadYAML[T](path)
	if err != nil {
		panic("config: " + path + ": " + err.Error())
	}
	return cfg
}
