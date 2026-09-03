package rpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStreamEcho_EndToEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addr := "127.0.0.1:18899" // fixed test port; adjust if it collides with another test in this package
	go func() { _ = StartServer(ctx, addr, nil) }()
	time.Sleep(100 * time.Millisecond) // wait for server to bind

	cli, err := NewDemoClient(addr, nil)
	require.NoError(t, err)

	frames, err := CallStreamEcho(context.Background(), cli, "hi")
	require.NoError(t, err)
	require.Equal(t, []string{"h", "i"}, frames)
}
