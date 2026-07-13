package log

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDomainHandler_InjectsFields(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	domainHandler := NewDomainHandler(handler, "order", "decision")

	logger := slog.New(domainHandler)
	logger.InfoContext(context.Background(), "test message")

	var result map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, "order", result["domain"])
	assert.Equal(t, "decision", result["log_type"])
	assert.Equal(t, "test message", result["msg"])
}

func TestDomainHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	domainHandler := NewDomainHandler(handler, "user", "event")

	logger := slog.New(domainHandler).With("user_id", "123")
	logger.InfoContext(context.Background(), "user created")

	var result map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))

	assert.Equal(t, "user", result["domain"])
	assert.Equal(t, "event", result["log_type"])
	assert.Equal(t, "123", result["user_id"])
}
