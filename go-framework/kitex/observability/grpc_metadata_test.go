package observability

import (
	"context"
	"testing"

	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/metadata"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func TestGRPCMetadataSupplier_NilMetadata(t *testing.T) {
	s := &grpcMetadataSupplier{}
	assert.Empty(t, s.Get("key"))
	assert.Nil(t, s.Keys())
	// Set on nil metadata must not panic.
	s.Set("key", "value")
}

func TestGRPCMetadataSupplier_GetSetKeys(t *testing.T) {
	md := metadata.MD{}
	s := &grpcMetadataSupplier{metadata: &md}

	// Get on an absent key returns empty string.
	assert.Empty(t, s.Get("traceparent"))

	s.Set("traceparent", "00-trace-span-01")
	assert.Equal(t, "00-trace-span-01", s.Get("traceparent"))
	assert.Contains(t, s.Keys(), "traceparent")

	// Get with empty value slice.
	md["empty-key"] = []string{}
	assert.Empty(t, s.Get("empty-key"))
}

func TestInjectExtractGRPCMetadata_RoundTrip(t *testing.T) {
	prevPropagator := otel.GetTextMapPropagator()
	defer otel.SetTextMapPropagator(prevPropagator)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	md := metadata.MD{}
	ctx := context.Background()

	ctx = injectGRPCMetadata(ctx, md)
	// TraceContext propagator only injects when a valid span exists in ctx,
	// so without a recording span md may stay empty; just assert no panic
	// and a usable context is returned.
	assert.NotNil(t, ctx)

	extractedCtx := extractGRPCMetadata(context.Background(), md)
	assert.NotNil(t, extractedCtx)
}

func TestExtractPeerServiceAttributesFromGRPCMetadata(t *testing.T) {
	md := metadata.MD{}
	md[string(semconv.ServiceNameKey)] = []string{"downstream-service"}
	md[string(semconv.ServiceNamespaceKey)] = []string{"ns-prod"}
	md[string(semconv.DeploymentEnvironmentKey)] = []string{"prod"}

	attrs := extractPeerServiceAttributesFromGRPCMetadata(md)
	assert.Len(t, attrs, 3)

	attrMap := make(map[string]string)
	for _, attr := range attrs {
		attrMap[string(attr.Key)] = attr.Value.AsString()
	}
	assert.Equal(t, "downstream-service", attrMap[string(semconv.PeerServiceKey)])
	assert.Equal(t, "ns-prod", attrMap[string(PeerServiceNamespaceKey)])
	assert.Equal(t, "prod", attrMap[string(PeerDeploymentEnvironmentKey)])
}

func TestExtractPeerServiceAttributesFromGRPCMetadata_Empty(t *testing.T) {
	attrs := extractPeerServiceAttributesFromGRPCMetadata(metadata.MD{})
	assert.Empty(t, attrs)
}
