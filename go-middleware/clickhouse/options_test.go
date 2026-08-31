package clickhouse

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientOption_WithTrace(t *testing.T) {
	o := &clientOptions{}
	WithTrace()(o)
	assert.True(t, o.trace)
}
