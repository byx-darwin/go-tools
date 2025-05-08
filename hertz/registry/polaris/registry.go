package polaris

import (
	"context"
	toolsConfig "gitee.com/byx_darwin/go-tools/config"
	httpServer "github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/app/server/registry"
	"github.com/cloudwego/hertz/pkg/common/config"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/hertz-contrib/registry/polaris"
)

func NewRegistry(ctx context.Context,
	registryOption toolsConfig.RegistryOption,
	log hlog.CtxLogger) ([]config.Option, error) {
	r, err := polaris.NewPolarisRegistry()
	if err != nil {
		log.CtxErrorf(ctx, "polaris NewPolarisRegistry fatal ：%v", err)
		return nil, err
	}
	info := &registry.Info{
		ServiceName: registryOption.Name,
		Addr:        utils.NewNetAddr(registryOption.Network, registryOption.Address),
		Tags: map[string]string{
			"namespace": registryOption.Space,
		},
	}
	options := make([]config.Option, 0, 1)
	options = append(options, httpServer.WithRegistry(r, info))
	return options, nil
}
