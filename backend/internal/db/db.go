package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"narratpage/internal/config"
	"narratpage/internal/dialect"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// Open 打开数据库连接并执行 schema 迁移（幂等）。
func Open(cfg config.Config) (*sql.DB, error) {
	var d *sql.DB
	var err error

	switch cfg.DBType {
	case "mysql":
		dsn := cfg.DBDSN
		if dsn == "" {
			dsn = fmt.Sprintf("narratpage:narratpage@tcp(127.0.0.1:3306)/narratpage?charset=utf8mb4&parseTime=true&loc=Local")
		}
		d, err = sql.Open("mysql", dsn)
	case "pgsql":
		dsn := cfg.DBDSN
		if dsn == "" {
			dsn = "postgres://narratpage:narratpage@127.0.0.1:5432/narratpage?sslmode=disable"
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

	if cfg.DBType == "mysql" || cfg.DBType == "pgsql" {
		d.SetMaxOpenConns(25)
		d.SetMaxIdleConns(5)
		d.SetConnMaxLifetime(5 * time.Minute)
	} else {
		// SQLite 单写者模型，连接数过多反而加剧锁竞争
		d.SetMaxOpenConns(4)
		d.SetMaxIdleConns(4)
	}

	if err := d.Ping(); err != nil {
		d.Close()
		return nil, err
	}

	if err := Migrate(d, cfg.DBType); err != nil {
		d.Close()
		return nil, err
	}

	// 旧版本库的列补齐：新增列对已存在的表需要 ALTER
	if err := backfillColumns(d, cfg.DBType); err != nil {
		d.Close()
		return nil, err
	}

	return d, nil
}

// Migrate 执行建表与索引语句。
//
// 幂等性：建表语句带 IF NOT EXISTS；索引语句在 SQLite / PostgreSQL 下
// 带 IF NOT EXISTS，而 MySQL 不支持该语法，故在 MySQL 下先查后建。
func Migrate(d *sql.DB, dbType string) error {
	for _, stmt := range SchemaFor(dbType) {
		// MySQL 索引需先判断是否已存在
		if dbType == "mysql" && strings.HasPrefix(stmt, "CREATE ") &&
			strings.Contains(stmt, "INDEX") {
			ok, err := mysqlIndexExists(d, stmt)
			if err != nil {
				return err
			}
			if ok {
				continue
			}
		}
		if _, err := d.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w (sql: %.80s)", err, stmt)
		}
	}
	return nil
}

// mysqlIndexExists 从 CREATE INDEX 语句中解析出表名与索引名并查询是否存在。
func mysqlIndexExists(d *sql.DB, stmt string) (bool, error) {
	name, table, ok := parseCreateIndex(stmt)
	if !ok {
		return false, nil
	}
	var n int
	err := d.QueryRow(
		"SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?",
		table, name).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("查询索引 %s 是否存在: %w", name, err)
	}
	return n > 0, nil
}

// parseCreateIndex 解析 `CREATE [UNIQUE] INDEX name ON table (cols)`。
func parseCreateIndex(stmt string) (name, table string, ok bool) {
	rest := stmt
	rest = strings.TrimPrefix(rest, "CREATE ")
	rest = strings.TrimPrefix(rest, "UNIQUE ")
	rest = strings.TrimPrefix(rest, "INDEX ")
	rest = strings.TrimPrefix(rest, "IF NOT EXISTS ")

	// 去掉可能的反引号/双引号包裹
	unquote := func(s string) string { return strings.Trim(s, "`\"") }

	nameEnd := strings.Index(rest, " ON ")
	if nameEnd < 0 {
		return "", "", false
	}
	name = unquote(strings.TrimSpace(rest[:nameEnd]))

	rest = rest[nameEnd+4:]
	rest = strings.TrimPrefix(rest, "ON ")
	rest = strings.TrimSpace(rest)

	tableEnd := strings.IndexAny(rest, " (")
	if tableEnd < 0 {
		return "", "", false
	}
	table = unquote(strings.TrimSpace(rest[:tableEnd]))
	return name, table, name != "" && table != ""
}

