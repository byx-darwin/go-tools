package auth

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/byx-darwin/go-tools/go-auth/device"
	"github.com/samber/hot"
	"github.com/samber/oops"
)

// compile-time interface check.
var _ device.Store = (*MemoryDeviceStore)(nil)

// deviceKey 是设备缓存的复合键。
type deviceKey struct {
	UserUUID string
	DeviceID string
}

// MemoryDeviceStore 基于内存的设备会话存储实现。
//
// 使用 samber/hot 缓存存储单个设备，sync.RWMutex 保护跨键操作（如 AddDevice 的 maxDevices 限制）。
// 适用于开发和测试环境，不适合生产使用。
type MemoryDeviceStore struct {
	cache *hot.HotCache[deviceKey, *device.Device]
	mu    sync.RWMutex
	ttl   time.Duration
	cfg   config
}

// NewMemoryDeviceStore 创建内存设备存储。
//
// 默认配置：
//   - deviceTTL: 30 天
//   - maxDevices: 5
//   - cacheSize: 1024
func NewMemoryDeviceStore(opts ...Option) *MemoryDeviceStore {
	cfg := applyDefaults(opts)
	return &MemoryDeviceStore{
		cache: hot.NewHotCache[deviceKey, *device.Device](hot.LRU, cfg.cacheSize).Build(),
		ttl:   cfg.deviceTTL,
		cfg:   cfg,
	}
}

// AddDevice 注册新设备并返回被踢出的设备。
//
// 当用户设备数超过 maxDevices 限制时，踢出 CreatedAt 最早的旧设备。
// 返回值是被踢出的设备列表，列表为空表示没有设备被踢出。
func (s *MemoryDeviceStore) AddDevice(_ context.Context, userUUID, deviceID, jti string, maxDevices int) ([]device.Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// maxDevices <= 0 means no limit.

	key := deviceKey{UserUUID: userUUID, DeviceID: deviceID}

	// 如果设备已存在，先删除再重新添加（刷新 JTI 和 TTL）。
	if _, ok, _ := s.cache.Get(key); ok {
		s.cache.Delete(key)
	}

	devices := s.listByUser(userUUID)

	var kicked []device.Device

	if maxDevices > 0 && len(devices) >= maxDevices {
		kicked = s.kickOldest(userUUID, devices, len(devices)-maxDevices+1)
	}

	now := time.Now()
	newDevice := &device.Device{
		DeviceID:  deviceID,
		JTI:       jti,
		UserUUID:  userUUID,
		CreatedAt: now,
	}
	s.cache.SetWithTTL(key, newDevice, s.ttl)

	return kicked, nil
}

// CheckDevice 检查指定设备会话是否有效。
//
// 返回 true 表示该设备会话有效（存在且 JTI 匹配）。
func (s *MemoryDeviceStore) CheckDevice(_ context.Context, userUUID, deviceID, jti string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := deviceKey{UserUUID: userUUID, DeviceID: deviceID}
	dev, ok, err := s.cache.Get(key)
	if err != nil {
		return false, oops.Wrapf(err, "device check")
	}
	if !ok {
		return false, nil
	}
	return dev.JTI == jti, nil
}

// RemoveDevice 移除指定设备的会话。
func (s *MemoryDeviceStore) RemoveDevice(_ context.Context, userUUID, deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache.Delete(deviceKey{UserUUID: userUUID, DeviceID: deviceID})
	return nil
}

// RemoveAllDevices 移除用户的所有设备会话（比如修改密码后）。
func (s *MemoryDeviceStore) RemoveAllDevices(_ context.Context, userUUID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, key := range s.cache.Keys() {
		if key.UserUUID == userUUID {
			s.cache.Delete(key)
		}
	}
	return nil
}

// ListDevices 列出用户的所有已注册设备。
func (s *MemoryDeviceStore) ListDevices(_ context.Context, userUUID string) ([]device.Device, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	devices := s.listByUser(userUUID)
	if len(devices) == 0 {
		return nil, nil
	}
	return devices, nil
}

// listByUser 获取用户的设备列表（调用方需持锁）。
func (s *MemoryDeviceStore) listByUser(userUUID string) []device.Device {
	var devices []device.Device
	for _, key := range s.cache.Keys() {
		if key.UserUUID == userUUID {
			dev, ok, _ := s.cache.Get(key)
			if ok {
				devices = append(devices, *dev)
			}
		}
	}
	return devices
}

// kickOldest 踢出最旧的 N 个设备（调用方需持锁）。
func (s *MemoryDeviceStore) kickOldest(userUUID string, devices []device.Device, n int) []device.Device {
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].CreatedAt.Before(devices[j].CreatedAt)
	})

	kicked := make([]device.Device, 0, n)
	for i := 0; i < n && i < len(devices); i++ {
		s.cache.Delete(deviceKey{UserUUID: userUUID, DeviceID: devices[i].DeviceID})
		kicked = append(kicked, devices[i])
	}
	return kicked
}
