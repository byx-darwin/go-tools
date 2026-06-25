package log

// Config 日志配置。
type Config struct {
	// Level 日志级别: "debug", "info", "warn", "error"（默认 "info"）。
	Level string `yaml:"level" json:"level"`

	// Format 输出格式: "json", "text"（默认 "json"）。
	Format string `yaml:"format" json:"format"`

	// Mode 输出模式: "console", "file", "both"（默认 "console"）。
	Mode string `yaml:"mode" json:"mode"`

	// AddSource 是否在日志中添加源码位置。
	AddSource bool `yaml:"add_source" json:"add_source"`

	// File 文件日志配置。
	File FileConfig `yaml:"file" json:"file"`

	// Categories 分类日志配置。
	Categories map[string]CategoryConfig `yaml:"categories" json:"categories"`

	// Masking 敏感数据脱敏配置。
	Masking MaskConfig `yaml:"masking" json:"masking"`
}

// FileConfig 文件日志配置。
type FileConfig struct {
	// Dir 日志文件目录。
	Dir string `yaml:"dir" json:"dir"`

	// Filename 日志文件名。
	Filename string `yaml:"filename" json:"filename"`

	// MaxSize 单个日志文件最大 MB（默认 100）。
	MaxSize int `yaml:"max_size" json:"max_size"`

	// MaxBackups 保留的旧日志文件最大数量（默认 7）。
	MaxBackups int `yaml:"max_backups" json:"max_backups"`

	// MaxAge 保留旧日志文件的最大天数（默认 30）。
	MaxAge int `yaml:"max_age" json:"max_age"`

	// Compress 是否 gzip 压缩旧日志文件。
	Compress bool `yaml:"compress" json:"compress"`
}

// CategoryConfig 分类日志配置。
type CategoryConfig struct {
	// Enabled 是否启用该分类。
	Enabled bool `yaml:"enabled" json:"enabled"`

	// File 日志文件名（相对于 File.Dir）。
	File string `yaml:"file" json:"file"`

	// Level 日志级别（覆盖全局 Level）。
	Level string `yaml:"level" json:"level"`
}

// MaskConfig 敏感数据脱敏配置。
type MaskConfig struct {
	// Enabled 是否启用脱敏。
	Enabled bool `yaml:"enabled" json:"enabled"`

	// MaskedFields 需要脱敏的字段列表。
	MaskedFields []string `yaml:"masked_fields" json:"masked_fields"`

	// Mode 脱敏模式: "full", "partial"。
	Mode string `yaml:"mode" json:"mode"`
}

// ConfigOption 定义 Config 配置选项函数。
type ConfigOption func(*Config)

// WithConfigLevel 设置日志级别。
func WithConfigLevel(level string) ConfigOption {
	return func(c *Config) {
		if level != "" {
			c.Level = level
		}
	}
}

// WithConfigFormat 设置输出格式。
func WithConfigFormat(format string) ConfigOption {
	return func(c *Config) {
		if format != "" {
			c.Format = format
		}
	}
}

// WithConfigMode 设置输出模式。
func WithConfigMode(mode string) ConfigOption {
	return func(c *Config) {
		if mode != "" {
			c.Mode = mode
		}
	}
}

// WithConfigAddSource 设置是否添加源码位置。
func WithConfigAddSource(addSource bool) ConfigOption {
	return func(c *Config) {
		c.AddSource = addSource
	}
}

// WithConfigFile 设置文件配置。
func WithConfigFile(file FileConfig) ConfigOption {
	return func(c *Config) {
		c.File = file
	}
}

// WithConfigCategories 设置分类配置。
func WithConfigCategories(categories map[string]CategoryConfig) ConfigOption {
	return func(c *Config) {
		if categories != nil {
			c.Categories = categories
		}
	}
}

// WithConfigMasking 设置脱敏配置。
func WithConfigMasking(masking MaskConfig) ConfigOption {
	return func(c *Config) {
		c.Masking = masking
	}
}

// NewConfig 创建 Config，支持 Options 配置。
//
// 默认配置：
//   - level: "info"
//   - format: "json"
//   - mode: "console"
func NewConfig(opts ...ConfigOption) Config {
	cfg := Config{
		Level:  defaultConfigLevel,
		Format: defaultConfigFormat,
		Mode:   defaultConfigMode,
		File:   NewFileConfig(),
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// FileOption 定义 FileConfig 配置选项函数。
type FileOption func(*FileConfig)

// WithFileDir 设置日志文件目录。
func WithFileDir(dir string) FileOption {
	return func(c *FileConfig) {
		if dir != "" {
			c.Dir = dir
		}
	}
}

// WithFilename 设置日志文件名。
func WithFilename(filename string) FileOption {
	return func(c *FileConfig) {
		if filename != "" {
			c.Filename = filename
		}
	}
}

// WithFileMaxSize 设置单个日志文件最大 MB。
func WithFileMaxSize(maxSize int) FileOption {
	return func(c *FileConfig) {
		if maxSize > 0 {
			c.MaxSize = maxSize
		}
	}
}

// WithFileMaxBackups 设置保留的旧日志文件最大数量。
func WithFileMaxBackups(maxBackups int) FileOption {
	return func(c *FileConfig) {
		if maxBackups > 0 {
			c.MaxBackups = maxBackups
		}
	}
}

// WithFileMaxAge 设置保留旧日志文件的最大天数。
func WithFileMaxAge(maxAge int) FileOption {
	return func(c *FileConfig) {
		if maxAge > 0 {
			c.MaxAge = maxAge
		}
	}
}

// WithFileCompress 设置是否 gzip 压缩旧日志文件。
func WithFileCompress(compress bool) FileOption {
	return func(c *FileConfig) {
		c.Compress = compress
	}
}

// NewFileConfig 创建 FileConfig，支持 Options 配置。
//
// 默认配置：
//   - maxSize: 100
//   - maxBackups: 7
//   - maxAge: 30
func NewFileConfig(opts ...FileOption) FileConfig {
	cfg := FileConfig{
		MaxSize:    defaultFileMaxSize,
		MaxBackups: defaultFileMaxBackups,
		MaxAge:     defaultFileMaxAge,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// Config 默认值常量。
const (
	defaultConfigLevel  = "info"
	defaultConfigFormat = "json"
	defaultConfigMode   = "console"

	defaultFileMaxSize    = 100
	defaultFileMaxBackups = 7
	defaultFileMaxAge     = 30
)
