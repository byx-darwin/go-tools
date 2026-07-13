// Package config 提供 Polaris（北极星）服务治理平台的远程配置加载。
package config

import (
	goerror "github.com/byx-darwin/go-tools/go-common/error"

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
		return nil, goerror.ErrPolarisInit.Wrap(err)
	}

	configFile, err := configAPI.GetConfigFile(namespace, fileGroup, fileName) //nolint:staticcheck // Polaris SDK 尚未提供 FetchConfigFile 稳定版本
	if err != nil {
		return nil, goerror.ErrPolarisGetConfig.Wrap(err)
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
