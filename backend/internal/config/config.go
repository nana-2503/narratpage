// Package config 加载运行时配置（环境变量驱动）。
package config

import (
	"os"
	"strconv"
	"time"
)

// Config 运行时配置。
type Config struct {
	Port         string
	DataDir      string
	FrontendDist string
	JWTSecret    string
	JWTExpiry    time.Duration
	SiteURL      string

	// 首次安装时创建的初始管理员。仅在 users 表为空时生效，
	// 日常认证走 users 表，不依赖这两个变量。
	AdminUsername string
	AdminPassword string

	// TrustProxy 声明后端位于可信反向代理（nginx / 网关）之后。
	// 开启后限流会解析 X-Forwarded-For 以获得真实客户端 IP；
	// 若后端可被直连访问则不应开启，否则客户端可伪造该头绕过限流。
	TrustProxy bool

	// 数据库
	DBType string // sqlite | mysql | pgsql
	DBDSN  string

	// Redis（可选）
	RedisEnabled bool
	RedisURL     string
}

// Load 从环境变量加载配置。
func Load() Config {
	return Config{
		Port:          envOr("PORT", "3000"),
		DataDir:       envOr("DATA_DIR", "data"),
		FrontendDist:  os.Getenv("FRONTEND_DIST"),
		JWTSecret:     envOr("JWT_SECRET", "dev-only-secret-change-me"),
		JWTExpiry:     durOr("JWT_EXPIRES_IN", 7*24*time.Hour),
		SiteURL:       trimSlash(envOr("SITE_URL", "http://localhost:8080")),
		AdminUsername: envOr("ADMIN_USERNAME", "admin"),
		AdminPassword: envOr("ADMIN_PASSWORD", "admin123"),
		TrustProxy:    boolOr("TRUST_PROXY", false),
		DBType:        envOr("DB_TYPE", "sqlite"),
		DBDSN:         os.Getenv("DB_DSN"),
		RedisEnabled:  boolOr("REDIS_ENABLED", false),
		RedisURL:      os.Getenv("REDIS_URL"),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func durOr(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func boolOr(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
