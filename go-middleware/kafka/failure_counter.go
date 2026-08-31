package kafka

import (
	"sync"
	"sync/atomic"
)

// FailureCounter 记录并查询按 key 维度的失败次数，供 Consumer.HandleMessage
// 判断是否达到 DLQ 转发阈值。默认实现为进程内存级（memFailureCounter），可通过
// WithFailureCounter 替换为跨实例存储实现（如 Redis，本包不提供）。
type FailureCounter interface {
	// Incr 递增 key 的失败计数并返回递增后的值。
	Incr(key string) int
	// Reset 清零 key 的失败计数（成功处理或已转发 DLQ 后调用）。
	Reset(key string)
}

// memFailureCounter 是 FailureCounter 的进程内存实现，跟随 Consumer 生命周期，
// 不做跨实例/跨进程累计。
type memFailureCounter struct {
	counts sync.Map // map[string]*int64
}

// newMemFailureCounter 创建内存失败计数器。
func newMemFailureCounter() *memFailureCounter {
	return &memFailureCounter{}
}

// Incr 递增 key 的失败计数并返回递增后的值。
func (c *memFailureCounter) Incr(key string) int {
	v, _ := c.counts.LoadOrStore(key, new(int64))
	n := atomic.AddInt64(v.(*int64), 1)
	return int(n)
}

// Reset 清零 key 的失败计数。
func (c *memFailureCounter) Reset(key string) {
	c.counts.Delete(key)
}
