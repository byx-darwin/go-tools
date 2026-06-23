package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSignRequest(t *testing.T) {
	ak := "test-ak"
	sk := "test-sk"
	path := "/api/v1/test"
	timestamp := int64(1700000000)

	sign := signRequest(ak, sk, path, timestamp)
	assert.NotEmpty(t, sign, "sign should not be empty")
	// HMAC-SHA256 hex is 64 chars
	assert.Len(t, sign, 64, "HMAC-SHA256 hex output should be 64 characters")
}

func TestSignRequest_Deterministic(t *testing.T) {
	s1 := signRequest("ak", "sk", "/path", 1234567890)
	s2 := signRequest("ak", "sk", "/path", 1234567890)
	assert.Equal(t, s1, s2, "same inputs should produce same signature")
}

func TestSignRequest_DifferentInputs(t *testing.T) {
	s1 := signRequest("ak1", "sk", "/path", 1234567890)
	s2 := signRequest("ak2", "sk", "/path", 1234567890)
	assert.NotEqual(t, s1, s2, "different AK should produce different signature")

	s3 := signRequest("ak1", "sk", "/other", 1234567890)
	assert.NotEqual(t, s1, s3, "different path should produce different signature")

	s4 := signRequest("ak1", "sk", "/path", 9999999999)
	assert.NotEqual(t, s1, s4, "different timestamp should produce different signature")
}

func TestSignRequest_HexFormat(t *testing.T) {
	sign := signRequest("ak", "sk", "/p", 12345)
	for _, ch := range sign {
		assert.True(t, (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f'),
			"sign should only contain hex characters, got '%c'", ch)
	}
}
