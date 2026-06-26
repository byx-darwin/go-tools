package auth

import (
	"context"
	"testing"
	"time"

	"github.com/byx-darwin/go-tools/go-auth/device"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryDeviceStore_ImplementsInterface(t *testing.T) {
	// compile-time check (see device_memory.go var _ device.Store = ...)
	var _ device.Store = (*MemoryDeviceStore)(nil)
}

func TestMemoryDeviceStore_New(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		s := NewMemoryDeviceStore()
		assert.NotNil(t, s)
		assert.NotNil(t, s.cache)
		assert.Equal(t, defaultDeviceTTL, s.ttl)
		assert.Equal(t, defaultMaxDevices, s.cfg.maxDevices)
	})

	t.Run("custom options", func(t *testing.T) {
		s := NewMemoryDeviceStore(
			WithDeviceTTL(1*time.Hour),
			WithMaxDevices(3),
			WithCacheSize(256),
		)
		assert.Equal(t, 1*time.Hour, s.ttl)
		assert.Equal(t, 3, s.cfg.maxDevices)
	})
}

func TestMemoryDeviceStore_AddDevice(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryDeviceStore()

	t.Run("add within limit", func(t *testing.T) {
		kicked, err := s.AddDevice(ctx, "user-1", "dev-1", "jti-1", 5)
		require.NoError(t, err)
		assert.Empty(t, kicked)
	})

	t.Run("add same device updates JTI", func(t *testing.T) {
		kicked, err := s.AddDevice(ctx, "user-1", "dev-1", "jti-2", 5)
		require.NoError(t, err)
		assert.Empty(t, kicked)

		ok, err := s.CheckDevice(ctx, "user-1", "dev-1", "jti-2")
		require.NoError(t, err)
		assert.True(t, ok)

		// old JTI should be invalid
		ok, err = s.CheckDevice(ctx, "user-1", "dev-1", "jti-1")
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

func TestMemoryDeviceStore_AddDevice_KickOldest(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryDeviceStore()

	// 添加设备，间隔一小段时间确保 CreatedAt 不同。
	_, err := s.AddDevice(ctx, "user-1", "dev-1", "jti-1", 2)
	require.NoError(t, err)

	time.Sleep(5 * time.Millisecond)

	_, err = s.AddDevice(ctx, "user-1", "dev-2", "jti-2", 2)
	require.NoError(t, err)

	time.Sleep(5 * time.Millisecond)

	// 第三个设备加入，maxDevices=2，应该踢出 dev-1。
	kicked, err := s.AddDevice(ctx, "user-1", "dev-3", "jti-3", 2)
	require.NoError(t, err)
	require.Len(t, kicked, 1)
	assert.Equal(t, "dev-1", kicked[0].DeviceID)

	// dev-1 应该已移除。
	ok, err := s.CheckDevice(ctx, "user-1", "dev-1", "jti-1")
	require.NoError(t, err)
	assert.False(t, ok)

	// dev-2 和 dev-3 应该还在。
	ok, err = s.CheckDevice(ctx, "user-1", "dev-2", "jti-2")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = s.CheckDevice(ctx, "user-1", "dev-3", "jti-3")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestMemoryDeviceStore_AddDevice_MultipleKick(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryDeviceStore()

	for i := 0; i < 3; i++ {
		_, err := s.AddDevice(ctx, "user-1", "dev-"+string(rune('a'+i)), "jti-"+string(rune('a'+i)), 5)
		require.NoError(t, err)
		time.Sleep(5 * time.Millisecond)
	}

	// 添加 2 个新设备，maxDevices=3，应该踢出最旧的 2 个。
	kicked, err := s.AddDevice(ctx, "user-1", "dev-x", "jti-x", 3)
	require.NoError(t, err)
	assert.Len(t, kicked, 1)

	time.Sleep(5 * time.Millisecond)

	kicked, err = s.AddDevice(ctx, "user-1", "dev-y", "jti-y", 3)
	require.NoError(t, err)
	assert.Len(t, kicked, 1)

	// 验证最终只剩 3 个设备。
	devices, err := s.ListDevices(ctx, "user-1")
	require.NoError(t, err)
	assert.Len(t, devices, 3)
}

func TestMemoryDeviceStore_CheckDevice(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryDeviceStore()

	_, err := s.AddDevice(ctx, "user-1", "dev-1", "jti-abc", 5)
	require.NoError(t, err)

	t.Run("valid JTI", func(t *testing.T) {
		ok, err := s.CheckDevice(ctx, "user-1", "dev-1", "jti-abc")
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("invalid JTI", func(t *testing.T) {
		ok, err := s.CheckDevice(ctx, "user-1", "dev-1", "jti-wrong")
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("non-existent device", func(t *testing.T) {
		ok, err := s.CheckDevice(ctx, "user-1", "dev-999", "jti-abc")
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("non-existent user", func(t *testing.T) {
		ok, err := s.CheckDevice(ctx, "user-999", "dev-1", "jti-abc")
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

func TestMemoryDeviceStore_RemoveDevice(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryDeviceStore()

	_, err := s.AddDevice(ctx, "user-1", "dev-1", "jti-1", 5)
	require.NoError(t, err)
	_, err = s.AddDevice(ctx, "user-1", "dev-2", "jti-2", 5)
	require.NoError(t, err)

	require.NoError(t, s.RemoveDevice(ctx, "user-1", "dev-1"))

	ok, err := s.CheckDevice(ctx, "user-1", "dev-1", "jti-1")
	require.NoError(t, err)
	assert.False(t, ok)

	// dev-2 不受影响。
	ok, err = s.CheckDevice(ctx, "user-1", "dev-2", "jti-2")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestMemoryDeviceStore_RemoveDevice_NonExist(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryDeviceStore()

	// 移除不存在的设备不应报错。
	require.NoError(t, s.RemoveDevice(ctx, "user-1", "dev-999"))
}

func TestMemoryDeviceStore_RemoveAllDevices(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryDeviceStore()

	_, err := s.AddDevice(ctx, "user-1", "dev-1", "jti-1", 5)
	require.NoError(t, err)
	_, err = s.AddDevice(ctx, "user-1", "dev-2", "jti-2", 5)
	require.NoError(t, err)
	_, err = s.AddDevice(ctx, "user-2", "dev-3", "jti-3", 5)
	require.NoError(t, err)

	require.NoError(t, s.RemoveAllDevices(ctx, "user-1"))

	// user-1 的设备全部移除。
	devices, err := s.ListDevices(ctx, "user-1")
	require.NoError(t, err)
	assert.Empty(t, devices)

	// user-2 的设备不受影响。
	devices, err = s.ListDevices(ctx, "user-2")
	require.NoError(t, err)
	assert.Len(t, devices, 1)
}

func TestMemoryDeviceStore_RemoveAllDevices_NoDevices(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryDeviceStore()

	// 用户没有任何设备时不应报错。
	require.NoError(t, s.RemoveAllDevices(ctx, "user-empty"))
}

func TestMemoryDeviceStore_ListDevices(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryDeviceStore()

	_, err := s.AddDevice(ctx, "user-1", "dev-1", "jti-1", 5)
	require.NoError(t, err)
	_, err = s.AddDevice(ctx, "user-1", "dev-2", "jti-2", 5)
	require.NoError(t, err)

	devices, err := s.ListDevices(ctx, "user-1")
	require.NoError(t, err)
	assert.Len(t, devices, 2)

	// 空用户返回 nil。
	devices, err = s.ListDevices(ctx, "user-empty")
	require.NoError(t, err)
	assert.Nil(t, devices)
}

func TestMemoryDeviceStore_DeviceTTLExpiry(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryDeviceStore(WithDeviceTTL(200 * time.Millisecond))

	_, err := s.AddDevice(ctx, "user-1", "dev-1", "jti-1", 5)
	require.NoError(t, err)

	ok, err := s.CheckDevice(ctx, "user-1", "dev-1", "jti-1")
	require.NoError(t, err)
	assert.True(t, ok)

	time.Sleep(400 * time.Millisecond)

	ok, err = s.CheckDevice(ctx, "user-1", "dev-1", "jti-1")
	require.NoError(t, err)
	assert.False(t, ok, "device should expire after TTL")
}

func TestMemoryDeviceStore_CrossUserIsolation(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryDeviceStore()

	_, err := s.AddDevice(ctx, "user-1", "dev-1", "jti-1", 5)
	require.NoError(t, err)
	_, err = s.AddDevice(ctx, "user-2", "dev-1", "jti-2", 5)
	require.NoError(t, err)

	// 同名设备不同用户互不影响。
	ok, err := s.CheckDevice(ctx, "user-1", "dev-1", "jti-1")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = s.CheckDevice(ctx, "user-2", "dev-1", "jti-2")
	require.NoError(t, err)
	assert.True(t, ok)

	// 删除 user-1 的设备不影响 user-2。
	require.NoError(t, s.RemoveDevice(ctx, "user-1", "dev-1"))

	ok, err = s.CheckDevice(ctx, "user-2", "dev-1", "jti-2")
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestMemoryDeviceStore_AddDevice_KickWithZeroMax(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryDeviceStore()

	// maxDevices=0 时回退到 cfg.maxDevices（默认 5）。
	for i := 0; i < 10; i++ {
		_, err := s.AddDevice(ctx, "user-1", "dev-"+string(rune('a'+i)), "jti-"+string(rune('a'+i)), 0)
		require.NoError(t, err)
	}

	devices, err := s.ListDevices(ctx, "user-1")
	require.NoError(t, err)
	assert.Len(t, devices, 5)
}
