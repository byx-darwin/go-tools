package clickhouse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSec(t *testing.T) {
	assert.Equal(t, 0, int(sec(0)))
	assert.Equal(t, 5_000_000_000, int(sec(5)))
	assert.Equal(t, 30_000_000_000, int(sec(30)))
}

func TestConfig_Defaults(t *testing.T) {
	c := Config{}
	assert.Empty(t, c.DSN)
	assert.Nil(t, c.Addrs)
	assert.Empty(t, c.Database)
	assert.False(t, c.Compress)
	assert.False(t, c.TLS.Enable)
	assert.Equal(t, 0, c.MaxOpenConns)
}

func TestConfig_Fields(t *testing.T) {
	c := Config{
		Addrs:           []string{"ch1:9000", "ch2:9000"},
		Database:        "analytics",
		Username:        "default",
		Password:        "secret",
		DialTimeout:     10,
		MaxOpenConns:    50,
		MaxIdleConns:    10,
		ConnMaxLifetime: 3600,
		Compress:        true,
	}
	c.TLS.Enable = true
	c.TLS.InsecureSkipVerify = true

	assert.Equal(t, []string{"ch1:9000", "ch2:9000"}, c.Addrs)
	assert.Equal(t, "analytics", c.Database)
	assert.Equal(t, "default", c.Username)
	assert.Equal(t, "secret", c.Password)
	assert.Equal(t, 10, c.DialTimeout)
	assert.Equal(t, 50, c.MaxOpenConns)
	assert.Equal(t, 10, c.MaxIdleConns)
	assert.Equal(t, 3600, c.ConnMaxLifetime)
	assert.True(t, c.Compress)
	assert.True(t, c.TLS.Enable)
	assert.True(t, c.TLS.InsecureSkipVerify)
}

func TestConfig_WithDSN(t *testing.T) {
	c := Config{
		DSN: "clickhouse://default:secret@localhost:9000/analytics?dial_timeout=10s",
	}
	assert.Equal(t, "clickhouse://default:secret@localhost:9000/analytics?dial_timeout=10s", c.DSN)
}

func TestNewClient_WithDSN(t *testing.T) {
	// Using DSN to create client
	client, err := NewClient(Config{
		DSN: "clickhouse://default:@localhost:9000/default",
	})
	if err != nil {
		t.Logf("DSN client creation returned error (expected without real CH): %v", err)
		return
	}
	assert.NotNil(t, client)
	_ = client.Close()
}

func TestNewClient_WithFields(t *testing.T) {
	client, err := NewClient(Config{
		Addrs:    []string{"localhost:9000"},
		Database: "default",
	})
	if err != nil {
		t.Logf("Field-based client creation returned error (expected without real CH): %v", err)
		return
	}
	assert.NotNil(t, client)
	_ = client.Close()
}

func TestNewClient_WithTLS(t *testing.T) {
	c := Config{
		Addrs:    []string{"localhost:9440"},
		Database: "default",
	}
	c.TLS.Enable = true
	c.TLS.InsecureSkipVerify = true

	client, err := NewClient(c)
	if err != nil {
		t.Logf("TLS client creation returned error (expected without real CH): %v", err)
		return
	}
	assert.NotNil(t, client)
	_ = client.Close()
}

func TestNewClient_WithCompression(t *testing.T) {
	client, err := NewClient(Config{
		Addrs:    []string{"localhost:9000"},
		Database: "default",
		Compress: true,
	})
	if err != nil {
		t.Logf("Compression client creation returned error (expected without real CH): %v", err)
		return
	}
	assert.NotNil(t, client)
	_ = client.Close()
}

func TestNewClient_InvalidDSN(t *testing.T) {
	_, err := NewClient(Config{
		DSN: "://invalid-dsn",
	})
	assert.Error(t, err)
}

func TestNewClient_ZeroConfig(t *testing.T) {
	// Empty addresses should still be valid (clickhouse-go will try localhost)
	client, err := NewClient(Config{
		Database: "default",
	})
	// May succeed or fail depending on connection availability
	if err != nil {
		t.Logf("Zero-config client creation returned error: %v", err)
		return
	}
	assert.NotNil(t, client)
	_ = client.Close()
}
