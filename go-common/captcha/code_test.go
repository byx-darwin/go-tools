package captcha

import (
	"strings"
	"testing"
)

func TestGenerateDigitCode_Length(t *testing.T) {
	for _, length := range []int{1, 4, 6, 10} {
		code := GenerateDigitCode(length)
		if len(code) != length {
			t.Errorf("GenerateDigitCode(%d) length = %d", length, len(code))
		}
	}
}

func TestGenerateDigitCode_AllDigits(t *testing.T) {
	code := GenerateDigitCode(100)
	for _, c := range code {
		if c < '0' || c > '9' {
			t.Errorf("GenerateDigitCode contains non-digit: %c", c)
		}
	}
}

func TestGenerateCode_Alphanumeric(t *testing.T) {
	code := GenerateCode(200, "alphanumeric")
	if len(code) != 200 {
		t.Errorf("length = %d, want 200", len(code))
	}
	for _, c := range code {
		if !strings.ContainsRune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", c) {
			t.Errorf("alphanumeric code contains invalid char: %c", c)
		}
	}
}

func TestGenerateCode_UnknownCharset_FallsBackToDigit(t *testing.T) {
	code := GenerateCode(50, "unknown")
	for _, c := range code {
		if c < '0' || c > '9' {
			t.Errorf("unknown charset should fallback to digit, got: %c", c)
		}
	}
}

func TestGenerateCode_DifferentEachCall(t *testing.T) {
	a := GenerateDigitCode(20)
	b := GenerateDigitCode(20)
	if a == b {
		t.Error("two consecutive calls should produce different codes")
	}
}
