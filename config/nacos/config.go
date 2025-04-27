package nacos

type Config struct {
	// Nacos config
	IPAddr              string
	Port                uint64
	TimeoutMs           uint64 // Default: 5000
	NotLoadCacheAtStart bool   // Default: true
	NamespaceId         string
	LogDir              string // Default: /tmp/nacos/log
	CacheDir            string // Default: /tmp/nacos/cache
	LogLevel            string // Default: info

	Username string
	Password string
	// Nacos config end
}
