package observability

import (
	"context"
	"testing"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func TestGetServiceFromResourceAttributes(t *testing.T) {
	attrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String("order-service"),
		semconv.ServiceNamespaceKey.String("production"),
		semconv.DeploymentEnvironmentKey.String("prod-us-east"),
	}

	name, namespace, env := getServiceFromResourceAttributes(attrs)
	assert.Equal(t, "order-service", name)
	assert.Equal(t, "production", namespace)
	assert.Equal(t, "prod-us-east", env)
}

func TestGetServiceFromResourceAttributes_Empty(t *testing.T) {
	name, namespace, env := getServiceFromResourceAttributes(nil)
	assert.Empty(t, name)
	assert.Empty(t, namespace)
	assert.Empty(t, env)
}

func TestGetServiceFromResourceAttributes_Partial(t *testing.T) {
	attrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String("user-service"),
	}

	name, namespace, env := getServiceFromResourceAttributes(attrs)
	assert.Equal(t, "user-service", name)
	assert.Empty(t, namespace)
	assert.Empty(t, env)
}

func TestExtractPeerServiceAttributesFromMetaInfo(t *testing.T) {
	md := map[string]string{
		"service.name":           "gateway-service",
		"service.namespace":      "production",
		"deployment.environment": "prod-us-west",
		"unrelated.key":          "should-be-ignored",
	}

	attrs := extractPeerServiceAttributesFromMetaInfo(md)
	assert.Len(t, attrs, 3)

	attrMap := make(map[attribute.Key]string)
	for _, attr := range attrs {
		attrMap[attr.Key] = attr.Value.AsString()
	}
	assert.Equal(t, "gateway-service", attrMap[semconv.PeerServiceKey])
	assert.Equal(t, "production", attrMap[PeerServiceNamespaceKey])
	assert.Equal(t, "prod-us-west", attrMap[PeerDeploymentEnvironmentKey])
}

func TestExtractPeerServiceAttributesFromMetaInfo_Empty(t *testing.T) {
	attrs := extractPeerServiceAttributesFromMetaInfo(nil)
	assert.Empty(t, attrs)
}

func TestInjectPeerServiceToMetaInfo(t *testing.T) {
	ctx := metainfo.WithBackwardValuesToSend(context.Background())

	attrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String("order-service"),
		semconv.ServiceNamespaceKey.String("production"),
		semconv.DeploymentEnvironmentKey.String("prod-us-east"),
	}

	injectPeerServiceToMetaInfo(ctx, attrs)

	sent := metainfo.AllBackwardValuesToSend(ctx)
	assert.Equal(t, "order-service", sent[string(semconv.ServiceNameKey)])
	assert.Equal(t, "production", sent[string(semconv.ServiceNamespaceKey)])
	assert.Equal(t, "prod-us-east", sent[string(semconv.DeploymentEnvironmentKey)])
}

func TestInjectPeerServiceToMetaInfo_EmptyAttrs(t *testing.T) {
	ctx := metainfo.WithBackwardValuesToSend(context.Background())

	// Should not panic and should leave nothing sent when attrs are empty.
	injectPeerServiceToMetaInfo(ctx, nil)

	sent := metainfo.AllBackwardValuesToSend(ctx)
	assert.Empty(t, sent)
}

func TestInjectPeerServiceToMetaInfo_NoBackwardContext(t *testing.T) {
	// Without metainfo.WithBackwardValuesToSend, SendBackwardValuesFromMap
	// silently no-ops; this must not panic.
	attrs := []attribute.KeyValue{semconv.ServiceNameKey.String("svc")}
	injectPeerServiceToMetaInfo(context.Background(), attrs)
}
