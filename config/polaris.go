package config

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/polarismesh/polaris-go"
	"github.com/polarismesh/polaris-go/pkg/model"
)

func ConfigPolarisApi(namespace, fileGroup, fileName string,
	listener func(event model.ConfigFileChangeEvent)) (model.ConfigFile, error) {
	//解析配置文件
	configAPI, err := polaris.NewConfigAPI()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("polaris 初始化失败: %v", err))
	}
	//获取远程配置文件
	configFile, err := configAPI.GetConfigFile(namespace, fileGroup, fileName)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("获取远程配置文件错误: %v", err))
	}
	if listener != nil {
		configFile.AddChangeListener(listener)
	}
	return configFile, nil
}
