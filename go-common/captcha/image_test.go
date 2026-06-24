package captcha

import (
	"testing"
	"time"
)

func TestNewImageCaptcha_Defaults(t *testing.T) {
	ic := NewImageCaptcha()
	if ic == nil {
		t.Fatal("NewImageCaptcha returned nil")
	}
}

func TestNewImageCaptcha_WithOptions(t *testing.T) {
	ic := NewImageCaptcha(
		WithWidth(300),
		WithHeight(100),
		WithKeyLong(4),
		WithImageCacheLength(200),
		WithImageCacheExpiration(2*time.Minute),
	)
	if ic == nil {
		t.Fatal("NewImageCaptcha with options returned nil")
	}
}

func TestNewImageCaptchaLegacy(t *testing.T) {
	ic := NewImageCaptchaLegacy(240, 80, 6, 100, 2*time.Minute)
	if ic == nil {
		t.Fatal("NewImageCaptchaLegacy returned nil")
	}
}

func TestImageCaptcha_GenerateAndVerify(t *testing.T) {
	ic := NewImageCaptcha(WithKeyLong(4), WithImageCacheExpiration(2*time.Minute))

	id, b64s, answer, err := ic.Generate()
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if id == "" {
		t.Error("Generate returned empty id")
	}
	if b64s == "" {
		t.Error("Generate returned empty base64 image")
	}
	if answer == "" {
		t.Error("Generate returned empty answer")
	}

	if !ic.Verify(id, answer, false) {
		t.Error("Verify should return true for correct answer")
	}
	if ic.Verify(id, "WRONG", false) {
		t.Error("Verify should return false for wrong answer")
	}
}

func TestImageCaptcha_VerifyClear(t *testing.T) {
	ic := NewImageCaptcha(WithKeyLong(4), WithImageCacheExpiration(2*time.Minute))

	id, _, answer, err := ic.Generate()
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if !ic.Verify(id, answer, true) {
		t.Error("first Verify with clear should succeed")
	}
	if ic.Verify(id, answer, false) {
		t.Error("Verify after clear should fail")
	}
}

func TestImageOption_ZeroValuesIgnored(t *testing.T) {
	// 零值不应覆盖默认值
	ic := NewImageCaptcha(
		WithWidth(0),
		WithHeight(0),
		WithKeyLong(0),
		WithImageCacheLength(0),
		WithImageCacheExpiration(0),
	)
	if ic == nil {
		t.Fatal("NewImageCaptcha with zero options returned nil")
	}
	// 能正常生成和验证即可
	id, _, answer, err := ic.Generate()
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if !ic.Verify(id, answer, false) {
		t.Error("Verify should succeed with default-configured captcha")
	}
}
