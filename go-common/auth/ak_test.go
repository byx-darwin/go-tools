package auth

import (
	"strings"
	"testing"
)

func TestGetRandAk_Length(t *testing.T) {
	sizes := []int{0, 1, 5, 10, 32, 64}
	for _, size := range sizes {
		ak := GetRandAk(size)
		if len(ak) != size {
			t.Errorf("GetRandAk(%d) length = %d, want %d", size, len(ak), size)
		}
	}
}

func TestGetRandAk_OnlyAlphanumeric(t *testing.T) {
	const validChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLOMNOPQRSTUVWXYZ123456789"

	// Generate multiple AKs to increase coverage
	for range 20 {
		ak := GetRandAk(100)
		for _, ch := range ak {
			if !strings.ContainsRune(validChars, ch) {
				t.Errorf("GetRandAk contains invalid char: %c", ch)
			}
		}
	}
}

func TestGetRandAk_Uniqueness(t *testing.T) {
	// Two calls with large length should almost certainly produce different results
	set := make(map[string]bool)
	for range 100 {
		ak := GetRandAk(16)
		if set[ak] {
			t.Errorf("duplicate AK generated (very unlikely): %s", ak)
		}
		set[ak] = true
	}
}

func TestRefreshSK_ProducesValue(t *testing.T) {
	ak := GetRandAk(10)
	sk := RefreshSK(ak)
	if sk == "" {
		t.Error("RefreshSK should return non-empty string")
	}
}

func TestRefreshSK_SameInputProducesConsistentFormat(t *testing.T) {
	// RefreshSK uses time.Now() internally so it changes per call,
	// but the output should always be a hex-encoded MD5 (32 hex chars)
	ak := GetRandAk(10)
	sk := RefreshSK(ak)

	// MD5 hex is always 32 chars
	if len(sk) != 32 {
		t.Errorf("RefreshSK hex length = %d, want 32 (MD5 hex)", len(sk))
	}

	// All chars should be hex
	for _, ch := range sk {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			t.Errorf("RefreshSK contains non-hex char: %c", ch)
		}
	}
}

func TestRefreshSK_DifferentAKProducesDifferentSK(t *testing.T) {
	ak1 := "testAK1"
	ak2 := "testAK2"

	// Same timestamp but different AK → different SK
	// (Note: RefreshSK uses time.Now(), we just verify the signer is different)
	sk1 := RefreshSK(ak1)
	sk2 := RefreshSK(ak2)
	if sk1 == sk2 {
		t.Error("different AKs should produce different SKs")
	}
}

func TestIntegration_AKAndSK(t *testing.T) {
	// End-to-end: generate AK, then SK
	ak := GetRandAk(32)
	if len(ak) != 32 {
		t.Fatalf("AK length = %d, want 32", len(ak))
	}

	sk := RefreshSK(ak)
	if len(sk) != 32 {
		t.Fatalf("SK length = %d, want 32 (MD5 hex)", len(sk))
	}
}
