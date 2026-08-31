package tls

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseJSONLine(t *testing.T) {
	line := []byte(`{"time":"2024-01-01T00:00:00Z","level":"INFO","msg":"hello","key":"val"}`)
	fields := parseJSONLine(line)
	assert.Equal(t, "INFO", fields["level"])
	assert.Equal(t, "hello", fields["msg"])
	assert.Equal(t, "val", fields["key"])
}

func TestParseJSONLine_Invalid(t *testing.T) {
	assert.Nil(t, parseJSONLine([]byte(`not json`)))
}

func TestParseJSONLine_Empty(t *testing.T) {
	assert.Nil(t, parseJSONLine([]byte{}))
}

func TestNewFileShipper_RequiresFilePath(t *testing.T) {
	_, err := NewFileShipper(FileShipperConfig{
		ProducerConfig: ProducerConfig{
			Endpoint:        "tls.example.com",
			Region:          "cn-beijing",
			TopicID:         "t",
			AccessKeyID:     "ak",
			AccessKeySecret: "sk",
		},
	})
	assert.ErrorContains(t, err, "file_path")
}

func TestFileShipper_Success(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/app.log"
	require.NoError(t, os.WriteFile(path, []byte(`{"level":"info","msg":"line1"}`+"\n"), 0o644))

	shipper, err := NewFileShipper(FileShipperConfig{
		ProducerConfig: ProducerConfig{
			Endpoint:        "tls.example.com",
			AccessKeyID:     "ak",
			AccessKeySecret: "sk",
			Region:          "cn-beijing",
			TopicID:         "topic-123",
			Source:          "test",
		},
		FilePath:      path,
		CheckInterval: 100 * time.Millisecond,
	})
	require.NoError(t, err)

	shipper.Start()
	// Don't wait for tail loop — Close() will drain and return
	_ = shipper.Close()
}

func TestFileShipper_Defaults(t *testing.T) {
	shipper, _ := NewFileShipper(FileShipperConfig{
		ProducerConfig: ProducerConfig{
			Endpoint:        "tls.example.com",
			Region:          "cn-beijing",
			TopicID:         "t",
			AccessKeyID:     "ak",
			AccessKeySecret: "sk",
			FlushInterval:   1 * time.Hour,
			BatchSize:       9999,
		},
		FilePath:      "/tmp/test.log",
		CheckInterval: 100 * time.Millisecond,
	})
	assert.Equal(t, 100*time.Millisecond, shipper.config.CheckInterval)
	assert.Equal(t, 64*1024, shipper.config.MaxLineSize)
	_ = shipper.Close()
}

func TestFileShipper_ShipSince_StopsBeforeFailedLine(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/app.log"
	line1 := `{"level":"info","msg":"line1"}` + "\n"
	line2 := `{"level":"info","msg":"line2"}` + "\n"
	require.NoError(t, os.WriteFile(path, []byte(line1+line2), 0o644))

	// BatchSize=1 makes every SendLog call flush immediately, and the fake
	// endpoint makes that flush fail — so the very first line should fail.
	shipper, err := NewFileShipper(FileShipperConfig{
		ProducerConfig: ProducerConfig{
			Endpoint:        "tls.example.com",
			AccessKeyID:     "ak",
			AccessKeySecret: "sk",
			Region:          "cn-beijing",
			TopicID:         "topic-123",
			BatchSize:       1,
			FlushInterval:   time.Hour,
		},
		FilePath:      path,
		CheckInterval: time.Hour,
	})
	require.NoError(t, err)
	defer func() { _ = shipper.Close() }()

	newOffset, shipErr := shipper.shipSince(0)
	assert.NoError(t, shipErr)
	// The failed first line must not be skipped: offset stays at 0 so it is
	// retried on the next tick, instead of jumping past it (previous behavior
	// silently dropped the line and advanced to fi.Size()).
	assert.Equal(t, int64(0), newOffset)
}
