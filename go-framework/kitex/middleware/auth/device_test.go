package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gopkg/cloud/metainfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/byx-darwin/go-tools/go-auth/device"
)

// memoryDeviceStore 是仅用于测试的最小 device.Store 实现。
type memoryDeviceStore struct {
	valid     bool
	checkErr  error
	lastCheck [3]string // userUUID, deviceID, jti
}

func (m *memoryDeviceStore) AddDevice(_ context.Context, _, _, _ string, _ int) ([]device.Device, error) {
	return nil, nil
}
func (m *memoryDeviceStore) CheckDevice(_ context.Context, userUUID, deviceID, jti string) (bool, error) {
	m.lastCheck = [3]string{userUUID, deviceID, jti}
	if m.checkErr != nil {
		return false, m.checkErr
	}
	return m.valid, nil
}
func (m *memoryDeviceStore) RemoveDevice(_ context.Context, _, _ string) error  { return nil }
func (m *memoryDeviceStore) RemoveAllDevices(_ context.Context, _ string) error { return nil }
func (m *memoryDeviceStore) ListDevices(_ context.Context, _ string) ([]device.Device, error) {
	return nil, nil
}

func withDeviceMetainfo(ctx context.Context, userUUID, deviceID, jti string) context.Context {
	if userUUID != "" {
		ctx = metainfo.WithPersistentValue(ctx, metaKeyDeviceUserUUID, userUUID)
	}
	if deviceID != "" {
		ctx = metainfo.WithPersistentValue(ctx, metaKeyDeviceID, deviceID)
	}
	if jti != "" {
		ctx = metainfo.WithPersistentValue(ctx, metaKeyDeviceJTI, jti)
	}
	return metainfo.TransferForward(ctx)
}

func TestDeviceAuthServer_Success(t *testing.T) {
	store := &memoryDeviceStore{valid: true}
	mw := DeviceAuthServer(store)

	ctx := withDeviceMetainfo(context.Background(), "user-1", "device-1", "jti-1")
	err := mw(noopEndpoint(nil))(ctx, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, [3]string{"user-1", "device-1", "jti-1"}, store.lastCheck)
}

func TestDeviceAuthServer_IncompleteIdentity(t *testing.T) {
	store := &memoryDeviceStore{valid: true}
	mw := DeviceAuthServer(store)

	ctx := withDeviceMetainfo(context.Background(), "user-1", "", "jti-1")
	err := mw(noopEndpoint(nil))(ctx, nil, nil)
	require.Error(t, err)
}

func TestDeviceAuthServer_Kicked(t *testing.T) {
	store := &memoryDeviceStore{valid: false}
	mw := DeviceAuthServer(store)

	ctx := withDeviceMetainfo(context.Background(), "user-1", "device-1", "jti-1")
	err := mw(noopEndpoint(nil))(ctx, nil, nil)
	require.Error(t, err)
}

func TestDeviceAuthServer_StoreError(t *testing.T) {
	store := &memoryDeviceStore{checkErr: errors.New("store down")}
	mw := DeviceAuthServer(store)

	ctx := withDeviceMetainfo(context.Background(), "user-1", "device-1", "jti-1")
	err := mw(noopEndpoint(nil))(ctx, nil, nil)
	require.Error(t, err)
}

func TestDeviceAuthClient_InjectsMetainfo(t *testing.T) {
	extract := func(ctx context.Context) (string, string, string, bool) {
		return "user-1", "device-1", "jti-1", true
	}
	mw := DeviceAuthClient(extract)

	var gotCtx context.Context
	err := mw(noopEndpoint(&gotCtx))(context.Background(), nil, nil)
	require.NoError(t, err)

	forwarded := metainfo.TransferForward(gotCtx)
	userUUID, ok := metainfo.GetPersistentValue(forwarded, metaKeyDeviceUserUUID)
	require.True(t, ok)
	assert.Equal(t, "user-1", userUUID)
	deviceID, ok := metainfo.GetPersistentValue(forwarded, metaKeyDeviceID)
	require.True(t, ok)
	assert.Equal(t, "device-1", deviceID)
	jti, ok := metainfo.GetPersistentValue(forwarded, metaKeyDeviceJTI)
	require.True(t, ok)
	assert.Equal(t, "jti-1", jti)
}

func TestDeviceAuthClient_NilExtractor(t *testing.T) {
	mw := DeviceAuthClient(nil)
	err := mw(noopEndpoint(nil))(context.Background(), nil, nil)
	require.Error(t, err)
}

func TestDeviceAuthClient_ExtractorIncomplete(t *testing.T) {
	extract := func(ctx context.Context) (string, string, string, bool) {
		return "user-1", "", "", true
	}
	mw := DeviceAuthClient(extract)
	err := mw(noopEndpoint(nil))(context.Background(), nil, nil)
	require.Error(t, err)
}
