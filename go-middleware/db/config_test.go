package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Defaults(t *testing.T) {
	c := &Config{}
	assert.Equal(t, 0, c.MaxOpenCons)
	assert.Equal(t, 0, c.MaxIdleCons)
	assert.Empty(t, c.Driver)
	assert.False(t, c.SqlLog)
}

func TestConfig_Fields(t *testing.T) {
	c := Config{
		Driver:         "mysql",
		Source:         "root:password@tcp(localhost:3306)/test",
		Name:           "testdb",
		TablePrefix:    "t_",
		SqlLog:         true,
		MaxOpenCons:    100,
		MaxIdleCons:    10,
		ConMaxLifetime: 3600,
		MaxIdleTime:    600,
	}

	assert.Equal(t, "mysql", c.Driver)
	assert.Equal(t, "root:password@tcp(localhost:3306)/test", c.Source)
	assert.Equal(t, "testdb", c.Name)
	assert.Equal(t, "t_", c.TablePrefix)
	assert.True(t, c.SqlLog)
	assert.Equal(t, 100, c.MaxOpenCons)
	assert.Equal(t, 10, c.MaxIdleCons)
	assert.Equal(t, 3600, c.ConMaxLifetime)
	assert.Equal(t, 600, c.MaxIdleTime)
}

func TestConfig_Drivers(t *testing.T) {
	drivers := []string{"mysql", "postgres", "sqlite3", "mongodb"}
	for _, d := range drivers {
		c := Config{Driver: d}
		assert.Equal(t, d, c.Driver)
	}
}
