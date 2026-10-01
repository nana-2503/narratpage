package repo

import (
	"database/sql"

	"narratpage/internal/db"
	"narratpage/internal/dialect"
)

// 本文件集中收口仓储层对 db 包的调用，避免每个文件重复传 dbType，
// 同时让仓储层内部保持「只关心业务、不关心方言」。

// Dialect 返回当前数据库方言，供仓储层处理类型转换。
func Dialect(dbType string) dialect.Dialect { return dialect.Dialect(dbType) }

func dbCount(d *sql.DB, dbType, query string, args ...any) (int, error) {
	return db.Count(d, dbType, query, args...)
}

func dbQuery(d *sql.DB, dbType, query string, args ...any) (*sql.Rows, error) {
	return db.Query(d, dbType, query, args...)
}

func dbQueryRow(d *sql.DB, dbType, query string, args ...any) *sql.Row {
	return db.QueryRow(d, dbType, query, args...)
}

func dbExec(d *sql.DB, dbType, query string, args ...any) (sql.Result, error) {
	return db.Exec(d, dbType, query, args...)
}

func dbInsert(d *sql.DB, dbType, table string, cols []string, values []any) (int64, error) {
	return db.Insert(d, dbType, table, cols, values)
}

func dbUpdate(d *sql.DB, dbType, table string, sets map[string]any, where string, args ...any) (int64, error) {
	return db.Update(d, dbType, table, sets, where, args...)
}

func dbDelete(d *sql.DB, dbType, table, where string, args ...any) (int64, error) {
	return db.Delete(d, dbType, table, where, args...)
}
