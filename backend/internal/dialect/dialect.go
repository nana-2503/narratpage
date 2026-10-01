// Package dialect 抽象 SQLite / MySQL / PostgreSQL 三库差异。
//
// 业务层统一使用 ? 占位符，PostgreSQL 下由 RewritePlaceholders
// 改写为 $1；插入语句统一走 db.Insert，由本包决定用
// RETURNING 还是 LastInsertId。
package dialect

import (
	"strconv"
	"strings"
)

type Dialect string

const (
	SQLite     Dialect = "sqlite"
	MySQL      Dialect = "mysql"
	PostgreSQL Dialect = "pgsql"
)

func (d Dialect) String() string { return string(d) }

// ---------- 占位符 ----------

// Placeholder 返回第 i 个占位符（从 1 开始）。
// SQLite/MySQL 用 ?；PostgreSQL 用 $1, $2, ...
func (d Dialect) Placeholder(i int) string {
	if d == PostgreSQL {
		return "$" + strconv.Itoa(i)
	}
	return "?"
}

// RewritePlaceholders 将 SQL 中的 ? 替换为当前数据库的占位符。
// 业务 SQL 中不存在含 ? 的字符串字面量，故不做词法分析。
func (d Dialect) RewritePlaceholders(query string) string {
	if d != PostgreSQL {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 8)
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ---------- 能力开关 ----------

// SupportsReturning 报告该库是否支持 INSERT ... RETURNING。
// SQLite 3.35+ 与 PostgreSQL 支持；MySQL 不支持，需退回 LastInsertId。
func (d Dialect) SupportsReturning() bool { return d != MySQL }

// SupportsCTE 报告是否支持 WITH 子句（SQLite / PostgreSQL / MySQL 8 均支持）。
func (d Dialect) SupportsCTE() bool { return true }

// ---------- LIKE 转义 ----------

// LikeEscape 是统一的 LIKE 转义字符。
// 刻意不用反斜杠：反斜杠在 MySQL 字符串字面量中本身是转义符，
// 写成 ESCAPE '\' 会得到未闭合的字面量。
const LikeEscape = "!"

// EscapeLike 转义 LIKE 通配符，使其按字面量参与匹配。
func EscapeLike(v string) string {
	var b strings.Builder
	b.Grow(len(v) + 8)
	for _, r := range v {
		if r == '!' || r == '%' || r == '_' {
			b.WriteByte('!')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// LikeClause 返回 ` LIKE ? ESCAPE '!'` 片段（含前导空格）。
func LikeClause(column string) string {
	return " " + column + " LIKE ? ESCAPE '" + LikeEscape + "'"
}

// ---------- 标识符与类型 ----------

// Quote 引用标识符，避免与保留字冲突。
func (d Dialect) Quote(ident string) string {
	if d == MySQL {
		return "`" + ident + "`"
	}
	return `"` + ident + `"`
}

// Text 无界文本类型。MySQL 的 TEXT 列不允许带 DEFAULT，故类型方法
// 不提供默认值，调用方需按需改用 Varchar。
func (d Dialect) Text() string { return "TEXT" }

// Varchar 定长上限文本类型，可安全携带 DEFAULT。
func (d Dialect) Varchar(n int) string { return "VARCHAR(" + strconv.Itoa(n) + ")" }

// BigInt 64 位整数。
func (d Dialect) BigInt() string { return "BIGINT" }

// Bool 布尔类型（MySQL/SQLite 用 0/1 整数存储）。
func (d Dialect) Bool() string {
	if d == PostgreSQL {
		return "BOOLEAN"
	}
	return "INTEGER"
}

// Timestamp 时间戳：以 UTC ISO-8601 文本存储，三库一致且可按字典序排序。
// 刻意用 VARCHAR 而非 TEXT：MySQL 不允许 TEXT 列带 DEFAULT，
// 而时间戳列需要 DEFAULT 当前时间。
func (d Dialect) Timestamp() string { return "VARCHAR(32)" }

// NowLiteral 插入当前时间的 SQL 字面量。
func (d Dialect) NowLiteral() string {
	if d == SQLite {
		return "strftime('%Y-%m-%dT%H:%M:%fZ', 'now')"
	}
	return "NOW()"
}

// BoolLiteral 返回布尔值的存储字面量。
func (d Dialect) BoolLiteral(v bool) string {
	if d == PostgreSQL {
		if v {
			return "true"
		}
		return "false"
	}
	if v {
		return "1"
	}
	return "0"
}

// QuoteBool 将 Go bool 转为该库可绑定的参数值。
func (d Dialect) QuoteBool(v bool) any {
	if d == PostgreSQL {
		return v
	}
	if v {
		return 1
	}
	return 0
}

// ---------- 排序 / 限制 ----------

// Desc 返回 `col DESC` 片段。
func Desc(col string) string { return col + " DESC" }

// Asc 返回 `col ASC` 片段。
func Asc(col string) string { return col + " ASC" }

// LimitOffset 返回 LIMIT/OFFSET 片段；limit<=0 表示不限制。
// 三库语法一致（MySQL 的 LIMIT n OFFSET m 亦被支持）。
func LimitOffset(limit, offset int) string {
	switch {
	case limit <= 0 && offset <= 0:
		return ""
	case limit <= 0:
		// 无上限时用 PG/SQLite 接受的最大值占位
		return " LIMIT 9223372036854775807 OFFSET " + strconv.Itoa(offset)
	case offset <= 0:
		return " LIMIT " + strconv.Itoa(limit)
	default:
		return " LIMIT " + strconv.Itoa(limit) + " OFFSET " + strconv.Itoa(offset)
	}
}
