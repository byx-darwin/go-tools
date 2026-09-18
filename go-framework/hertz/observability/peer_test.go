package observability

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.36.0"
)

func TestSemconvAttributeKeyToHTTPHeader(t *testing.T) {
	assert.Equal(t, "service-name", semconvAttributeKeyToHTTPHeader("service.name"))
	assert.Equal(t, "deployment-environment-name", semconvAttributeKeyToHTTPHeader("deployment.environment.name"))
}

func TestGetServiceFromResourceAttributes(t *testing.T) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName("svc-a"),
		semconv.ServiceNamespace("ns-a"),
		semconv.DeploymentEnvironmentName("prod"),
		attribute.String("irrelevant", "x"),
	}
	name, ns, env := getServiceFromResourceAttributes(attrs)
	assert.Equal(t, "svc-a", name)
	assert.Equal(t, "ns-a", ns)
	assert.Equal(t, "prod", env)
}

func TestGetServiceFromResourceAttributes_Empty(t *testing.T) {
	name, ns, env := getServiceFromResourceAttributes(nil)
	assert.Empty(t, name)
	assert.Empty(t, ns)
	assert.Empty(t, env)
}

func TestInjectPeerServiceToHTTPHeaders_And_Extract(t *testing.T) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName("svc-a"),
		semconv.ServiceNamespace("ns-a"),
		semconv.DeploymentEnvironmentName("prod"),
	}

	header := &protocol.RequestHeader{}
	injectPeerServiceToHTTPHeaders(attrs, header)

	assert.Equal(t, "svc-a", header.Get("service-name"))
	assert.Equal(t, "ns-a", header.Get("service-namespace"))
	assert.Equal(t, "prod", header.Get("deployment-environment-name"))

	extracted := extractPeerServiceAttributesFromHTTPHeaders(header)
	got := map[attribute.Key]string{}
	for _, a := range extracted {
		got[a.Key] = a.Value.AsString()
	}
	assert.Equal(t, "svc-a", got[semconv.PeerServiceKey])
	assert.Equal(t, "ns-a", got[PeerServiceNamespaceKey])
	assert.Equal(t, "prod", got[PeerDeploymentEnvironmentKey])
}

func TestInjectPeerServiceToHTTPHeaders_Empty(t *testing.T) {
	header := &protocol.RequestHeader{}
	injectPeerServiceToHTTPHeaders(nil, header)
	assert.Empty(t, header.Get("service-name"))
}

func TestExtractPeerServiceAttributesFromHTTPHeaders_NoHeaders(t *testing.T) {
	header := &protocol.RequestHeader{}
	attrs := extractPeerServiceAttributesFromHTTPHeaders(header)
	assert.Empty(t, attrs)
}
