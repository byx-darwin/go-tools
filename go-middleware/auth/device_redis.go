package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/byx-darwin/go-tools/go-auth/device"
	"github.com/redis/go-redis/v9"
)

// compile-time interface check.
var _ device.Store = (*RedisDeviceStore)(nil)

// deviceEntry 是 Device 的 JSON 序列化结构，存储在 Redis Hash 中。
type deviceEntry struct {
	JTI       string    `json:"jti"`
	CreatedAt time.Time `json:"created_at"`
}

// RedisDeviceStore 基于 Redis 的设备会话存储实现。
//
// 使用 Redis Hash 存储设备信息:
//   - Key: {prefix}device:{userUUID}
//   - Field: {deviceID}
//   - Value: JSON(jti, created_at)
//
// 支持 TTL 自动过期和 maxDevices 限制（超出时按 created_at 踢出最旧设备）。
type RedisDeviceStore struct {
	client redis.UniversalClient
	ttl    time.Duration
	prefix string
}

// NewRedisDeviceStore 创建 Redis 设备存储。
//
// 默认配置：
//   - deviceTTL: 30 天
//   - keyPrefix: ""
func NewRedisDeviceStore(client redis.UniversalClient, opts ...Option) *RedisDeviceStore {
	cfg := applyDefaults(opts)
	return &RedisDeviceStore{
		client: client,
		ttl:    cfg.deviceTTL,
		prefix: cfg.keyPrefix,
	}
}

// deviceKey 构建 Redis Hash key。
func (s *RedisDeviceStore) deviceKey(userUUID string) string {
	return s.prefix + "device:" + userUUID
}

// AddDevice 注册新设备并返回被踢出的设备。
//
// 当用户设备数超过 maxDevices 限制时，踢出 CreatedAt 最早的旧设备。
// 返回值是被踢出的设备列表，列表为空表示没有设备被踢出。
func (s *RedisDeviceStore) AddDevice(ctx context.Context, userUUID, deviceID, jti string, maxDevices int) ([]device.Device, error) {
	key := s.deviceKey(userUUID)

	// 先获取现有设备列表。
	devices, err := s.listDevicesFromHash(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("device add: %w", err)
	}

	// 如果设备已存在，先移除再重新添加（刷新 JTI 和 TTL）。
	filtered := make([]device.Device, 0, len(devices))
	for _, d := range devices {
		if d.DeviceID != deviceID {
			filtered = append(filtered, d)
		}
	}

	var kicked []device.Device

	// 检查是否需要踢出旧设备。
	if maxDevices > 0 && len(filtered) >= maxDevices {
		// 按 CreatedAt 排序，最旧的在前。
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].CreatedAt.Before(filtered[j].CreatedAt)
		})

		kickCount := len(filtered) - maxDevices + 1
		kicked = make([]device.Device, 0, kickCount)
		for i := 0; i < kickCount && i < len(filtered); i++ {
			kicked = append(kicked, filtered[i])
			_ = s.client.HDel(ctx, key, filtered[i].DeviceID).Err()
		}
		// 从 filtered 中移除被踢出的设备。
		filtered = filtered[kickCount:]
	}

	// 保存新设备。
	entry := deviceEntry{
		JTI:       jti,
		CreatedAt: time.Now(),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return nil, fmt.Errorf("device marshal: %w", err)
	}

	if err = s.client.HSet(ctx, key, deviceID, data).Err(); err != nil {
		return nil, fmt.Errorf("device add: %w", err)
	}

	// 刷新整个 Hash 的 TTL。
	if err = s.client.Expire(ctx, key, s.ttl).Err(); err != nil {
		return nil, fmt.Errorf("device expire: %w", err)
	}

	return kicked, nil
}

// CheckDevice 检查指定设备会话是否有效。
//
// 返回 true 表示该设备会话有效（存在且 JTI 匹配）。
func (s *RedisDeviceStore) CheckDevice(ctx context.Context, userUUID, deviceID, jti string) (bool, error) {
	key := s.deviceKey(userUUID)

	data, err := s.client.HGet(ctx, key, deviceID).Bytes()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, fmt.Errorf("device check: %w", err)
	}

	var entry deviceEntry
	if err = json.Unmarshal(data, &entry); err != nil {
		return false, fmt.Errorf("device unmarshal: %w", err)
	}

	return entry.JTI == jti, nil
}

// RemoveDevice 移除指定设备的会话。
func (s *RedisDeviceStore) RemoveDevice(ctx context.Context, userUUID, deviceID string) error {
	key := s.deviceKey(userUUID)

	if err := s.client.HDel(ctx, key, deviceID).Err(); err != nil {
		return fmt.Errorf("device remove: %w", err)
	}

	return nil
}

// RemoveAllDevices 移除用户的所有设备会话（比如修改密码后）。
func (s *RedisDeviceStore) RemoveAllDevices(ctx context.Context, userUUID string) error {
	key := s.deviceKey(userUUID)

	if err := s.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("device remove all: %w", err)
	}

	return nil
}

// ListDevices 列出用户的所有已注册设备。
func (s *RedisDeviceStore) ListDevices(ctx context.Context, userUUID string) ([]device.Device, error) {
	key := s.deviceKey(userUUID)

	devices, err := s.listDevicesFromHash(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("device list: %w", err)
	}

	if len(devices) == 0 {
		return nil, nil
	}

	return devices, nil
}

// listDevicesFromHash 从 Redis Hash 中获取用户的所有设备（内部方法）。
func (s *RedisDeviceStore) listDevicesFromHash(ctx context.Context, key string) ([]device.Device, error) {
	result, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, nil
	}

	devices := make([]device.Device, 0, len(result))
	for deviceID, raw := range result {
		var entry deviceEntry
		if err = json.Unmarshal([]byte(raw), &entry); err != nil {
			return nil, fmt.Errorf("device unmarshal field %s: %w", deviceID, err)
		}
		devices = append(devices, device.Device{
			DeviceID:  deviceID,
			JTI:       entry.JTI,
			CreatedAt: entry.CreatedAt,
		})
	}

	return devices, nil
}
