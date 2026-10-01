package db

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"narratpage/internal/config"
	"narratpage/internal/dialect"
)

func testConfig(dir, dbType, dsn string) config.Config {
	return config.Config{
		DataDir:   dir,
		DBType:    dbType,
		DBDSN:     dsn,
		JWTSecret: "test",
		JWTExpiry: time.Hour,
		Port:      "0",
	}
}

func assertTablesExist(t *testing.T, d *sql.DB, dbType string, tables ...string) {
	t.Helper()
	for _, table := range tables {
		var n int
		var err error
		switch dbType {
		case "mysql":
			err = d.QueryRow(
				"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
				table).Scan(&n)
		case "pgsql":
			err = d.QueryRow(
				"SELECT COUNT(*) FROM information_schema.tables WHERE table_name = $1", table).Scan(&n)
		default:
			err = d.QueryRow(
				"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&n)
		}
		if err != nil {
			t.Errorf("查询表 %s 是否存在时出错: %v", table, err)
			continue
		}
		if n != 1 {
			t.Errorf("表 %s 未成功创建", table)
		}
	}
}

func escapeForTest(v string) string { return dialect.EscapeLike(v) }

func slugFor(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
