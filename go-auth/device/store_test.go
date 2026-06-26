package device

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDeviceStruct 验证 Device 结构体字段存在且类型正确。
func TestDeviceStruct(t *testing.T) {
	now := time.Now()
	d := Device{
		DeviceID:  "device-123",
		JTI:       "jti-abc",
		UserUUID:  "user-456",
		CreatedAt: now,
	}

	assert.Equal(t, "device-123", d.DeviceID)
	assert.Equal(t, "jti-abc", d.JTI)
	assert.Equal(t, "user-456", d.UserUUID)
	assert.Equal(t, now, d.CreatedAt)
}

// TestDeviceZeroValues 验证 Device 零值有效（字段无 nil 风险）。
func TestDeviceZeroValues(t *testing.T) {
	d := Device{}
	assert.Empty(t, d.DeviceID)
	assert.Empty(t, d.JTI)
	assert.Empty(t, d.UserUUID)
	assert.True(t, d.CreatedAt.IsZero())
}

// ── Store 接口编译期检查 ──

type mockStore struct {
	addDeviceFn        func(ctx context.Context, userUUID, deviceID, jti string, maxDevices int) ([]Device, error)
	checkDeviceFn      func(ctx context.Context, userUUID, deviceID, jti string) (bool, error)
	removeDeviceFn     func(ctx context.Context, userUUID, deviceID string) error
	removeAllDevicesFn func(ctx context.Context, userUUID string) error
	listDevicesFn      func(ctx context.Context, userUUID string) ([]Device, error)
}

func (m *mockStore) AddDevice(ctx context.Context, userUUID, deviceID, jti string, maxDevices int) ([]Device, error) {
	return m.addDeviceFn(ctx, userUUID, deviceID, jti, maxDevices)
}

func (m *mockStore) CheckDevice(ctx context.Context, userUUID, deviceID, jti string) (bool, error) {
	return m.checkDeviceFn(ctx, userUUID, deviceID, jti)
}

func (m *mockStore) RemoveDevice(ctx context.Context, userUUID, deviceID string) error {
	return m.removeDeviceFn(ctx, userUUID, deviceID)
}

func (m *mockStore) RemoveAllDevices(ctx context.Context, userUUID string) error {
	return m.removeAllDevicesFn(ctx, userUUID)
}

func (m *mockStore) ListDevices(ctx context.Context, userUUID string) ([]Device, error) {
	return m.listDevicesFn(ctx, userUUID)
}

// TestStoreInterface 验证 Store 接口契约。
func TestStoreInterface(t *testing.T) {
	t.Run("AddDevice returns kicked devices", func(t *testing.T) {
		store := &mockStore{
			addDeviceFn: func(_ context.Context, _, _, _ string, _ int) ([]Device, error) {
				return []Device{
					{DeviceID: "old-device", UserUUID: "user-1"},
				}, nil
			},
		}
		devices, err := store.AddDevice(context.Background(), "user-1", "new-device", "jti-new", 1)
		assert.NoError(t, err)
		assert.Len(t, devices, 1)
		assert.Equal(t, "old-device", devices[0].DeviceID)
	})

	t.Run("AddDevice returns empty when within limit", func(t *testing.T) {
		store := &mockStore{
			addDeviceFn: func(_ context.Context, _, _, _ string, _ int) ([]Device, error) {
				return nil, nil
			},
		}
		devices, err := store.AddDevice(context.Background(), "user-1", "new-device", "jti-new", 5)
		assert.NoError(t, err)
		assert.Empty(t, devices)
	})

	t.Run("AddDevice returns error", func(t *testing.T) {
		store := &mockStore{
			addDeviceFn: func(_ context.Context, _, _, _ string, _ int) ([]Device, error) {
				return nil, errors.New("storage error")
			},
		}
		devices, err := store.AddDevice(context.Background(), "user-1", "d", "j", 1)
		assert.Error(t, err)
		assert.Nil(t, devices)
	})

	t.Run("CheckDevice returns true for valid session", func(t *testing.T) {
		store := &mockStore{
			checkDeviceFn: func(_ context.Context, _, _, jti string) (bool, error) {
				return jti == "jti-valid", nil
			},
		}
		ok, err := store.CheckDevice(context.Background(), "user-1", "device-1", "jti-valid")
		assert.NoError(t, err)
		assert.True(t, ok)

		ok, err = store.CheckDevice(context.Background(), "user-1", "device-1", "jti-invalid")
		assert.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("CheckDevice returns error", func(t *testing.T) {
		store := &mockStore{
			checkDeviceFn: func(_ context.Context, _, _, _ string) (bool, error) {
				return false, errors.New("check error")
			},
		}
		ok, err := store.CheckDevice(context.Background(), "user-1", "d", "j")
		assert.Error(t, err)
		assert.False(t, ok)
	})

	t.Run("RemoveDevice succeeds", func(t *testing.T) {
		var removedUser, removedDevice string
		store := &mockStore{
			removeDeviceFn: func(_ context.Context, userUUID, deviceID string) error {
				removedUser = userUUID
				removedDevice = deviceID
				return nil
			},
		}
		err := store.RemoveDevice(context.Background(), "user-1", "device-1")
		assert.NoError(t, err)
		assert.Equal(t, "user-1", removedUser)
		assert.Equal(t, "device-1", removedDevice)
	})

	t.Run("RemoveDevice returns error", func(t *testing.T) {
		store := &mockStore{
			removeDeviceFn: func(_ context.Context, _, _ string) error {
				return errors.New("remove error")
			},
		}
		err := store.RemoveDevice(context.Background(), "user-1", "d")
		assert.Error(t, err)
	})

	t.Run("RemoveAllDevices succeeds", func(t *testing.T) {
		var removedUser string
		store := &mockStore{
			removeAllDevicesFn: func(_ context.Context, userUUID string) error {
				removedUser = userUUID
				return nil
			},
		}
		err := store.RemoveAllDevices(context.Background(), "user-1")
		assert.NoError(t, err)
		assert.Equal(t, "user-1", removedUser)
	})

	t.Run("RemoveAllDevices returns error", func(t *testing.T) {
		store := &mockStore{
			removeAllDevicesFn: func(_ context.Context, _ string) error {
				return errors.New("remove all error")
			},
		}
		err := store.RemoveAllDevices(context.Background(), "user-1")
		assert.Error(t, err)
	})

	t.Run("ListDevices returns devices", func(t *testing.T) {
		store := &mockStore{
			listDevicesFn: func(_ context.Context, userUUID string) ([]Device, error) {
				if userUUID == "user-1" {
					return []Device{
						{DeviceID: "device-1", UserUUID: "user-1", CreatedAt: time.Now()},
						{DeviceID: "device-2", UserUUID: "user-1", CreatedAt: time.Now()},
					}, nil
				}
				return nil, nil
			},
		}
		devices, err := store.ListDevices(context.Background(), "user-1")
		assert.NoError(t, err)
		assert.Len(t, devices, 2)

		devices, err = store.ListDevices(context.Background(), "empty-user")
		assert.NoError(t, err)
		assert.Empty(t, devices)
	})

	t.Run("ListDevices returns error", func(t *testing.T) {
		store := &mockStore{
			listDevicesFn: func(_ context.Context, _ string) ([]Device, error) {
				return nil, errors.New("list error")
			},
		}
		devices, err := store.ListDevices(context.Background(), "user-1")
		assert.Error(t, err)
		assert.Nil(t, devices)
	})
}
