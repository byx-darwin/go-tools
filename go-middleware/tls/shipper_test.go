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
