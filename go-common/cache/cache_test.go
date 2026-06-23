package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNew_Builds(t *testing.T) {
	c := New[string, int](LRU, 10).Build()
	assert.Equal(t, 0, c.Len())
}

func TestSetAndGet(t *testing.T) {
	c := New[string, string](LRU, 10).Build()
	c.Set("k", "v")
	v, ok, _ := c.Get("k")
	assert.True(t, ok)
	assert.Equal(t, "v", v)
}

func TestGetMissing(t *testing.T) {
	c := New[string, string](LRU, 10).Build()
	_, ok, _ := c.Get("x")
	assert.False(t, ok)
}

func TestSetWithTTL_Expires(t *testing.T) {
	c := New[string, string](LRU, 10).Build()
	c.SetWithTTL("k", "v", 50*time.Millisecond)

	v, ok, _ := c.Get("k")
	assert.True(t, ok)
	assert.Equal(t, "v", v)

	time.Sleep(100 * time.Millisecond)
	_, ok, _ = c.Get("k")
	assert.False(t, ok, "key should be expired")
}

func TestHas(t *testing.T) {
	c := New[string, int](LRU, 10).Build()
	c.Set("a", 1)
	assert.True(t, c.Has("a"))
	assert.False(t, c.Has("b"))
}

func TestKeys(t *testing.T) {
	c := New[string, int](LRU, 10).Build()
	c.Set("a", 1)
	c.Set("b", 2)
	assert.Len(t, c.Keys(), 2)
}

func TestDelete(t *testing.T) {
	c := New[string, int](LRU, 10).Build()
	c.Set("a", 1)
	assert.True(t, c.Delete("a"))
	assert.False(t, c.Has("a"))
	assert.Equal(t, 0, c.Len())
}

func TestPurge(t *testing.T) {
	c := New[string, int](LRU, 10).Build()
	c.Set("a", 1)
	c.Set("b", 2)
	c.Purge()
	assert.Equal(t, 0, c.Len())
}

func TestLen(t *testing.T) {
	c := New[string, int](LRU, 10).Build()
	c.Set("a", 1)
	assert.Equal(t, 1, c.Len())
	c.Set("b", 2)
	assert.Equal(t, 2, c.Len())
	c.Delete("a")
	assert.Equal(t, 1, c.Len())
}

func TestFIFOEviction(t *testing.T) {
	c := New[string, int](FIFO, 3).Build()
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	c.Set("d", 4)
	assert.False(t, c.Has("a"), "FIFO: oldest key should be evicted")
	assert.True(t, c.Has("d"))
	assert.Equal(t, 3, c.Len())
}

func TestLRUEviction(t *testing.T) {
	c := New[string, int](LRU, 3).Build()
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	// Access "a" to make it recently used
	c.Get("a")
	c.Set("d", 4)
	assert.False(t, c.Has("b"), "LRU: least recently used should be evicted")
	assert.True(t, c.Has("a"), "LRU: accessed key should stay")
	assert.True(t, c.Has("d"))
}

func TestWithTTL(t *testing.T) {
	c := New[string, string](LRU, 10).WithTTL(100 * time.Millisecond).Build()
	c.Set("k", "v")
	_, ok, _ := c.Get("k")
	assert.True(t, ok)
	time.Sleep(150 * time.Millisecond)
	_, ok, _ = c.Get("k")
	assert.False(t, ok)
}

func TestPeek(t *testing.T) {
	c := New[string, string](LRU, 10).Build()
	c.Set("k", "v")
	v, ok := c.Peek("k")
	assert.True(t, ok)
	assert.Equal(t, "v", v)
}

func TestPurgeThenReuse(t *testing.T) {
	c := New[string, int](LRU, 10).Build()
	c.Set("a", 1)
	c.Purge()
	c.Set("b", 2)
	v, ok, _ := c.Get("b")
	assert.True(t, ok)
	assert.Equal(t, 2, v)
	assert.Equal(t, 1, c.Len())
}
