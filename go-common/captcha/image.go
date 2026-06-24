package captcha

import (
	"time"

	"github.com/mojocn/base64Captcha"
)

const (
	defaultWidth        = 240
	defaultHeight       = 80
	defaultKeyLong      = 6
	defaultCacheLength  = 1024
	defaultCacheExpires = 5 * time.Minute
)

// ImageOption 定义图形验证码配置选项。
type ImageOption func(*imageConfig)

type imageConfig struct {
	width            int
	height           int
	keyLong          int
	cacheLength      int
	cacheExpiresTime time.Duration
}

// WithWidth 设置图片宽度（像素）。
func WithWidth(width int) ImageOption {
	return func(c *imageConfig) {
		if width > 0 {
			c.width = width
		}
	}
}

// WithHeight 设置图片高度（像素）。
func WithHeight(height int) ImageOption {
	return func(c *imageConfig) {
		if height > 0 {
			c.height = height
		}
	}
}

// WithKeyLong 设置验证码字符长度。
func WithKeyLong(keyLong int) ImageOption {
	return func(c *imageConfig) {
		if keyLong > 0 {
			c.keyLong = keyLong
		}
	}
}

// WithImageCacheLength 设置本地缓存条目上限。
func WithImageCacheLength(length int) ImageOption {
	return func(c *imageConfig) {
		if length > 0 {
			c.cacheLength = length
		}
	}
}

// WithImageCacheExpiration 设置缓存过期时间。
func WithImageCacheExpiration(d time.Duration) ImageOption {
	return func(c *imageConfig) {
		if d > 0 {
			c.cacheExpiresTime = d
		}
	}
}

// ImageCaptcha 封装 base64Captcha，提供图形验证码的生成与校验。
type ImageCaptcha struct {
	captcha *base64Captcha.Captcha
}

// NewImageCaptcha 创建图形验证码实例，支持 Options 配置。
//
// 默认配置：
//   - width: 240
//   - height: 80
//   - keyLong: 6
//   - cacheLength: 1024
//   - cacheExpiresTime: 5m
//
// 内部使用 CacheStore（基于 samber/hot FIFO）替代 base64Captcha 默认内存存储。
func NewImageCaptcha(opts ...ImageOption) *ImageCaptcha {
	cfg := &imageConfig{
		width:            defaultWidth,
		height:           defaultHeight,
		keyLong:          defaultKeyLong,
		cacheLength:      defaultCacheLength,
		cacheExpiresTime: defaultCacheExpires,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	driver := base64Captcha.NewDriverDigit(cfg.height, cfg.width, cfg.keyLong, 0.7, 80)
	store := NewCacheStore(
		WithCapacity(cfg.cacheLength),
		WithExpiration(cfg.cacheExpiresTime),
	)
	return &ImageCaptcha{
		captcha: base64Captcha.NewCaptcha(driver, store),
	}
}

// NewImageCaptchaLegacy 保留向后兼容。
//
// Deprecated: 使用 NewImageCaptcha 配合 Options 替代。
func NewImageCaptchaLegacy(width, height, keyLong, cacheLength int, cacheExpiresTime time.Duration) *ImageCaptcha {
	return NewImageCaptcha(
		WithWidth(width),
		WithHeight(height),
		WithKeyLong(keyLong),
		WithImageCacheLength(cacheLength),
		WithImageCacheExpiration(cacheExpiresTime),
	)
}

// Generate 生成验证码，返回 (id, base64Image, answer, error)。
// answer 为验证码答案，一般不传给前端，仅用于调试。
func (ic *ImageCaptcha) Generate() (id, b64s, answer string, err error) {
	return ic.captcha.Generate()
}

// Verify 校验用户输入是否与验证码匹配。
// clear=true 时校验通过后自动清除缓存（一次性使用）。
func (ic *ImageCaptcha) Verify(id, answer string, clear bool) bool {
	return ic.captcha.Verify(id, answer, clear)
}
