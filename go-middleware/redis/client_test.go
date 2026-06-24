package redis

import (
	"testing"
)

func TestClientOption_WithTrace(t *testing.T) {
	o := &clientOptions{}
	WithTrace()(o)
	if !o.trace {
		t.Error("WithTrace should set trace to true")
	}
}

func TestClientOption_DefaultNoTrace(t *testing.T) {
	o := &clientOptions{}
	// 不应用任何 option，默认不追踪
	if o.trace {
		t.Error("default trace should be false")
	}
}

func TestClientOption_MultipleOptions(t *testing.T) {
	o := &clientOptions{}
	opts := []ClientOption{WithTrace()}
	for _, opt := range opts {
		opt(o)
	}
	if !o.trace {
		t.Error("WithTrace applied via loop should set trace to true")
	}
}
