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

func TestRefreshSK_Length(t *testing.T) {
	if sk := RefreshSK(); len(sk) != 64 {
		t.Errorf("RefreshSK length = %d, want 64 (32-byte hex)", len(sk))
	}
}

func TestRefreshSK_HexCharset(t *testing.T) {
	sk := RefreshSK()
	for _, ch := range sk {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			t.Errorf("RefreshSK contains non-hex char: %c", ch)
		}
	}
}

func TestRefreshSK_Uniqueness(t *testing.T) {
	set := make(map[string]bool)
	for range 100 {
		sk := RefreshSK()
		if set[sk] {
			t.Errorf("duplicate SK generated (astronomically unlikely): %s", sk)
		}
		set[sk] = true
	}
}

func TestIntegration_AKAndSK(t *testing.T) {
	ak := GetRandAk(32)
	if len(ak) != 32 {
		t.Fatalf("AK length = %d, want 32", len(ak))
	}
	sk := RefreshSK()
	if len(sk) != 64 {
		t.Fatalf("SK length = %d, want 64 (32-byte hex)", len(sk))
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
