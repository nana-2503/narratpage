package repo

import (
	"database/sql"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"narratpage/internal/db"
	"narratpage/internal/models"
)

// Media 媒体仓储。
type Media struct {
	DB     *sql.DB
	DBType string
}

func NewMedia(database *sql.DB, dbType string) *Media {
	return &Media{DB: database, DBType: dbType}
}

const mediaColumns = `id, filename, url, mime, size, title, alt_text,
	COALESCE(caption, ''), width, height, uploaded_by, created_at`

func scanMedia(row interface{ Scan(...any) error }) (*models.Media, error) {
	var m models.Media
	if err := row.Scan(&m.ID, &m.Filename, &m.URL, &m.Mime, &m.Size, &m.Title,
		&m.AltText, &m.Caption, &m.Width, &m.Height, &m.UploadedBy, &m.CreatedAt); err != nil {
		return nil, err
	}
	return &m, nil
}

// Create 登记一条媒体记录。
func (m *Media) Create(item *models.Media) (int64, error) {
	return db.Insert(m.DB, m.DBType, "media",
		[]string{"filename", "url", "mime", "size", "title", "alt_text", "caption",
			"width", "height", "uploaded_by", "created_at"},
		[]any{item.Filename, item.URL, item.Mime, item.Size, item.Title, item.AltText,
			item.Caption, item.Width, item.Height, item.UploadedBy, db.NowISO()})
}

// List 分页列出媒体。
func (m *Media) List(page, pageSize int, mimePrefix, search string) (models.Paged[models.Media], error) {
	where := []string{"1 = 1"}
	var args []any
	if mimePrefix != "" {
		where = append(where, "mime LIKE ?")
		args = append(args, mimePrefix+"%")
	}
	if search != "" {
		like := "%" + dialectEscape(search) + "%"
		where = append(where, "(filename LIKE ? ESCAPE '!' OR title LIKE ? ESCAPE '!' OR alt_text LIKE ? ESCAPE '!')")
		args = append(args, like, like, like)
	}
	whereSQL := "WHERE " + strings.Join(where, " AND ")

	total, err := db.Count(m.DB, m.DBType, "SELECT COUNT(*) FROM media "+whereSQL, args...)
	if err != nil {
		return models.Paged[models.Media]{}, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 24
	}
	rows, err := db.Query(m.DB, m.DBType,
		"SELECT "+mediaColumns+" FROM media "+whereSQL+" ORDER BY id DESC LIMIT "+
			itoa(pageSize)+" OFFSET "+itoa((page-1)*pageSize), args...)
	if err != nil {
		return models.Paged[models.Media]{}, err
	}
	defer rows.Close()
	items := []models.Media{}
	for rows.Next() {
		it, err := scanMedia(rows)
		if err != nil {
			return models.Paged[models.Media]{}, err
		}
		items = append(items, *it)
	}
	return models.NewPaged(items, total, page, pageSize), nil
}

// ByID 按 ID 取媒体。
func (m *Media) ByID(id int64) (*models.Media, error) {
	return scanMedia(db.QueryRow(m.DB, m.DBType,
		"SELECT "+mediaColumns+" FROM media WHERE id = ?", id))
}

// ByFilename 按文件名取媒体。
func (m *Media) ByFilename(name string) (*models.Media, error) {
	return scanMedia(db.QueryRow(m.DB, m.DBType,
		"SELECT "+mediaColumns+" FROM media WHERE filename = ?", name))
}

// Update 更新媒体元信息。
func (m *Media) Update(id int64, title, alt, caption string) error {
	_, err := db.Update(m.DB, m.DBType, "media",
		map[string]any{"title": title, "alt_text": alt, "caption": caption}, "id = ?", id)
	return err
}

// Delete 删除媒体记录。
func (m *Media) Delete(id int64) (int64, error) {
	return db.Delete(m.DB, m.DBType, "media", "id = ?", id)
}

// Stats 统计媒体数量与总大小。
func (m *Media) Stats() (count int, totalSize int64) {
	_ = db.QueryRow(m.DB, m.DBType,
		"SELECT COUNT(*), COALESCE(SUM(size), 0) FROM media").Scan(&count, &totalSize)
	return
}

// PostsUsing 引用了该媒体的文章数，用于删除前提示。
func (m *Media) PostsUsing(url string) int {
	n, _ := db.Count(m.DB, m.DBType,
		"SELECT COUNT(*) FROM posts WHERE cover_url = ? OR content LIKE ?",
		url, "%"+url+"%")
	return n
}

// ---------- 文章元数据 ----------

// PostMeta 读写文章的扩展字段。
type PostMeta struct {
	DB     *sql.DB
	DBType string
}

func NewPostMeta(database *sql.DB, dbType string) *PostMeta {
	return &PostMeta{DB: database, DBType: dbType}
}

// Set 写入元数据。
func (p *PostMeta) Set(postID int64, key, value string) error {
	return db.Upsert(p.DB, p.DBType, "postmeta",
		[]string{"post_id", "meta_key", "meta_value"},
		[]any{postID, key, value},
		[]string{"meta_value"},
		[]string{"post_id", "meta_key"},
	)
}

// Get 读取元数据。
func (p *PostMeta) Get(postID int64, key string) (string, bool) {
	var v string
	err := db.QueryRow(p.DB, p.DBType,
		"SELECT meta_value FROM postmeta WHERE post_id = ? AND meta_key = ?", postID, key).Scan(&v)
	if err != nil {
		return "", false
	}
	return v, true
}

