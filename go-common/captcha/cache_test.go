package captcha

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewCacheStore_Defaults(t *testing.T) {
	store := NewCacheStore()
	if store == nil {
		t.Fatal("NewCacheStore returned nil")
	}
	if store.Expiration() != 5*time.Minute {
		t.Errorf("default Expiration = %v, want 5m", store.Expiration())
	}
	if store.PreKey() != "CAPTCHA_" {
		t.Errorf("default PreKey = %q, want CAPTCHA_", store.PreKey())
	}
}

func TestNewCacheStore_WithOptions(t *testing.T) {
	store := NewCacheStore(
		WithCapacity(100),
		WithExpiration(30*time.Second),
		WithPreKey("TEST_"),
	)
	if store.Expiration() != 30*time.Second {
		t.Errorf("Expiration = %v, want 30s", store.Expiration())
	}
	if store.PreKey() != "TEST_" {
		t.Errorf("PreKey = %q, want TEST_", store.PreKey())
	}
}

func TestNewCacheStoreWithTTL_Compat(t *testing.T) {
	store := NewCacheStoreWithTTL(10, 30*time.Second)
	if store.Expiration() != 30*time.Second {
		t.Errorf("Expiration = %v, want 30s", store.Expiration())
	}
}

func TestNewCacheStoreWithConfig_Compat(t *testing.T) {
	store := NewCacheStoreWithConfig(100, 30*time.Second, "MY_")
	if store.Expiration() != 30*time.Second {
		t.Errorf("Expiration = %v, want 30s", store.Expiration())
	}
	if store.PreKey() != "MY_" {
		t.Errorf("PreKey = %q, want MY_", store.PreKey())
	}
}

func TestCacheStoreSetAndGet(t *testing.T) {
	store := NewCacheStore(WithCapacity(10))

	err := store.Set("testid", "ABCD")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val := store.Get("testid", false)
	if val != "ABCD" {
		t.Errorf("Get returned %q, want %q", val, "ABCD")
	}
}

func TestCacheStoreGetClear(t *testing.T) {
	store := NewCacheStore(WithCapacity(10))

	require.NoError(t, store.Set("testid", "XYZ"))
	val := store.Get("testid", true)

	if val != "XYZ" {
		t.Errorf("Get returned %q, want %q", val, "XYZ")
	}

	val2 := store.Get("testid", false)
	if val2 != "" {
		t.Errorf("Get after clear should return empty, got %q", val2)
	}
}

func TestCacheStoreGetNoClear(t *testing.T) {
	store := NewCacheStore(WithCapacity(10))

	require.NoError(t, store.Set("testid", "DATA"))
	val := store.Get("testid", false)

	if val != "DATA" {
		t.Errorf("Get returned %q, want %q", val, "DATA")
	}

	val2 := store.Get("testid", false)
	if val2 != "DATA" {
		t.Errorf("Get without clear should still return value, got %q", val2)
	}
}

func TestCacheStoreVerify(t *testing.T) {
	store := NewCacheStore(WithCapacity(10))

	require.NoError(t, store.Set("id1", "A1B2"))
	if !store.Verify("id1", "A1B2", false) {
		t.Error("Verify should return true for correct answer")
	}
	if store.Verify("id1", "WRONG", false) {
		t.Error("Verify should return false for wrong answer")
	}
}

func TestCacheStoreVerifyClear(t *testing.T) {
	store := NewCacheStore(WithCapacity(10))

	require.NoError(t, store.Set("id1", "S3CR3T"))
	if !store.Verify("id1", "S3CR3T", true) {
		t.Error("first Verify should succeed")
	}
	if store.Verify("id1", "S3CR3T", false) {
		t.Error("Verify after clear should fail (key gone)")
	}
}

func TestCacheStoreGetMissingKey(t *testing.T) {
	store := NewCacheStore(WithCapacity(10))

	val := store.Get("nonexistent", false)
	if val != "" {
		t.Errorf("Get for missing key should return empty, got %q", val)
	}
}

func TestCacheStorePreKeyPrefix(t *testing.T) {
	store := NewCacheStore(WithCapacity(10), WithPreKey("TEST_"))

	require.NoError(t, store.Set("abc", "123"))
	val := store.Get("abc", false)
	if val != "123" {
		t.Errorf("Get with same id should return stored value, got %q", val)
	}
	if store.PreKey() != "TEST_" {
		t.Errorf("PreKey = %q, want TEST_", store.PreKey())
	}
}

func TestCacheStoreExpiration(t *testing.T) {
	store := NewCacheStore(WithCapacity(10))
	store.SetExpiration(50 * time.Millisecond)

	require.NoError(t, store.Set("exp", "value"))
	val := store.Get("exp", false)
	if val != "value" {
		t.Fatal("value should exist before expiry")
	}

	time.Sleep(100 * time.Millisecond)
	val = store.Get("exp", false)
	if val != "" {
		t.Error("value should be expired, got", val)
	}
}

func TestSetPreKey(t *testing.T) {
	store := NewCacheStore()
	if store.PreKey() != "CAPTCHA_" {
		t.Fatalf("default PreKey = %q, want CAPTCHA_", store.PreKey())
	}

	store.SetPreKey("ADMIN_")
	if store.PreKey() != "ADMIN_" {
		t.Errorf("PreKey after SetPreKey = %q, want ADMIN_", store.PreKey())
	}

	// 空字符串不应改变前缀
	store.SetPreKey("")
	if store.PreKey() != "ADMIN_" {
		t.Errorf("empty SetPreKey should not change PreKey, got %q", store.PreKey())
	}
}

func TestUpdate(t *testing.T) {
	store := NewCacheStore()
	store.Update(
		WithPreKey("UPDATED_"),
		WithExpiration(10*time.Second),
	)
	if store.PreKey() != "UPDATED_" {
		t.Errorf("PreKey = %q, want UPDATED_", store.PreKey())
	}
	if store.Expiration() != 10*time.Second {
		t.Errorf("Expiration = %v, want 10s", store.Expiration())
	}
}

func TestGetAndDelete(t *testing.T) {
	store := NewCacheStore(WithCapacity(10))
	require.NoError(t, store.Set("k1", "v1"))

	val, ok := store.GetAndDelete("k1")
	if !ok || val != "v1" {
		t.Errorf("GetAndDelete = (%q, %v), want (v1, true)", val, ok)
	}

	// 第二次应该找不到
	_, ok = store.GetAndDelete("k1")
	if ok {
		t.Error("second GetAndDelete should return false")
	}
}

func TestClear(t *testing.T) {
	store := NewCacheStore(WithCapacity(10))
	require.NoError(t, store.Set("a", "1"))
	require.NoError(t, store.Set("b", "2"))

	store.Clear()
	if store.Len() != 0 {
		t.Errorf("Len after Clear = %d, want 0", store.Len())
	}
}
