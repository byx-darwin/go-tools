package captcha

import (
	"strconv"
	"testing"
	"time"
)

func TestMathImageCaptcha_Generate(t *testing.T) {
	mc := NewMathImageCaptcha()
	id, b64s, answer, err := mc.Generate()

	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if id == "" {
		t.Error("Generate() id is empty")
	}
	if b64s == "" {
		t.Error("Generate() b64s is empty")
	}
	if answer == "" {
		t.Error("Generate() answer is empty")
	}

	// 验证答案是数字
	if _, err := strconv.Atoi(answer); err != nil {
		t.Errorf("Generate() answer = %q, want numeric", answer)
	}
}

func TestMathImageCaptcha_Verify(t *testing.T) {
	mc := NewMathImageCaptcha()
	id, _, answer, err := mc.Generate()

	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// 正确答案应该通过验证
	if !mc.Verify(id, answer, false) {
		t.Error("Verify() with correct answer = false, want true")
	}

	// 错误答案应该失败
	if mc.Verify(id, "999999", false) {
		t.Error("Verify() with wrong answer = true, want false")
	}

	// shouldClear=true 时验证后应该清除
	if !mc.Verify(id, answer, true) {
		t.Error("Verify() with shouldClear=true = false, want true")
	}

	// 清除后再次验证应该失败
	if mc.Verify(id, answer, false) {
		t.Error("Verify() after clear = true, want false")
	}
}

func TestMathImageCaptcha_WithOptions(t *testing.T) {
	mc := NewMathImageCaptcha(
		WithWidth(300),
		WithHeight(100),
		WithImageCacheLength(512),
		WithImageCacheExpiration(2*time.Minute),
	)

	id, b64s, answer, err := mc.Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if id == "" || b64s == "" || answer == "" {
		t.Error("Generate() returned empty values")
	}
}
