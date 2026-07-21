package auth

import (
	"strings"
	"testing"
)

func TestGetRandAk_Length(t *testing.T) {
	sizes := []int{0, 1, 5, 10, 32, 64}
	for _, size := range sizes {
		if ak := GetRandAk(size); len(ak) != size {
			t.Errorf("GetRandAk(%d) length = %d, want %d", size, len(ak), size)
		}
	}
	if ak := GetRandAk(-5); ak != "" {
		t.Errorf("GetRandAk(-5) = %q, want empty string", ak)
	}
}

func TestGetRandAk_OnlyAlphanumeric(t *testing.T) {
	const validChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
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
	set := make(map[string]bool)
	for range 100 {
		ak := GetRandAk(16)
		if set[ak] {
			t.Errorf("duplicate AK generated (very unlikely): %s", ak)
		}
		set[ak] = true
	}
}

func TestGetRandAk_Coverage(t *testing.T) {
	ak := GetRandAk(10000)
	if !strings.ContainsRune(ak, '0') {
		t.Error("GetRandAk never produced '0' over 10000 chars; charset must include 0")
	}
	if !strings.ContainsRune(ak, '9') {
		t.Error("GetRandAk never produced '9' over 10000 chars; Intn(61) off-by-one must be fixed")
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

func TestAkCharset_Sanity(t *testing.T) {
	seen := make(map[rune]bool)
	for _, ch := range akCharset {
		if seen[ch] {
			t.Errorf("akCharset contains duplicate rune: %c", ch)
		}
		seen[ch] = true
	}
	if len(seen) != 62 {
		t.Errorf("akCharset unique runes = %d, want 62", len(seen))
	}
	for _, must := range []rune{'0', '9', 'O', 'a', 'Z'} {
		if !seen[must] {
			t.Errorf("akCharset missing required rune: %c", must)
		}
	}
}
