package api

import (
	"regexp"
	"strings"
)

// DSN 重写辅助：把 DSN 中的库名换成测试专用库，避免污染开发数据。

var (
	// MySQL: user:pass@tcp(host:port)/dbname?params
	mysqlDBRe = regexp.MustCompile(`/([^/?]*)(\?|$)`)
	// PostgreSQL: postgres://user:pass@host:port/dbname?params
	pgDBRe = regexp.MustCompile(`/(?:[^/?]*)(\?|$)`)
)

// replaceMySQLDB 替换 MySQL DSN 中的库名。
func replaceMySQLDB(dsn, schema string) string {
	return mysqlDBRe.ReplaceAllString(dsn, "/"+schema+"$2")
}

// replacePGDB 替换 PostgreSQL DSN 中的库名。
func replacePGDB(dsn, schema string) string {
	// 只替换 path 部分，避免动到协议头与查询参数
	base, query, hasQuery := strings.Cut(dsn, "?")
	base = pgDBRe.ReplaceAllString(base, "/"+schema+"$1")
	if hasQuery {
		return base + "?" + query
	}
	return base
}
