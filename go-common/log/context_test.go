package log_test

import (
	"context"
	"testing"

	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/stretchr/testify/require"
)

func TestContext_RequestID(t *testing.T) {
	ctx := context.Background()
	ctx = log.WithRequestID(ctx, "req-123")
	require.Equal(t, "req-123", log.RequestIDFromContext(ctx))
}

func TestContext_RequestID_Empty(t *testing.T) {
	ctx := context.Background()
	require.Equal(t, "", log.RequestIDFromContext(ctx))
}

func TestContext_GenericValue(t *testing.T) {
	ctx := context.Background()
	ctx = log.WithContextValue(ctx, "user_id", "456")
	require.Equal(t, "456", log.ContextValue(ctx, "user_id"))
}

func TestContext_GenericValue_NotFound(t *testing.T) {
	ctx := context.Background()
	require.Equal(t, "", log.ContextValue(ctx, "nonexistent"))
}

func TestContext_MultipleValues(t *testing.T) {
	ctx := context.Background()
	ctx = log.WithContextValue(ctx, "key1", "val1")
	ctx = log.WithContextValue(ctx, "key2", "val2")
	require.Equal(t, "val1", log.ContextValue(ctx, "key1"))
	require.Equal(t, "val2", log.ContextValue(ctx, "key2"))
}

func TestContext_Keys(t *testing.T) {
	require.Equal(t, "request_id", log.ContextKeyRequestID)
	require.Equal(t, "trace_id", log.ContextKeyTraceID)
	require.Equal(t, "span_id", log.ContextKeySpanID)
}
