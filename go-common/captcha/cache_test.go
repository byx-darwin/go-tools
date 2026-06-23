package captcha

import (
	"testing"
	"time"
)

func TestNewCacheStore(t *testing.T) {
	store := NewCacheStore(10)
	if store == nil {
		t.Fatal("NewCacheStore returned nil")
	}
	if store.Expiration != 5*time.Minute {
		t.Errorf("default Expiration = %v, want 5m", store.Expiration)
	}
}

func TestCacheStoreSetAndGet(t *testing.T) {
	store := NewCacheStore(10)

	err := store.Set("testid", "ABCD")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val := store.Get("CAPTCHA_testid", false)
	if val != "ABCD" {
		t.Errorf("Get returned %q, want %q", val, "ABCD")
	}
}

func TestCacheStoreGetClear(t *testing.T) {
	store := NewCacheStore(10)

	store.Set("testid", "XYZ")
	val := store.Get("CAPTCHA_testid", true) // clear=true should delete after read

	if val != "XYZ" {
		t.Errorf("Get returned %q, want %q", val, "XYZ")
	}

	// Second get should return empty (key was cleared)
	val2 := store.Get("CAPTCHA_testid", false)
	if val2 != "" {
		t.Errorf("Get after clear should return empty, got %q", val2)
	}
}

func TestCacheStoreGetNoClear(t *testing.T) {
	store := NewCacheStore(10)

	store.Set("testid", "DATA")
	val := store.Get("CAPTCHA_testid", false) // clear=false keeps key

	if val != "DATA" {
		t.Errorf("Get returned %q, want %q", val, "DATA")
	}

	// Second get still returns the value
	val2 := store.Get("CAPTCHA_testid", false)
	if val2 != "DATA" {
		t.Errorf("Get without clear should still return value, got %q", val2)
	}
}

func TestCacheStoreVerify(t *testing.T) {
	store := NewCacheStore(10)

	store.Set("id1", "A1B2")
	if !store.Verify("id1", "A1B2", false) {
		t.Error("Verify should return true for correct answer")
	}
	if store.Verify("id1", "WRONG", false) {
		t.Error("Verify should return false for wrong answer")
	}
}

func TestCacheStoreVerifyClear(t *testing.T) {
	store := NewCacheStore(10)

	store.Set("id1", "S3CR3T")
	// Verify with clear=true: succeeds and clears
	if !store.Verify("id1", "S3CR3T", true) {
		t.Error("first Verify should succeed")
	}
	// Second attempt: key is cleared, so Get returns empty => mismatch
	if store.Verify("id1", "S3CR3T", false) {
		t.Error("Verify after clear should fail (key gone)")
	}
}

func TestCacheStoreGetMissingKey(t *testing.T) {
	store := NewCacheStore(10)

	val := store.Get("CAPTCHA_nonexistent", false)
	if val != "" {
		t.Errorf("Get for missing key should return empty, got %q", val)
	}
}

func TestCacheStorePreKeyPrefix(t *testing.T) {
	store := NewCacheStore(10)

	store.Set("abc", "123")
	// Get without PreKey prefix should return empty
	val := store.Get("abc", false)
	if val != "" {
		t.Errorf("Get without prefix should return empty, got %q", val)
	}
}

func TestCacheStoreExpiration(t *testing.T) {
	store := NewCacheStore(10)
	store.Expiration = 50 * time.Millisecond

	store.Set("exp", "value")
	val := store.Get("CAPTCHA_exp", false)
	if val != "value" {
		t.Fatal("value should exist before expiry")
	}

	time.Sleep(100 * time.Millisecond)
	val = store.Get("CAPTCHA_exp", false)
	if val != "" {
		t.Error("value should be expired, got", val)
	}
}
