package main

// allCases 返回所有测试用例（按执行顺序）。
func allCases() []TestCase {
	return []TestCase{
		// ── Health ──
		{
			Name:     "health",
			Method:   "GET",
			Path:     "/health",
			Assert:   and(statusCode(200), jsonHas("code", "data")),
			Category: "health",
		},

		// ── go-common ──
		{
			Name:     "common/crypto",
			Method:   "GET",
			Path:     "/common/crypto",
			Assert:   and(statusCode(200), dataFieldHas("md5", "sha256")),
			Category: "go-common",
		},
		{
			Name:     "common/cache",
			Method:   "GET",
			Path:     "/common/cache",
			Assert:   and(statusCode(200), dataFieldHas("algorithm", "get_value")),
			Category: "go-common",
		},
		{
			Name:     "common/error",
			Method:   "GET",
			Path:     "/common/error",
			Assert:   and(statusCode(200), dataFieldHas("predefined", "error_ranges")),
			Category: "go-common",
		},
		{
			Name:     "common/netutil",
			Method:   "GET",
			Path:     "/common/netutil",
			Assert:   and(statusCode(200), dataFieldHas("internal_ip", "network_available")),
			Category: "go-common",
		},
		{
			Name:     "common/timeutil",
			Method:   "GET",
			Path:     "/common/timeutil",
			Assert:   and(statusCode(200), dataFieldHas("format_yyyy_mm_dd", "half_year")),
			Category: "go-common",
		},
		{
			Name:     "common/log",
			Method:   "GET",
			Path:     "/common/log",
			Assert:   and(statusCode(200), dataFieldHas("message", "categories")),
			Category: "go-common",
		},
		{
			Name:     "common/httpclient",
			Method:   "GET",
			Path:     "/common/httpclient",
			Assert:   statusCode(200),
			Category: "go-common",
		},
		{
			Name:     "common/template",
			Method:   "GET",
			Path:     "/common/template",
			Assert:   and(statusCode(200), dataFieldHas("rendered", "plural")),
			Category: "go-common",
		},
		{
			Name:     "common/executil",
			Method:   "GET",
			Path:     "/common/executil",
			Assert:   and(statusCode(200), dataFieldHas("stdout", "exit_code")),
			Category: "go-common",
		},
		{
			Name:     "common/astutil",
			Method:   "GET",
			Path:     "/common/astutil",
			Assert:   and(statusCode(200), dataFieldHas("functions", "imports")),
			Category: "go-common",
		},
		{
			Name:     "common/aksk",
			Method:   "GET",
			Path:     "/common/aksk",
			Assert:   and(statusCode(200), dataFieldHas("ak", "sk", "hmac_signature")),
			Category: "go-common",
		},

		// Captcha (GET + POST chain).
		{
			Name:     "common/captcha",
			Method:   "GET",
			Path:     "/common/captcha",
			Assert:   and(statusCode(200), dataFieldHas("id", "image", "answer")),
			Category: "go-common",
			AfterRun: func(_ int, body []byte) error {
				if err := extractToken(body, "id", "captcha_id"); err != nil {
					return err
				}
				return extractToken(body, "answer", "captcha_answer")
			},
		},
		{
			Name:      "common/captcha/verify",
			Method:    "POST",
			Path:      "/common/captcha/verify",
			Body:      `{"id":"${captcha_id}","answer":"${captcha_answer}"}`,
			Assert:    and(statusCode(200), dataFieldHas("verified")),
			Category:  "go-common",
			DependsOn: "common/captcha",
		},

		// ── go-auth: JWT ──
		{
			Name:     "auth/jwt/sign",
			Method:   "POST",
			Path:     "/auth/jwt/sign",
			Body:     `{"user_id":"test-user-001"}`,
			Assert:   and(statusCode(200), dataFieldHas("access_token", "refresh_token")),
			Category: "go-auth",
			AfterRun: func(_ int, body []byte) error {
				if err := extractToken(body, "access_token", "access_token"); err != nil {
					return err
				}
				return extractToken(body, "refresh_token", "refresh_token")
			},
		},
		{
			Name:      "auth/jwt/verify",
			Method:    "POST",
			Path:      "/auth/jwt/verify",
			Body:      `{"token":"${access_token}"}`,
			Assert:    and(statusCode(200), dataFieldHas("valid", "user_uuid")),
			Category:  "go-auth",
			DependsOn: "auth/jwt/sign",
		},
		{
			Name:      "auth/jwt/refresh",
			Method:    "POST",
			Path:      "/auth/jwt/refresh",
			Body:      `{"refresh_token":"${refresh_token}"}`,
			Assert:    and(statusCode(200), dataFieldHas("access_token")),
			Category:  "go-auth",
			DependsOn: "auth/jwt/sign",
		},

		// ── go-auth: Session ──
		{
			Name:     "auth/session/create",
			Method:   "POST",
			Path:     "/auth/session",
			Body:     `{"user_id":"test-user-001","data":{"role":"admin"}}`,
			Assert:   and(statusCode(200), dataFieldHas("session_id", "expires_at")),
			Category: "go-auth",
			AfterRun: func(_ int, body []byte) error {
				return extractToken(body, "session_id", "session_id")
			},
		},
		{
			Name:      "auth/session/get",
			Method:    "GET",
			Path:      "/auth/session",
			Headers:   map[string]string{"X-Session-Id": "${session_id}"},
			Assert:    and(statusCode(200), dataFieldHas("session_id", "user_uuid")),
			Category:  "go-auth",
			DependsOn: "auth/session/create",
		},
		{
			Name:      "auth/session/delete",
			Method:    "DELETE",
			Path:      "/auth/session",
			Headers:   map[string]string{"X-Session-Id": "${session_id}"},
			Assert:    and(statusCode(200), dataFieldHas("deleted")),
			Category:  "go-auth",
			DependsOn: "auth/session/create",
		},

		// ── go-auth: Device ──
		{
			Name:     "auth/device/register",
			Method:   "POST",
			Path:     "/auth/device",
			Body:     `{"user_id":"test-user-001","device_id":"device-001"}`,
			Assert:   and(statusCode(200), dataFieldHas("device_id", "jti")),
			Category: "go-auth",
			AfterRun: func(_ int, body []byte) error {
				return extractToken(body, "device_id", "device_id")
			},
		},
		{
			Name:      "auth/device/list",
			Method:    "GET",
			Path:      "/auth/device?user_id=test-user-001",
			Assert:    and(statusCode(200), dataFieldHas("devices")),
			Category:  "go-auth",
			DependsOn: "auth/device/register",
		},
		{
			Name:      "auth/device/remove",
			Method:    "DELETE",
			Path:      "/auth/device?user_id=test-user-001&device_id=device-001",
			Assert:    and(statusCode(200), dataFieldHas("deleted")),
			Category:  "go-auth",
			DependsOn: "auth/device/register",
		},

		// ── Protected Routes ──
		{
			Name:     "protected/jwt/no-token",
			Method:   "GET",
			Path:     "/protected/jwt",
			Assert:   statusCode(401),
			Category: "auth-protected",
		},
		{
			Name:      "protected/jwt/valid",
			Method:    "GET",
			Path:      "/protected/jwt",
			Headers:   map[string]string{"Authorization": "Bearer ${access_token}"},
			Assert:    and(statusCode(200), dataFieldHas("user_uuid")),
			Category:  "auth-protected",
			DependsOn: "auth/jwt/sign",
		},
		{
			Name:      "protected/session/valid",
			Method:    "GET",
			Path:      "/protected/session",
			Headers:   map[string]string{"X-Session-Id": "${session_id}"},
			Assert:    and(statusCode(200), dataFieldHas("session_id")),
			Category:  "auth-protected",
			DependsOn: "auth/session/create",
		},
		{
			Name:      "protected/device/valid",
			Method:    "GET",
			Path:      "/protected/device",
			Headers:   map[string]string{"Authorization": "Bearer ${access_token}"},
			Assert:    and(statusCode(200), dataFieldHas("user_uuid", "device_id")),
			Category:  "auth-protected",
			DependsOn: "auth/jwt/sign",
		},

		// ── Middleware (skip in local mode) ──
		{
			Name:     "middleware/redis",
			Method:   "GET",
			Path:     "/middleware/redis",
			Assert:   statusCode(200),
			Category: "middleware",
			SkipIf:   serviceNotAvailable("redis"),
		},
		{
			Name:     "middleware/kafka",
			Method:   "POST",
			Path:     "/middleware/kafka",
			Body:     `{"key":"test","value":"hello"}`,
			Assert:   statusCode(200),
			Category: "middleware",
			SkipIf:   serviceNotAvailable("kafka"),
		},
		{
			Name:     "middleware/db",
			Method:   "GET",
			Path:     "/middleware/db",
			Assert:   statusCode(200),
			Category: "middleware",
			SkipIf:   serviceNotAvailable("db"),
		},
		{
			Name:     "middleware/es",
			Method:   "GET",
			Path:     "/middleware/es",
			Assert:   statusCode(200),
			Category: "middleware",
			SkipIf:   serviceNotAvailable("elasticsearch"),
		},
		{
			Name:     "middleware/clickhouse",
			Method:   "GET",
			Path:     "/middleware/clickhouse",
			Assert:   statusCode(200),
			Category: "middleware",
			SkipIf:   serviceNotAvailable("clickhouse"),
		},

		// ── Config ──
		{
			Name:     "config/load",
			Method:   "GET",
			Path:     "/config/load",
			Assert:   and(statusCode(200), jsonHas("code", "data")),
			Category: "config",
		},
		{
			Name:     "config/duration",
			Method:   "GET",
			Path:     "/config/duration",
			Assert:   and(statusCode(200), dataFieldHas("jwt_access_expiration")),
			Category: "config",
		},
		{
			Name:     "config/hot-reload",
			Method:   "POST",
			Path:     "/config/hot-reload",
			Assert:   and(statusCode(200), dataFieldHas("message")),
			Category: "config",
		},
		{
			Name:     "config/polaris",
			Method:   "GET",
			Path:     "/config/polaris",
			Assert:   statusCode(200),
			Category: "config",
		},

		// ── RPC ──
		{
			Name:     "rpc/echo",
			Method:   "GET",
			Path:     "/rpc/echo?message=hello",
			Assert:   and(statusCode(200), dataFieldHas("message", "service")),
			Category: "rpc",
		},
		{
			Name:     "rpc/health",
			Method:   "GET",
			Path:     "/rpc/health",
			Assert:   and(statusCode(200), dataFieldHas("healthy")),
			Category: "rpc",
		},
	}
}
