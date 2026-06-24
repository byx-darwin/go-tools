package config

import "time"

// CaptchaOption 图形验证码配置（用于 YAML 反序列化）。
// 零值字段由下游 NewImageCaptcha 的 Options 自动填充默认值。
//
// YAML 示例：
//
//	captcha:
//	  key_long: 6              # 验证码字符长度（默认 6）
//	  img_width: 240           # 图片宽度 px（默认 240）
//	  img_height: 80           # 图片高度 px（默认 80）
//	  cache_length: 1024       # 本地缓存条目上限（默认 1024）
//	  cache_expires_time: 120s # 缓存过期时间（默认 120s，最小 2s）
type CaptchaOption struct {
	// KeyLong 验证码字符长度。默认 6。
	KeyLong int `json:"key_long" yaml:"key_long"`

	// ImgWidth 验证码图片宽度（像素）。默认 240。
	ImgWidth int `json:"img_width" yaml:"img_width"`

	// ImgHeight 验证码图片高度（像素）。默认 80。
	ImgHeight int `json:"img_height" yaml:"img_height"`

	// CacheLength 本地缓存条目上限。默认 1024。
	CacheLength int `json:"cache_length" yaml:"cache_length"`

	// CacheExpiresTime 缓存过期时间。默认 120s，最小 2s。
	// 使用 time.Duration，YAML 支持 120s / 2m 等格式。
	CacheExpiresTime time.Duration `json:"cache_expires_time" yaml:"cache_expires_time"`
}
