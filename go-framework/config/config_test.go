package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// ── Duration ──

func TestDuration_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"30s", 30 * time.Second},
		{"5m", 5 * time.Minute},
		{"1h", 1 * time.Hour},
		{"500ms", 500 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var d Duration
			err := yaml.Unmarshal([]byte(tt.input), &d)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, d.Duration)
		})
	}
}

func TestDuration_UnmarshalYAML_Invalid(t *testing.T) {
	var d Duration
	err := yaml.Unmarshal([]byte("bad"), &d)
	assert.Error(t, err)
}

// ── RegistryOption / JaegerOption ──

func TestRegistryOption(t *testing.T) {
	r := RegistryOption{
		Enable:  true,
		Space:   "prod",
		Name:    "svc",
		Version: "v1",
	}
	assert.True(t, r.Enable)
	assert.Equal(t, "svc", r.Name)
}

func TestJaegerOption(t *testing.T) {
	j := JaegerOption{Enable: true, Endpoint: "http://j:14268"}
	assert.True(t, j.Enable)
}

// ── LoadYAML ──

func TestLoadYAML(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/cfg.yaml"
	require.NoError(t, os.WriteFile(path, []byte("name: test\nport: 8080\n"), 0o644))

	type appCfg struct {
		Name string `yaml:"name"`
		Port int    `yaml:"port"`
	}

	cfg, err := LoadYAML[appCfg](path)
	require.NoError(t, err)
	assert.Equal(t, "test", cfg.Name)
	assert.Equal(t, 8080, cfg.Port)
}

func TestLoadYAML_NotFound(t *testing.T) {
	_, err := LoadYAML[struct{}]("/nonexistent.yaml")
	assert.Error(t, err)
}

func TestMustLoadYAML_OK(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/ok.yaml"
	require.NoError(t, os.WriteFile(path, []byte("x: 1"), 0o644))

	type cfg struct {
		X int `yaml:"x"`
	}
	assert.NotPanics(t, func() {
		c := MustLoadYAML[cfg](path)
		assert.Equal(t, 1, c.X)
	})
}

func TestMustLoadYAML_Panic(t *testing.T) {
	assert.Panics(t, func() {
		MustLoadYAML[struct{}]("/no/such.yaml")
	})
}
