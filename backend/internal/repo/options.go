package repo

import (
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"

	"narratpage/internal/db"
	"narratpage/internal/models"
)

// Options 读写站点设置。
type Options struct {
	DB     *sql.DB
	DBType string
}

// NewOptions 创建设置仓储。
func NewOptions(database *sql.DB, dbType string) *Options {
	return &Options{DB: database, DBType: dbType}
}

// optionKey 前缀，避免与将来其他键值用途冲突。
const optionKey = "site_"

// Get 读取单个设置项。
func (o *Options) Get(key string) (string, bool) {
	var v sql.NullString
	err := db.QueryRow(o.DB, o.DBType,
		"SELECT option_value FROM options WHERE option_key = ?", optionKey+key).Scan(&v)
	if err != nil {
		return "", false
	}
	return v.String, v.Valid
}

// Set 写入单个设置项（存在则更新）。
func (o *Options) Set(key, value string) error {
	return db.Upsert(o.DB, o.DBType, "options",
		[]string{"option_key", "option_value", "updated_at"},
		[]any{optionKey + key, value, db.NowISO()},
		[]string{"option_value", "updated_at"},
		[]string{"option_key"},
	)
}

// Site 读取完整站点设置，并与默认值合并。
func (o *Options) Site() models.SiteOptions {
	site := models.DefaultSiteOptions()

	if v, ok := o.Get("config"); ok && v != "" {
		var stored models.SiteOptions
		if err := json.Unmarshal([]byte(v), &stored); err == nil {
			// 以存储值覆盖默认值，但保留统计字段
			stored.PostCount = 0
			stored.PageCount = 0
			stored.CommentCount = 0
			stored.TagCount = 0
			if stored.Links == nil {
				stored.Links = []models.SiteLink{}
			}
			return stored
		}
	}
	return site
}

// SaveSite 保存站点设置。
func (o *Options) SaveSite(site models.SiteOptions) error {
	data, err := json.Marshal(site)
	if err != nil {
		return err
	}
	return o.Set("config", string(data))
}

// EnsureDefaults 首次启动时写入默认设置。
func (o *Options) EnsureDefaults() error {
	if _, ok := o.Get("config"); ok {
		return nil
	}
	return o.SaveSite(models.DefaultSiteOptions())
}

// ---------- 统计 ----------

// Expr 生成自增表达式（MySQL 与 PostgreSQL 的 upsert 差异已在
// Upsert 中收敛，这里仅处理自增本身）。
func Expr(col, dbType, op string) string {
	return col + " " + op + " 1"
}

// CountRows 统计某表行数。
func (o *Options) CountRows(table string) int {
	n, _ := db.Count(o.DB, o.DBType, "SELECT COUNT(*) FROM "+table)
	return n
}

// Stats 汇总站点统计数字。
func (o *Options) Stats() models.SiteOptions {
	site := o.Site()
	site.PostCount = o.CountRows("posts")
	site.PageCount = o.publishedTypeCount("page")
	site.CommentCount = o.publishedCommentCount()
	site.TagCount = o.CountRows("tags")
	return site
}

func (o *Options) publishedTypeCount(typ string) int {
	n, _ := db.Count(o.DB, o.DBType,
		"SELECT COUNT(*) FROM posts WHERE type = ? AND status = 'published' AND deleted_at IS NULL",
		typ)
	return n
}

func (o *Options) publishedCommentCount() int {
	n, _ := db.Count(o.DB, o.DBType,
		"SELECT COUNT(*) FROM comments WHERE status = 'approved'")
	return n
}

// ---------- 键值直读（供其他模块使用） ----------

// String 读取字符串项。
func (o *Options) String(key, def string) string {
	if v, ok := o.Get(key); ok && v != "" {
		return v
	}
	return def
}

// Bool 读取布尔项。
func (o *Options) Bool(key string, def bool) bool {
	if v, ok := o.Get(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

// Int 读取整数项。
func (o *Options) Int(key string, def int) int {
	if v, ok := o.Get(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}
