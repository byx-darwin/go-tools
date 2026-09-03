package sse

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	hertzsse "github.com/cloudwego/hertz/pkg/protocol/sse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteErrorEvent(t *testing.T) {
	addr, stop := startTestServer(t, func(_ context.Context, c *app.RequestContext) {
		w := hertzsse.NewWriter(c)
		require.NoError(t, writeErrorEvent(w, 500, "internal server error"))
		require.NoError(t, w.Close())
	})
	defer stop()

	conn := rawHTTPGet(t, addr, "/sse")
	defer func() { _ = conn.Close() }()

	out := readAllWithTimeout(t, conn, 2*time.Second)

	assert.Contains(t, out, "event: error\n")

	const prefix = "data: "
	idx := bytes.Index([]byte(out), []byte(prefix))
	require.GreaterOrEqual(t, idx, 0)
	rest := out[idx+len(prefix):]
	line := rest[:bytes.IndexByte([]byte(rest), '\n')]

	var payload sseErrorPayload
	require.NoError(t, json.Unmarshal([]byte(line), &payload))
	assert.Equal(t, 500, payload.Code)
	assert.Equal(t, "internal server error", payload.Msg)
	assert.Nil(t, payload.Data)
}
