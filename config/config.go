package config

// JaegerOption 链路追踪配置
type JaegerOption struct {
	Enable   bool   `json:"enable" yaml:"enable"`      //是否启用链路追踪
	Endpoint string `json:"endpoint"  yaml:"endpoint"` //地址
}

type RegistryOption struct {
	Enable  bool   `json:"enable" yaml:"enable"`    //是否启用注册中心
	Space   string `json:"space"  yaml:"space"`     //服务空间名称
	Name    string `json:"name"  yaml:"name"`       //服务名称
	Version string `json:"version"  yaml:"version"` //版本信息
	Env     string `json:"env" yaml:"env"`          //环境信息
	Network string `json:"network"  yaml:"network"` //网络信息
	Address string `json:"address"  yaml:"address"` //地址信息
}
