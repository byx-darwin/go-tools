package config

import (
	"testing"
	"time"
)

func TestCaptchaOption_ZeroValues(t *testing.T) {
	c := &CaptchaOption{}
	// 零值字段保留（由下游 NewImageCaptcha Options 处理默认值）
	if c.KeyLong != 0 {
		t.Errorf("zero KeyLong = %d, want 0", c.KeyLong)
	}
	if c.ImgWidth != 0 {
		t.Errorf("zero ImgWidth = %d, want 0", c.ImgWidth)
	}
	if c.CacheExpiresTime != 0 {
		t.Errorf("zero CacheExpiresTime = %v, want 0", c.CacheExpiresTime)
	}
}

func TestCaptchaOption_FullValues(t *testing.T) {
	c := &CaptchaOption{
		KeyLong:          8,
		ImgWidth:         300,
		ImgHeight:        100,
		CacheLength:      2048,
		CacheExpiresTime: 60 * time.Second,
	}
	if c.KeyLong != 8 {
		t.Errorf("KeyLong = %d, want 8", c.KeyLong)
	}
	if c.ImgWidth != 300 {
		t.Errorf("ImgWidth = %d, want 300", c.ImgWidth)
	}
	if c.ImgHeight != 100 {
		t.Errorf("ImgHeight = %d, want 100", c.ImgHeight)
	}
	if c.CacheLength != 2048 {
		t.Errorf("CacheLength = %d, want 2048", c.CacheLength)
	}
	if c.CacheExpiresTime != 60*time.Second {
		t.Errorf("CacheExpiresTime = %v, want 60s", c.CacheExpiresTime)
	}
}
