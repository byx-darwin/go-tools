package es

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Defaults(t *testing.T) {
	c := Config{}
	assert.Nil(t, c.Addresses)
	assert.Empty(t, c.Username)
	assert.Empty(t, c.Password)
	assert.False(t, c.TLS.Enable)
	assert.Equal(t, 0, c.MaxRetries)
	assert.Equal(t, 0, c.MaxIdleConnsPerHost)
}

func TestConfig_Fields(t *testing.T) {
	c := Config{
		Addresses:           []string{"https://es1:9200", "https://es2:9200"},
		Username:            "elastic",
		Password:            "secret",
		APIKey:              "api-key-xxx",
		CloudID:             "my-cloud-id",
		MaxRetries:          5,
		MaxIdleConnsPerHost: 20,
	}
	c.TLS.Enable = true
	c.TLS.InsecureSkipVerify = true

	assert.Equal(t, []string{"https://es1:9200", "https://es2:9200"}, c.Addresses)
	assert.Equal(t, "elastic", c.Username)
	assert.Equal(t, "secret", c.Password)
	assert.Equal(t, "api-key-xxx", c.APIKey)
	assert.Equal(t, "my-cloud-id", c.CloudID)
	assert.Equal(t, 5, c.MaxRetries)
	assert.Equal(t, 20, c.MaxIdleConnsPerHost)
	assert.True(t, c.TLS.Enable)
	assert.True(t, c.TLS.InsecureSkipVerify)
}

func TestNewClient_Basic(t *testing.T) {
	client, err := NewClient(Config{
		Addresses: []string{"http://localhost:9200"},
	})
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_WithAuth(t *testing.T) {
	client, err := NewClient(Config{
		Addresses: []string{"http://localhost:9200"},
		Username:  "elastic",
		Password:  "changeme",
	})
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_WithAPIKey(t *testing.T) {
	client, err := NewClient(Config{
		Addresses: []string{"http://localhost:9200"},
		APIKey:    "base64-encoded-api-key",
	})
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_WithCloudID(t *testing.T) {
	client, err := NewClient(Config{
		CloudID:  "my-cluster:ZXVyLXdlc3QtMS5hd3MuZWxhc3RpYy5jbzo0NDMk",
		Username: "elastic",
		Password: "secret",
	})
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_WithTLS(t *testing.T) {
	c := Config{
		Addresses: []string{"https://localhost:9200"},
	}
	c.TLS.Enable = true
	c.TLS.InsecureSkipVerify = true

	client, err := NewClient(c)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_WithMaxIdleConns(t *testing.T) {
	client, err := NewClient(Config{
		Addresses:           []string{"http://localhost:9200"},
		MaxIdleConnsPerHost: 10,
		MaxRetries:          5,
	})
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClient_Defaults(t *testing.T) {
	// Empty config should still create a client (defaults to localhost)
	client, err := NewClient(Config{})
	assert.NoError(t, err)
	assert.NotNil(t, client)
}
