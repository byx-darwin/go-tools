package polaris

import (
	"context"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/registry"
	"github.com/cloudwego/kitex/server"
	"github.com/kitex-contrib/polaris"
)

func NewRegistry(ctx context.Context,
	nameSpace, serviceName string,
	log klog.CtxLogger) ([]server.Option, error) {
	r, err := polaris.NewPolarisRegistry(polaris.ServerOptions{})
	if err != nil {
		log.CtxErrorf(ctx, "polaris NewPolarisRegistry fatal ：%v", err)
		return nil, err
	}
	info := &registry.Info{
		ServiceName: serviceName,
		Tags: map[string]string{
			polaris.NameSpaceTagKey: nameSpace,
		},
	}
	options := make([]server.Option, 0, 2)
	options = append(options, server.WithRegistry(r))
	options = append(options, server.WithRegistryInfo(info))
	return options, nil
}
