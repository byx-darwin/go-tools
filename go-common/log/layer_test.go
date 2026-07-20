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

func TestLayerFunctions_Category(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := &Logger{Logger: slog.New(handler), level: slog.LevelDebug}
	SetDefault(logger)
	defer SetDefault(nil)

	tests := []struct {
		name     string
		fn       func(context.Context) *Logger
		expected string
	}{
		{"App", App, "app"},
		{"DB", DB, "db"},
		{"Access", Access, "access"},
		{"RPC", RPC, "rpc"},
		{"MQ", MQ, "mq"},
		{"Cache", Cache, "cache"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			l := tt.fn(context.Background())
			l.InfoContext(context.Background(), "test")

			var result map[string]any
			require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
			assert.Equal(t, tt.expected, result["category"])
		})
	}
}
