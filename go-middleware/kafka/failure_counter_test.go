package kafka

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMemFailureCounter_IncrReturnsRunningCount(t *testing.T) {
	c := newMemFailureCounter()
	assert.Equal(t, 1, c.Incr("k1"))
	assert.Equal(t, 2, c.Incr("k1"))
	assert.Equal(t, 3, c.Incr("k1"))
}

func TestMemFailureCounter_KeysAreIsolated(t *testing.T) {
	c := newMemFailureCounter()
	assert.Equal(t, 1, c.Incr("k1"))
	assert.Equal(t, 1, c.Incr("k2"))
	assert.Equal(t, 2, c.Incr("k1"))
}

func TestMemFailureCounter_ResetClearsCount(t *testing.T) {
	c := newMemFailureCounter()
	c.Incr("k1")
	c.Incr("k1")
	c.Reset("k1")
	assert.Equal(t, 1, c.Incr("k1"))
}

func TestMemFailureCounter_ResetUnknownKeyNoop(t *testing.T) {
	c := newMemFailureCounter()
	c.Reset("unknown")
	assert.Equal(t, 1, c.Incr("unknown"))
}

func TestMemFailureCounter_ConcurrentIncrSameKey(t *testing.T) {
	c := newMemFailureCounter()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Incr("k1")
		}()
	}
	wg.Wait()
	assert.Equal(t, 101, c.Incr("k1"))
}
