package db

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"narratpage/internal/config"
	"narratpage/internal/dialect"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// Open 打开数据库连接并执行 schema 迁移（CREATE IF NOT EXISTS，幂等）。
func Open(cfg config.Config) (*sql.DB, error) {
	var d *sql.DB
	var err error

	switch cfg.DBType {
	case "mysql":
		dsn := cfg.DBDSN
		if dsn == "" {
			dsn = fmt.Sprintf("%s:%s@tcp(127.0.0.1:3306)/%s?charset=utf8mb4&parseTime=true&loc=Local",
				cfg.AdminUsername, cfg.AdminPassword, "narratpage")
		}
		d, err = sql.Open("mysql", dsn)
	case "pgsql":
		dsn := cfg.DBDSN
		if dsn == "" {
			dsn = fmt.Sprintf("postgres://%s:%s@127.0.0.1:5432/narratpage?sslmode=disable",
				cfg.AdminUsername, cfg.AdminPassword)
		}
		d, err = sql.Open("postgres", dsn)
	default:
		path := filepath.Join(cfg.DataDir, "blog.db")
		dsn := fmt.Sprintf(
			"file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)",
			path,
		)
		d, err = sql.Open("sqlite", dsn)
	}

	if err != nil {
		return nil, err
	}

	switch cfg.DBType {
	case "mysql", "pgsql":
		d.SetMaxOpenConns(25)
		d.SetMaxIdleConns(5)
		d.SetConnMaxLifetime(5 * time.Minute)
	default:
		d.SetMaxOpenConns(4)
	}

	if err := d.Ping(); err != nil {
		d.Close()
		return nil, err
	}

	if err := migrate(d, cfg.DBType); err != nil {
		d.Close()
		return nil, err
	}

	return d, nil
}

// Dialect 返回当前数据库方言。
func Dialect(dbType string) dialect.Dialect {
	switch dbType {
	case "mysql":
		return dialect.MySQL
	case "pgsql":
		return dialect.PostgreSQL
	default:
		return dialect.SQLite
	}
}

// migrate 执行 schema 迁移。
func migrate(d *sql.DB, dbType string) error {
	schema := SchemaFor(dbType)
	// 逐语句执行（兼容不支持多语句一次的驱动）
	for _, stmt := range strings.Split(schema, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := d.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// SchemaFor 返回当前数据库的建表 SQL。
func SchemaFor(dbType string) string {
	d := Dialect(dbType)
	now := d.NowLiteral()

	return fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS users (
  id %s,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (%s)
);

CREATE TABLE IF NOT EXISTS categories (
  id %s,
  name TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL DEFAULT (%s)
);

CREATE TABLE IF NOT EXISTS posts (
  id %s,
  title TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE,
  summary TEXT NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '',
  cover_url TEXT NOT NULL DEFAULT '',
  category_id BIGINT REFERENCES categories(id) ON DELETE SET NULL,
  status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published')),
  views %s NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (%s),
  updated_at TEXT NOT NULL DEFAULT (%s),
  published_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_posts_status ON posts(status, published_at DESC);
CREATE INDEX IF NOT EXISTS idx_posts_category ON posts(category_id);

CREATE TABLE IF NOT EXISTS comments (
  id %s,
  post_id BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  author TEXT NOT NULL,
  content TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
  created_at TEXT NOT NULL DEFAULT (%s)
);

CREATE INDEX IF NOT EXISTS idx_comments_post ON comments(post_id, status);
`,
		d.AutoIncrement(), now,
		d.AutoIncrement(), now,
		d.AutoIncrement(), d.ColumnType("integer"), now, now,
		d.AutoIncrement(), now,
	)
}

// NowISO 返回当前时间的 ISO 格式字符串（与 Node toISOString 对齐）。
func NowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// RewritePlaceholders 将 SQL 中的 ? 替换为对应数据库的占位符。
func RewritePlaceholders(dbType, sql string) string {
	return Dialect(dbType).RewritePlaceholders(sql)
}

// Placeholder 返回第 i 个占位符。
func Placeholder(dbType string, i int) string {
	return Dialect(dbType).Placeholder(i)
}

// ExecPlaceholder 执行带 ? 占位符的语句，自动适配数据库。
func ExecPlaceholder(d *sql.DB, dbType, sql string, args ...any) (sql.Result, error) {
	return d.Exec(RewritePlaceholders(dbType, sql), args...)
}

// QueryPlaceholder 执行带 ? 占位符的查询，自动适配数据库。
func QueryPlaceholder(d *sql.DB, dbType, sql string, args ...any) (*sql.Rows, error) {
	return d.Query(RewritePlaceholders(dbType, sql), args...)
}

// QueryRowPlaceholder 执行带 ? 占位符的单行查询，自动适配数据库。
func QueryRowPlaceholder(d *sql.DB, dbType, sql string, args ...any) *sql.Row {
	return d.QueryRow(RewritePlaceholders(dbType, sql), args...)
}

// ScanInt64 辅助：扫描单个 int64。
func ScanInt64(row *sql.Row) (int64, error) {
	var n int64
	err := row.Scan(&n)
	return n, err
}

// StrToInt64 辅助：字符串转 int64。
func StrToInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// BoolToInt 辅助：bool 转 int（SQLite/MySQL 用）。
func BoolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
