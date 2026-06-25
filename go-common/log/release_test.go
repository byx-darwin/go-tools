package log_test

import (
	"testing"

	"github.com/byx-darwin/go-tools/go-common/log"
	"github.com/stretchr/testify/require"
)

func TestReleaseInfo_WithExtra(t *testing.T) {
	r := log.ReleaseInfo{
		ServiceName: "user-service",
		Version:     "v1.0.0",
	}
	r = r.WithExtra("region", "us-west-2")
	require.Equal(t, "us-west-2", r.Extra["region"])
}

func TestReleaseInfo_WithExtra_Multiple(t *testing.T) {
	r := log.ReleaseInfo{
		ServiceName: "user-service",
		Version:     "v1.0.0",
	}
	r = r.WithExtra("region", "us-west-2").
		WithExtra("env", "production")
	require.Equal(t, "us-west-2", r.Extra["region"])
	require.Equal(t, "production", r.Extra["env"])
}

func TestReleaseInfo_Fields(t *testing.T) {
	r := log.ReleaseInfo{
		ServiceName: "user-service",
		Version:     "v1.0.0",
		GitSHA:      "abc123",
		BuildTime:   "2026-06-25T10:00:00Z",
		Environment: "production",
	}
	require.Equal(t, "user-service", r.ServiceName)
	require.Equal(t, "v1.0.0", r.Version)
	require.Equal(t, "abc123", r.GitSHA)
	require.Equal(t, "2026-06-25T10:00:00Z", r.BuildTime)
	require.Equal(t, "production", r.Environment)
}
