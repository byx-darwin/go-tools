// Package config 提供 Polaris（北极星）服务治理平台的远程配置加载。
package config

import (
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"

	"github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// PolarisConfigFile 封装 Polaris 配置文件的获取和监听。
type PolarisConfigFile struct {
	cf model.ConfigFile
}

// LoadPolarisConfig 从 Polaris 拉取远程配置文件，支持 Options 配置。
//
// 用法：
//
//	pcf, err := config.LoadPolarisConfig(
//	    config.WithNamespace("production"),
//	    config.WithFileGroup("myapp"),
//	    config.WithFileName("config.yaml"),
//	    config.WithChangeListener(onChange),
//	)
func LoadPolarisConfig(opts ...PolarisOption) (*PolarisConfigFile, error) {
	cfg := &polarisConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	configAPI, err := polaris.NewConfigAPI()
	if err != nil {
		return nil, frameworkerror.ErrPolarisInit.Wrap(err)
	}

	configFile, err := configAPI.GetConfigFile(cfg.namespace, cfg.group, cfg.fileName) //nolint:staticcheck // Polaris SDK 尚未提供 FetchConfigFile 稳定版本
	if err != nil {
		return nil, frameworkerror.ErrPolarisGetConfig.Wrap(err)
	}

	if cfg.listener != nil {
		configFile.AddChangeListener(cfg.listener)
	}

	return &PolarisConfigFile{cf: configFile}, nil
}

// LoadPolarisConfigLegacy 从 Polaris 拉取远程配置文件。
//
// Deprecated: 使用 LoadPolarisConfig 配合 Options 替代。
func LoadPolarisConfigLegacy(namespace, fileGroup, fileName string,
	listener func(event model.ConfigFileChangeEvent)) (*PolarisConfigFile, error) {
	opts := []PolarisOption{
		WithNamespace(namespace),
		WithFileGroup(fileGroup),
		WithFileName(fileName),
	}
	if listener != nil {
		opts = append(opts, WithChangeListener(listener))
	}
	return LoadPolarisConfig(opts...)
}

// Content 返回配置文件内容。
func (p *PolarisConfigFile) Content() string {
	return p.cf.GetContent()
}
