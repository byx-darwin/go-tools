package tls

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func TestSpanLinks_Deduplicates(t *testing.T) {
	sc1 := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1},
		SpanID:     trace.SpanID{1},
		TraceFlags: trace.FlagsSampled,
	})
	sc2 := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{2},
		SpanID:     trace.SpanID{2},
		TraceFlags: trace.FlagsSampled,
	})

	links := spanLinks([]trace.SpanContext{sc1, sc1, sc2})

	require.Len(t, links, 2)
	require.Equal(t, sc1, links[0].SpanContext)
	require.Equal(t, sc2, links[1].SpanContext)
}

func TestSpanLinks_EmptyInput(t *testing.T) {
	links := spanLinks(nil)
	require.Empty(t, links)
}
