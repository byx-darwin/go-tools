package config

import (
	"testing"

	"github.com/polarismesh/polaris-go/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── PolarisOption ──

func TestWithNamespace(t *testing.T) {
	c := &polarisConfig{}
	WithNamespace("prod")(c)
	assert.Equal(t, "prod", c.namespace)
}

func TestWithNamespace_Empty(t *testing.T) {
	c := &polarisConfig{namespace: "keep"}
	WithNamespace("")(c)
	assert.Equal(t, "keep", c.namespace, "空字符串应忽略，不覆盖已有值")
}

func TestWithFileGroup(t *testing.T) {
	c := &polarisConfig{}
	WithFileGroup("myapp")(c)
	assert.Equal(t, "myapp", c.group)
}

func TestWithFileGroup_Empty(t *testing.T) {
	c := &polarisConfig{group: "keep"}
	WithFileGroup("")(c)
	assert.Equal(t, "keep", c.group)
}

func TestWithFileName(t *testing.T) {
	c := &polarisConfig{}
	WithFileName("config.yaml")(c)
	assert.Equal(t, "config.yaml", c.fileName)
}

func TestWithFileName_Empty(t *testing.T) {
	c := &polarisConfig{fileName: "keep.yaml"}
	WithFileName("")(c)
	assert.Equal(t, "keep.yaml", c.fileName)
}

func TestWithChangeListener(t *testing.T) {
	c := &polarisConfig{}
	called := false
	listener := func(event model.ConfigFileChangeEvent) { called = true }
	WithChangeListener(listener)(c)
	require.NotNil(t, c.listener)
	c.listener(model.ConfigFileChangeEvent{})
	assert.True(t, called)
}

func TestWithChangeListener_Nil(t *testing.T) {
	c := &polarisConfig{}
	WithChangeListener(nil)(c)
	assert.Nil(t, c.listener, "nil listener 应被忽略")
}

// ── LoadPolarisConfig ──

// TestLoadPolarisConfig_InitError 验证在没有可用 Polaris 服务器地址配置时，
// polaris.NewConfigAPI() 会因 serverConnector.addresses 为空而失败，
// LoadPolarisConfig 应包装为 ErrPolarisInit 返回。
func TestLoadPolarisConfig_InitError(t *testing.T) {
	_, err := LoadPolarisConfig(
		WithNamespace("default"),
		WithFileGroup("g"),
		WithFileName("f"),
	)
	require.Error(t, err)
}

func TestLoadPolarisConfig_NoOptions(t *testing.T) {
	_, err := LoadPolarisConfig()
	require.Error(t, err)
}

// ── LoadPolarisConfigLegacy ──

func TestLoadPolarisConfigLegacy_InitError(t *testing.T) {
	called := false
	_, err := LoadPolarisConfigLegacy("default", "g", "f", func(event model.ConfigFileChangeEvent) {
		called = true
	})
	require.Error(t, err)
	assert.False(t, called, "配置获取失败时不应触发监听器")
}

func TestLoadPolarisConfigLegacy_NilListener(t *testing.T) {
	_, err := LoadPolarisConfigLegacy("default", "g", "f", nil)
	require.Error(t, err)
}