// backfillColumns 为已存在的旧库补齐新版本新增的列。
// CREATE TABLE IF NOT EXISTS 不会改动已存在的表，因此存量库
// （如从 v1 升级而来的部署）需要显式 ALTER 才能用上新字段。
func backfillColumns(d *sql.DB, dbType string) error {
	// 只针对「必然缺失且允许为空或带默认值」的列做补充。
	// 每项：表名 -> 列名 -> 列定义片段。
	additions := map[string][]struct {
		col string
		def string
	}{
		"users": {
			{"email", "VARCHAR(255) NOT NULL DEFAULT ''"},
			{"display_name", "VARCHAR(64) NOT NULL DEFAULT ''"},
			{"role", "VARCHAR(24) NOT NULL DEFAULT 'admin'"},
			{"bio", "TEXT"},
			{"avatar_url", "VARCHAR(512) NOT NULL DEFAULT ''"},
			{"active", boolDef(dbType)},
			{"last_login_at", "TEXT"},
		},
		"posts": {
			{"type", "VARCHAR(24) NOT NULL DEFAULT 'post'"},
			{"sticky", boolDef(dbType)},
			{"password", "VARCHAR(128) NOT NULL DEFAULT ''"},
			{"menu_order", "BIGINT NOT NULL DEFAULT 0"},
			{"author_id", "BIGINT"},
			{"template", "VARCHAR(64) NOT NULL DEFAULT ''"},
			{"scheduled_at", "TEXT"},
			{"deleted_at", "TEXT"},
		},
		"categories": {
			{"description", "TEXT"},
			{"is_page", "BIGINT NOT NULL DEFAULT 0"},
			{"position", "BIGINT NOT NULL DEFAULT 0"},
		},
		"comments": {
			{"parent_id", "BIGINT"},
			{"author_id", "BIGINT"},
			{"email", "VARCHAR(255) NOT NULL DEFAULT ''"},
			{"website", "VARCHAR(255) NOT NULL DEFAULT ''"},
			{"ip_hash", "VARCHAR(64) NOT NULL DEFAULT ''"},
			{"is_pingback", boolDef(dbType)},
		},
	}

	for table, cols := range additions {
		for _, c := range cols {
			exists, err := columnExists(d, dbType, table, c.col)
			if err != nil {
				return err
			}
			if exists {
				continue
			}
			stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, c.col, c.def)
			if _, err := d.Exec(stmt); err != nil {
				// 存量库可能因方言细节失败，不阻断启动：缺失列由上层按零值处理
				continue
			}
		}
	}
	return nil
}

func boolDef(dbType string) string {
	if dbType == "pgsql" {
		return "BOOLEAN NOT NULL DEFAULT false"
	}
	return "INTEGER NOT NULL DEFAULT 0"
}

// columnExists 报告某列是否已存在。跨库实现：
// 优先查信息模式，失败则回退到「SELECT 该列 LIMIT 0」。
func columnExists(d *sql.DB, dbType, table, column string) (bool, error) {
	var query string
	switch dbType {
	case "mysql":
		query = "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?"
	case "pgsql":
		query = "SELECT COUNT(*) FROM information_schema.columns WHERE table_name = $1 AND column_name = $2"
	default:
		query = "SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?"
	}

	var n int
	var err error
	switch dbType {
	case "pgsql":
		err = d.QueryRow(query, table, column).Scan(&n)
	default:
		err = d.QueryRow(query, table, column).Scan(&n)
	}
	if err == nil {
		return n > 0, nil
	}

	// 回退：直接试着选这一列
	q := fmt.Sprintf("SELECT %s FROM %s LIMIT 0", column, table)
	if _, err := d.Exec(q); err != nil {
		return false, nil
	}
	return true, nil
}

// Dialect 返回当前数据库方言。
func Dialect(dbType string) dialect.Dialect { return dialect.Dialect(dbType) }

// NowISO 返回当前时间的 ISO 格式字符串（与前端 toISOString 对齐）。
func NowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// ---------- 参数化执行辅助 ----------
//
// 业务层统一写 ? 占位符，PostgreSQL 下改写为 $n。

// RewritePlaceholders 将 SQL 中的 ? 替换为对应数据库的占位符。
func RewritePlaceholders(dbType, query string) string {
	return dialect.Dialect(dbType).RewritePlaceholders(query)
}

// Exec 执行带 ? 占位符的语句。
func Exec(d *sql.DB, dbType, query string, args ...any) (sql.Result, error) {
	return d.Exec(RewritePlaceholders(dbType, query), args...)
}

// Query 查询。
func Query(d *sql.DB, dbType, query string, args ...any) (*sql.Rows, error) {
	return d.Query(RewritePlaceholders(dbType, query), args...)
}

// QueryRow 单行查询。
func QueryRow(d *sql.DB, dbType, query string, args ...any) *sql.Row {
	return d.QueryRow(RewritePlaceholders(dbType, query), args...)
}

// OpenRaw 按 DSN 打开连接，不执行迁移。
// 用于测试等需要「先建 schema、再迁移」的场景。
func OpenRaw(dbType, dsn string) (*sql.DB, error) {
	driver := "sqlite"
	switch dbType {
	case "mysql":
		driver = "mysql"
	case "pgsql":
		driver = "postgres"
	}
	return sql.Open(driver, dsn)
}

// Exec2 在裸连接上执行一条语句（不依赖已迁移的表）。
// 供测试执行 CREATE DATABASE / CREATE SCHEMA 等 DDL 使用。
func Exec2(dsn, dbType, query string) (sql.Result, error) {
	d, err := OpenRaw(dbType, dsn)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.Exec(query)
}