// All 读取文章全部元数据。
func (p *PostMeta) All(postID int64) (map[string]string, error) {
	rows, err := db.Query(p.DB, p.DBType,
		"SELECT meta_key, meta_value FROM postmeta WHERE post_id = ?", postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, nil
}

// DeleteKey 删除单个元数据键。
func (p *PostMeta) DeleteKey(postID int64, key string) error {
	_, err := db.Delete(p.DB, p.DBType, "postmeta", "post_id = ? AND meta_key = ?", postID, key)
	return err
}

// ---------- 历史版本 ----------

// Revisions 历史版本仓储。
type Revisions struct {
	DB     *sql.DB
	DBType string
}

func NewRevisions(database *sql.DB, dbType string) *Revisions {
	return &Revisions{DB: database, DBType: dbType}
}

// Save 保存一个版本快照。
func (r *Revisions) Save(postID int64, authorID *int64, title, content, excerpt string) (int64, error) {
	return db.Insert(r.DB, r.DBType, "revisions",
		[]string{"post_id", "author_id", "title", "content", "excerpt", "created_at"},
		[]any{postID, authorID, title, content, excerpt, db.NowISO()})
}

// List 列出某文章的历史版本。
func (r *Revisions) List(postID int64, limit int) ([]models.Revision, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	rows, err := db.Query(r.DB, r.DBType,
		"SELECT id, post_id, author_id, title, content, COALESCE(excerpt,''), created_at "+
			"FROM revisions WHERE post_id = ? ORDER BY id DESC LIMIT "+itoa(limit), postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Revision{}
	for rows.Next() {
		var rev models.Revision
		if err := rows.Scan(&rev.ID, &rev.PostID, &rev.AuthorID, &rev.Title,
			&rev.Content, &rev.Excerpt, &rev.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rev)
	}
	return out, nil
}

// ByID 取单个版本。
func (r *Revisions) ByID(id int64) (*models.Revision, error) {
	var rev models.Revision
	err := db.QueryRow(r.DB, r.DBType,
		"SELECT id, post_id, author_id, title, content, COALESCE(excerpt,''), created_at "+
			"FROM revisions WHERE id = ?", id).
		Scan(&rev.ID, &rev.PostID, &rev.AuthorID, &rev.Title, &rev.Content, &rev.Excerpt, &rev.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &rev, nil
}

// Delete 删除单个版本。
func (r *Revisions) Delete(id int64) (int64, error) {
	return db.Delete(r.DB, r.DBType, "revisions", "id = ?", id)
}

// Prune 保留最近 keep 个版本。
func (r *Revisions) Prune(postID int64, keep int) error {
	_, err := db.Exec(r.DB, r.DBType,
		"DELETE FROM revisions WHERE post_id = ? AND id NOT IN "+
			"(SELECT id FROM revisions WHERE post_id = ? ORDER BY id DESC LIMIT ?)",
		postID, postID, keep)
	return err
}

// ---------- 重定向 ----------

// Redirects 重定向仓储。
type Redirects struct {
	DB     *sql.DB
	DBType string
}

func NewRedirects(database *sql.DB, dbType string) *Redirects {
	return &Redirects{DB: database, DBType: dbType}
}

// List 列出全部重定向。
func (r *Redirects) List() ([]models.Redirect, error) {
	rows, err := db.Query(r.DB, r.DBType,
		"SELECT id, from_path, to_path, hits, enabled, created_at FROM redirects ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Redirect{}
	for rows.Next() {
		var d models.Redirect
		var enabled int
		if err := rows.Scan(&d.ID, &d.FromPath, &d.ToPath, &d.Hits, &enabled, &d.CreatedAt); err != nil {
			return nil, err
		}
		d.Enabled = enabled == 1
		out = append(out, d)
	}
	return out, nil
}

// Create 新增重定向。
func (r *Redirects) Create(from, to string) (int64, error) {
	return db.Insert(r.DB, r.DBType, "redirects",
		[]string{"from_path", "to_path", "enabled", "created_at"},
		[]any{from, to, 1, db.NowISO()})
}

// Delete 删除重定向。
func (r *Redirects) Delete(id int64) (int64, error) {
	return db.Delete(r.DB, r.DBType, "redirects", "id = ?", id)
}

// Find 查询命中的重定向。
func (r *Redirects) Find(path string) (string, bool) {
	var to string
	err := db.QueryRow(r.DB, r.DBType,
		"SELECT to_path FROM redirects WHERE from_path = ? AND enabled = ?", path, 1).Scan(&to)
	if err != nil {
		return "", false
	}
	return to, true
}

// Hit 记录一次跳转。
func (r *Redirects) Hit(id int64) {
	_, _ = db.Update(r.DB, r.DBType, "redirects",
		map[string]any{"hits": db.Increment("hits")}, "id = ?", id)
}

// ---------- 工具 ----------

// ErrNotFound 表示目标不存在。
var ErrNotFound = errors.New("not found")

// nowISO 当前时间（UTC ISO-8601），与模型层保持同一时间基准。
func nowISO() string { return db.NowISO() }

func nowMilli() int64 { return time.Now().UnixMilli() }

// SortStrings 按字典序排序字符串切片。
func SortStrings(s []string) { sort.Strings(s) }

// Itoa 整数转字符串。
func Itoa(n int) string { return strconv.Itoa(n) }
