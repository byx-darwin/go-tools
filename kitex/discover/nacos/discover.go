package nacos

import (
	"context"
	"gitcode.com/sznc/go-tools/config/nacos"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/kitex/client"
	"github.com/kitex-contrib/registry-nacos/resolver"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
)

type Suite struct{}

func (s *Suite) NewResolver(ctx context.Context,
	c nacos.Config,
	log hlog.CtxLogger) (client.Option, error) {
	sc := []constant.ServerConfig{
		*constant.NewServerConfig(c.IPAddr, c.Port),
	}
	cc := constant.ClientConfig{
		NamespaceId:         c.NamespaceId,
		TimeoutMs:           c.TimeoutMs,
		NotLoadCacheAtStart: c.NotLoadCacheAtStart,
		LogDir:              c.LogDir,
		CacheDir:            c.CacheDir,
		LogLevel:            c.LogLevel,
		Username:            c.Username,
		Password:            c.Password,
	}
	cli, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  &cc,
			ServerConfigs: sc,
		},
	)
	if err != nil {
		log.CtxErrorf(ctx, "NewNamingClient failure IPAddr:%v  Port:%v error：%v", c.IPAddr, c.Port, err)
		return client.Option{}, err
	}
	nanosResolver := resolver.NewNacosResolver(cli)

	return client.WithResolver(nanosResolver), nil

}