// Increment 返回列自增 1 的 SQL 片段。
func Increment(col string) string { return col + " + 1" }

// Upsert 按唯一键插入或更新一行。
//
// conflictCols 是冲突目标列（必须对应表上的唯一索引）。
// MySQL 用 ON DUPLICATE KEY UPDATE 且可省略冲突目标；
// SQLite / PostgreSQL 的 ON CONFLICT DO UPDATE 必须显式给出
// 冲突目标，否则报「requires inference specification」。
func Upsert(d *sql.DB, dbType, table string, cols []string, values []any,
	updateCols []string, conflictCols []string) error {
	ph := strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")
	colList := strings.Join(cols, ", ")
	base := "INSERT INTO " + table + " (" + colList + ") VALUES (" + ph + ")"

	if len(updateCols) == 0 {
		// 冲突时什么都不做
		if dbType == "mysql" {
			_, err := Exec(d, dbType,
				"INSERT IGNORE INTO "+table+" ("+colList+") VALUES ("+ph+")", values...)
			return err
		}
		target := ""
		if len(conflictCols) > 0 {
			target = " (" + strings.Join(conflictCols, ", ") + ")"
		}
		_, err := Exec(d, dbType, base+" ON CONFLICT"+target+" DO NOTHING", values...)
		return err
	}

	sets := make([]string, 0, len(updateCols))
	for _, c := range updateCols {
		if dbType == "mysql" {
			sets = append(sets, c+" = VALUES("+c+")")
		} else {
			sets = append(sets, c+" = EXCLUDED."+c)
		}
	}
	if dbType == "mysql" {
		_, err := Exec(d, dbType, base+" ON DUPLICATE KEY UPDATE "+strings.Join(sets, ", "), values...)
		return err
	}
	if len(conflictCols) == 0 {
		return fmt.Errorf("upsert %s: 非 MySQL 必须指定冲突目标列", table)
	}
	target := " (" + strings.Join(conflictCols, ", ") + ")"
	_, err := Exec(d, dbType,
		base+" ON CONFLICT"+target+" DO UPDATE SET "+strings.Join(sets, ", "), values...)
	return err
}

// Insert 执行 INSERT 并返回新行 ID。
//
// 跨库要点：SQLite 3.35+ 与 PostgreSQL 支持 RETURNING id，
// MySQL 不支持，必须退回 result.LastInsertId()。这是本项目
// 多数据库支持的关键分叉点。
func Insert(d *sql.DB, dbType, table string, cols []string, values []any) (int64, error) {
	ph := strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		table, strings.Join(cols, ", "), ph)

	if Dialect(dbType).SupportsReturning() {
		var id int64
		err := QueryRow(d, dbType, query+" RETURNING id", values...).Scan(&id)
		return id, err
	}

	res, err := Exec(d, dbType, query, values...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// InsertOrIgnore 插入并在唯一键冲突时静默跳过，返回是否真的插入。
// 用于「按需关联」这类幂等写入（如标签绑定）。
func InsertOrIgnore(d *sql.DB, dbType, table string, cols []string, values []any) (bool, error) {
	ph := strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")
	var query string
	switch dbType {
	case "mysql":
		query = fmt.Sprintf("INSERT IGNORE INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ", "), ph)
	default:
		query = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING",
			table, strings.Join(cols, ", "), ph)
	}
	res, err := Exec(d, dbType, query, values...)
	if err != nil {
		return false, err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return false, nil
	}
	return true, nil
}

// Update 执行 UPDATE 并返回受影响行数。
func Update(d *sql.DB, dbType, table string, sets map[string]any, where string, args ...any) (int64, error) {
	if len(sets) == 0 {
		return 0, nil
	}
	cols := make([]string, 0, len(sets))
	vals := make([]any, 0, len(sets)+len(args))
	for _, c := range sortedKeys(sets) {
		cols = append(cols, c+" = ?")
		vals = append(vals, sets[c])
	}
	vals = append(vals, args...)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s", table, strings.Join(cols, ", "), where)
	res, err := Exec(d, dbType, query, vals...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Delete 执行 DELETE 并返回受影响行数。
func Delete(d *sql.DB, dbType, table, where string, args ...any) (int64, error) {
	res, err := Exec(d, dbType, "DELETE FROM "+table+" WHERE "+where, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 保证生成 SQL 稳定，便于测试断言
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// Count 执行 COUNT(*) 查询。
func Count(d *sql.DB, dbType, query string, args ...any) (int, error) {
	var n int
	err := QueryRow(d, dbType, query, args...).Scan(&n)
	return n, err
}

// Tx 在事务中执行 fn，出错自动回滚。
// SQLite 在并发写入时可能返回锁错误，调用方可据此重试。
func Tx(d *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// EnsureDataDir 确保数据目录存在。
func EnsureDataDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}
