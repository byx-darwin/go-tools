// Package config 提供 Polaris（北极星）服务治理平台的远程配置加载。
package config

import (
	"fmt"

	"github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
)

// PolarisConfigFile 封装 Polaris 配置文件的获取和监听。
type PolarisConfigFile struct {
	cf model.ConfigFile
}

// LoadPolarisConfig 从 Polaris 拉取远程配置文件。
func LoadPolarisConfig(namespace, fileGroup, fileName string,
	listener func(event model.ConfigFileChangeEvent)) (*PolarisConfigFile, error) {
	configAPI, err := polaris.NewConfigAPI()
	if err != nil {
		return nil, fmt.Errorf("polaris init failed: %w", err)
	}

	configFile, err := configAPI.GetConfigFile(namespace, fileGroup, fileName)
	if err != nil {
		return nil, fmt.Errorf("polaris get config file: %w", err)
	}

	if listener != nil {
		configFile.AddChangeListener(listener)
	}

	return &PolarisConfigFile{cf: configFile}, nil
}

// Content 返回配置文件内容。
func (p *PolarisConfigFile) Content() string {
	return p.cf.GetContent()
}
