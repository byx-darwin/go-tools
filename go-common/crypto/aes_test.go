package crypto

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"
)

func TestAESGCMRoundtrip(t *testing.T) {
	key := []byte("0123456789abcdef") // 16 bytes
	tests := []struct {
		name      string
		plaintext []byte
	}{
		{"empty", []byte("")},
		{"single-byte", []byte("x")},
		{"short", []byte("hello")},
		{"one-block", []byte("data1234data1234")},
		{"multi-block", []byte("this is longer data spanning multiple blocks")},
	}

	g, err := NewAESGCM(key)
	if err != nil {
		t.Fatalf("NewAESGCM failed: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sealed, err := g.Seal(tt.plaintext)
			if err != nil {
				t.Fatalf("Seal failed: %v", err)
			}
			opened, err := g.Open(sealed)
			if err != nil {
				t.Fatalf("Open failed: %v", err)
			}
			if !bytes.Equal(opened, tt.plaintext) {
				t.Errorf("roundtrip mismatch: got %q, want %q", opened, tt.plaintext)
			}
		})
	}
}

func TestAESGCMKeySizes(t *testing.T) {
	plaintext := []byte("key size test")
	for _, size := range []int{16, 24, 32} {
		key := make([]byte, size)
		if _, err := rand.Read(key); err != nil {
			t.Fatalf("rand.Read failed: %v", err)
		}
		g, err := NewAESGCM(key)
		if err != nil {
			t.Fatalf("NewAESGCM(%d bytes) failed: %v", size, err)
		}
		sealed, err := g.Seal(plaintext)
		if err != nil {
			t.Fatalf("Seal failed: %v", err)
		}
		opened, err := g.Open(sealed)
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		if !bytes.Equal(opened, plaintext) {
			t.Errorf("roundtrip mismatch for %d-byte key", size)
		}
	}
}

func TestAESGCMInvalidKeySize(t *testing.T) {
	for _, size := range []int{0, 15, 17, 31, 33} {
		key := make([]byte, size)
		_, err := NewAESGCM(key)
		if !errors.Is(err, ErrInvalidKeySize) {
			t.Errorf("NewAESGCM(%d bytes) error = %v, want ErrInvalidKeySize", size, err)
		}
	}
}

func TestAESGCMTamperedCiphertext(t *testing.T) {
	key := []byte("0123456789abcdef")
	g, err := NewAESGCM(key)
	if err != nil {
		t.Fatalf("NewAESGCM failed: %v", err)
	}
	sealed, err := g.Seal([]byte("tamper me"))
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}
	sealed[len(sealed)-1] ^= 0xff // flip a bit in the tag
	if _, err := g.Open(sealed); err == nil {
		t.Error("Open should fail on tampered ciphertext")
	}
}

func TestAESGCMCiphertextTooShort(t *testing.T) {
	key := []byte("0123456789abcdef")
	g, err := NewAESGCM(key)
	if err != nil {
		t.Fatalf("NewAESGCM failed: %v", err)
	}
	_, err = g.Open([]byte("too short"))
	if !errors.Is(err, ErrCiphertextTooShort) {
		t.Errorf("Open(short) error = %v, want ErrCiphertextTooShort", err)
	}
}

func TestAESGCMAssociatedData(t *testing.T) {
	key := []byte("0123456789abcdef")
	plaintext := []byte("aad protected")

	sealer, err := NewAESGCM(key, WithAssociatedData([]byte("aad-A")))
	if err != nil {
		t.Fatalf("NewAESGCM(sealer) failed: %v", err)
	}
	sealed, err := sealer.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal failed: %v", err)
	}

	openerSame, err := NewAESGCM(key, WithAssociatedData([]byte("aad-A")))
	if err != nil {
		t.Fatalf("NewAESGCM(openerSame) failed: %v", err)
	}
	opened, err := openerSame.Open(sealed)
	if err != nil {
		t.Fatalf("Open with matching AAD failed: %v", err)
	}
	if !bytes.Equal(opened, plaintext) {
		t.Errorf("AAD roundtrip mismatch: got %q, want %q", opened, plaintext)
	}

	openerDiff, err := NewAESGCM(key, WithAssociatedData([]byte("aad-B")))
	if err != nil {
		t.Fatalf("NewAESGCM(openerDiff) failed: %v", err)
	}
	if _, err := openerDiff.Open(sealed); err == nil {
		t.Error("Open should fail when AAD does not match")
	}
}

func TestAESGCMRandomNonce(t *testing.T) {
	key := []byte("0123456789abcdef")
	g, err := NewAESGCM(key)
	if err != nil {
		t.Fatalf("NewAESGCM failed: %v", err)
	}
	plaintext := []byte("same plaintext")
	c1, err := g.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal #1 failed: %v", err)
	}
	c2, err := g.Seal(plaintext)
	if err != nil {
		t.Fatalf("Seal #2 failed: %v", err)
	}
	if bytes.Equal(c1, c2) {
		t.Error("two Seal calls of the same plaintext should produce different ciphertexts (random nonce)")
	}
}
