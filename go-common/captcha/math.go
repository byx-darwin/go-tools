package captcha

import (
	"github.com/mojocn/base64Captcha"
)

// MathImageCaptcha 封装 base64Captcha 的数学运算验证码。
// 生成类似 "3+5=?" 的数学表达式验证码。
type MathImageCaptcha struct {
	captcha *base64Captcha.Captcha
}

// NewMathImageCaptcha 创建数学运算验证码实例。
//
// 默认配置：
//   - width: 240
//   - height: 80
//   - noiseCount: 0
//   - showLineOptions: 无
//   - cacheLength: 1024
//   - cacheExpiresTime: 5m
//
// 生成的验证码格式：加/减/乘运算，如 "3+5=?"、"10-3=?"、"4x2=?"
func NewMathImageCaptcha(opts ...ImageOption) *MathImageCaptcha {
	cfg := &imageConfig{
		width:            defaultWidth,
		height:           defaultHeight,
		cacheLength:      defaultCacheLength,
		cacheExpiresTime: defaultCacheExpires,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	driver := base64Captcha.NewDriverMath(cfg.height, cfg.width, 0, 0, nil, nil, nil)
	store := NewCacheStore(
		WithCapacity(cfg.cacheLength),
		WithExpiration(cfg.cacheExpiresTime),
	)

	return &MathImageCaptcha{
		captcha: base64Captcha.NewCaptcha(driver, store),
	}
}

// Generate 生成数学验证码，返回 (id, base64Image, answer, error)。
// answer 为运算结果，如 "8"（对于 "3+5=?"）。
func (mc *MathImageCaptcha) Generate() (id, b64s, answer string, err error) {
	return mc.captcha.Generate()
}

// Verify 校验用户输入是否与验证码答案匹配。
// shouldClear=true 时校验通过后自动清除缓存（一次性使用）。
func (mc *MathImageCaptcha) Verify(id, answer string, shouldClear bool) bool {
	return mc.captcha.Verify(id, answer, shouldClear)
}
