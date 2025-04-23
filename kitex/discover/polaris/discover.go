package polaris

import (
	"context"
	"github.com/cloudwego/hertz/pkg/common/hlog"
	"github.com/cloudwego/kitex/client"
	"github.com/kitex-contrib/polaris"
)

func NewResolver(ctx context.Context,
	nameSpace string, log hlog.CtxLogger) (client.Option, error) {
	resolver, err := polaris.NewPolarisResolver(polaris.ClientOptions{})
	if err != nil {
		log.CtxErrorf(ctx, "NewPolarisResolver creates a polaris based resolver:%v error：%v", resolver, err)
		return client.Option{}, err
	}
	balancer, err := polaris.NewPolarisBalancer()
	if err != nil {
		log.CtxDebugf(ctx, "NewPolarisBalancer creates a polaris based balancer:%v error：%v", balancer, err)
		return client.Option{}, err
	}
	suite := &polaris.ClientSuite{
		DstNameSpace:       nameSpace,
		Resolver:           resolver,
		Balancer:           balancer,
		ReportCallResultMW: polaris.NewUpdateServiceCallResultMW(),
	}

	return client.WithSuite(suite), nil
}
