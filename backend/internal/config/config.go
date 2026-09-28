package config

import (
	"os"
	"strconv"
	"time"
)

// Config 运行时配置（环境变量驱动）。
type Config struct {
	Port          string
	DataDir       string
	FrontendDist  string
	JWTSecret     string
	JWTExpiry     time.Duration
	AdminUsername string
	AdminPassword string
	SiteURL       string

	// 数据库
	DBType string // sqlite | mysql | pgsql
	DBDSN  string

	// Redis（可选）
	RedisEnabled bool
	RedisURL     string
}

func Load() Config {
	return Config{
		Port:          envOr("PORT", "3000"),
		DataDir:       envOr("DATA_DIR", "data"),
		FrontendDist:  os.Getenv("FRONTEND_DIST"),
		JWTSecret:     envOr("JWT_SECRET", "dev-only-secret-change-me"),
		JWTExpiry:     durOr("JWT_EXPIRES_IN", 7*24*time.Hour),
		AdminUsername: envOr("ADMIN_USERNAME", "admin"),
		AdminPassword: envOr("ADMIN_PASSWORD", "admin123"),
		SiteURL:       trimSlash(envOr("SITE_URL", "http://localhost:8080")),
		DBType:        envOr("DB_TYPE", "sqlite"),
		DBDSN:         envOr("DB_DSN", ""),
		RedisEnabled:  boolOr("REDIS_ENABLED", false),
		RedisURL:      envOr("REDIS_URL", ""),
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
