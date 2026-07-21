package crypto

import (
	"crypto/sha256"
	"testing"
)

func TestMD5(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{"empty", []byte(""), "d41d8cd98f00b204e9800998ecf8427e"},
		{"hello", []byte("hello"), "5d41402abc4b2a76b9719d911017c592"},
		{"numbers", []byte("123456"), "e10adc3949ba59abbe56e057f20f883e"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MD5(tt.input)
			if got != tt.expected {
				t.Errorf("MD5(%q) = %s, want %s", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSHA1(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{"hello", []byte("hello"), "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"},
		{"empty", []byte(""), "da39a3ee5e6b4b0d3255bfef95601890afd80709"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SHA1(tt.input)
			if got != tt.expected {
				t.Errorf("SHA1(%q) = %s, want %s", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSHA512(t *testing.T) {
	data := []byte("hello")
	result := SHA512(data)
	if len(result) != 128 { // SHA512 produces 64 bytes = 128 hex chars
		t.Errorf("SHA512 hex output length = %d, want 128", len(result))
	}
}

func TestHmac(t *testing.T) {
	key := []byte("mykey")
	data := []byte("mydata")

	h := Hmac(key, data, sha256.New)
	if h == "" {
		t.Error("Hmac returned empty string")
	}

	// Same inputs produce same output
	h2 := Hmac(key, data, sha256.New)
	if h != h2 {
		t.Error("Hmac is not deterministic")
	}

	// Different key produces different output
	h3 := Hmac([]byte("other"), data, sha256.New)
	if h == h3 {
		t.Error("Hmac should differ with different keys")
	}
}

func TestEncodePwd(t *testing.T) {
	password := "mypassword"
	ak := "myaccesskey"

	pwd := EncodePwd(password, ak)
	if pwd == "" {
		t.Error("EncodePwd returned empty string")
	}

	// Deterministic
	pwd2 := EncodePwd(password, ak)
	if pwd != pwd2 {
		t.Error("EncodePwd is not deterministic")
	}
}
