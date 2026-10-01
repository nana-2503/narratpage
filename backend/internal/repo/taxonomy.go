package repo

import (
	"database/sql"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"narratpage/internal/db"
	"narratpage/internal/models"
)

// Taxonomy 分类与标签的公共逻辑。
type Taxonomy struct {
	DB     *sql.DB
	DBType string
}

// NewTaxonomy 创建分类/标签仓储。
func NewTaxonomy(database *sql.DB, dbType string) *Taxonomy {
	return &Taxonomy{DB: database, DBType: dbType}
}

// ---------- 分类 ----------

// ListCategories 列出分类及其文章数。
func (t *Taxonomy) ListCategories(isPage int, v Viewer) ([]models.Category, error) {
	query := `SELECT c.id, c.name, c.slug, COALESCE(c.description, ''), c.is_page, c.position,
		(SELECT COUNT(*) FROM posts p WHERE p.category_id = c.id
		   AND p.status = 'published' AND p.deleted_at IS NULL) AS post_count
		FROM categories c WHERE c.is_page = ? ORDER BY c.position ASC, c.id ASC`
	rows, err := db.Query(t.DB, t.DBType, query, isPage)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Category{}
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Description,
			&c.IsPage, &c.Position, &c.PostCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// CategoryByID 按 ID 取分类。
func (t *Taxonomy) CategoryByID(id int64) (*models.Category, error) {
	var c models.Category
	err := db.QueryRow(t.DB, t.DBType,
		"SELECT id, name, slug, COALESCE(description,''), is_page, position FROM categories WHERE id = ?", id).
		Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.IsPage, &c.Position)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CategoryBySlug 按 slug 取分类。
func (t *Taxonomy) CategoryBySlug(slug string) (*models.Category, error) {
	var c models.Category
	err := db.QueryRow(t.DB, t.DBType,
		"SELECT id, name, slug, COALESCE(description,''), is_page, position FROM categories WHERE slug = ?", slug).
		Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.IsPage, &c.Position)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateCategory 新建分类。
func (t *Taxonomy) CreateCategory(name, slug, description string, isPage int) (int64, error) {
	if slug == "" {
		slug = Slugify(name, "cat")
	}
	return db.Insert(t.DB, t.DBType, "categories",
		[]string{"name", "slug", "description", "is_page", "created_at"},
		[]any{name, slug, description, isPage, db.NowISO()})
}

// UpdateCategory 更新分类。
func (t *Taxonomy) UpdateCategory(id int64, name, slug, description string) error {
	_, err := db.Update(t.DB, t.DBType, "categories",
		map[string]any{"name": name, "slug": slug, "description": description},
		"id = ?", id)
	return err
}

// ReorderCategories 批量更新排序。
func (t *Taxonomy) ReorderCategories(ids []int64) error {
	return db.Tx(t.DB, func(tx *sql.Tx) error {
		for i, id := range ids {
			if _, err := tx.Exec(db.RewritePlaceholders(t.DBType,
				"UPDATE categories SET position = ? WHERE id = ?"), i, id); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteCategory 删除分类（文章落为未分类）。
func (t *Taxonomy) DeleteCategory(id int64) (int64, error) {
	return db.Delete(t.DB, t.DBType, "categories", "id = ?", id)
}

// CategorySlugTaken 报告分类 slug 是否被占用。
func (t *Taxonomy) CategorySlugTaken(slug string, exceptID int64) (bool, error) {
	n, err := db.Count(t.DB, t.DBType,
		"SELECT COUNT(*) FROM categories WHERE slug = ? AND id != ?", slug, exceptID)
	return n > 0, err
}

// ---------- 标签 ----------

// ListTags 列出标签及使用次数。
func (t *Taxonomy) ListTags(search string) ([]models.Tag, error) {
	query := `SELECT t.id, t.name, t.slug, COALESCE(t.description, ''),
		(SELECT COUNT(*) FROM post_tags pt JOIN posts p ON p.id = pt.post_id
		  WHERE pt.tag_id = t.id AND p.status = 'published' AND p.deleted_at IS NULL) AS post_count
		FROM tags t`
	var args []any
	if search != "" {
		like := "%" + dialectEscape(search) + "%"
		query += " WHERE t.name LIKE ? ESCAPE '!' OR t.slug LIKE ? ESCAPE '!'"
		args = append(args, like, like)
	}
	query += " ORDER BY post_count DESC, t.name ASC"
	rows, err := db.Query(t.DB, t.DBType, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Tag{}
	for rows.Next() {
		var tag models.Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Slug, &tag.Description, &tag.PostCount); err != nil {
			return nil, err
		}
		out = append(out, tag)
	}
	return out, nil
}

// TagBySlug 按 slug 取标签。
func (t *Taxonomy) TagBySlug(slug string) (*models.Tag, error) {
	var tag models.Tag
	err := db.QueryRow(t.DB, t.DBType,
		"SELECT id, name, slug, COALESCE(description,'') FROM tags WHERE slug = ?", slug).
		Scan(&tag.ID, &tag.Name, &tag.Slug, &tag.Description)
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

// CreateTag 新建标签，返回 ID。
//
// 幂等：先按 slug 查、再按名称查，命中则复用。
// 仅按 slug 去重是不够的——同一个中文名可能有不同 slug
// （例如手写的 "frontend" 与 Slugify 生成的 "前端"），
// 那会导致同名标签重复堆积。
func (t *Taxonomy) CreateTag(name, slug, description string) (int64, error) {
	if slug == "" {
		slug = Slugify(name, "tag")
	}
	if id, err := t.tagIDBySlug(slug); err == nil {
		return id, nil
	}
	if id, err := t.tagIDByName(name); err == nil {
		return id, nil
	}
	return db.Insert(t.DB, t.DBType, "tags",
		[]string{"name", "slug", "description", "created_at"},
		[]any{name, slug, description, db.NowISO()})
}

func (t *Taxonomy) tagIDByName(name string) (int64, error) {
	var id int64
	err := db.QueryRow(t.DB, t.DBType,
		"SELECT id FROM tags WHERE name = ? ORDER BY id ASC", name).Scan(&id)
	return id, err
}

func (t *Taxonomy) tagIDBySlug(slug string) (int64, error) {
	var id int64
	err := db.QueryRow(t.DB, t.DBType, "SELECT id FROM tags WHERE slug = ?", slug).Scan(&id)
	return id, err
}

// UpdateTag 更新标签。
func (t *Taxonomy) UpdateTag(id int64, name, slug, description string) error {
	_, err := db.Update(t.DB, t.DBType, "tags",
		map[string]any{"name": name, "slug": slug, "description": description}, "id = ?", id)
	return err
}

// DeleteTag 删除标签及其关联。
func (t *Taxonomy) DeleteTag(id int64) (int64, error) {
	if _, err := db.Delete(t.DB, t.DBType, "post_tags", "tag_id = ?", id); err != nil {
		return 0, err
	}
	return db.Delete(t.DB, t.DBType, "tags", "id = ?", id)
}

// TagSlugTaken 报告标签 slug 是否被占用。
func (t *Taxonomy) TagSlugTaken(slug string, exceptID int64) (bool, error) {
	n, err := db.Count(t.DB, t.DBType,
		"SELECT COUNT(*) FROM tags WHERE slug = ? AND id != ?", slug, exceptID)
	return n > 0, err
}

// ---------- 文章-标签关联 ----------

// SetPostTags 全量设置某文章的标签。
func (t *Taxonomy) SetPostTags(postID int64, tagIDs []int64) error {
	if _, err := db.Delete(t.DB, t.DBType, "post_tags", "post_id = ?", postID); err != nil {
		return err
	}
	seen := map[int64]bool{}
	for _, tid := range tagIDs {
		if tid <= 0 || seen[tid] {
			continue
		}
		seen[tid] = true
		if _, err := db.InsertOrIgnore(t.DB, t.DBType, "post_tags",
			[]string{"post_id", "tag_id"}, []any{postID, tid}); err != nil {
			return err
		}
	}
	return nil
}

// AddPostTag 追加单个标签。
func (t *Taxonomy) AddPostTag(postID, tagID int64) error {
	_, err := db.InsertOrIgnore(t.DB, t.DBType, "post_tags",
		[]string{"post_id", "tag_id"}, []any{postID, tagID})
	return err
}

// RemovePostTag 移除单个标签。
func (t *Taxonomy) RemovePostTag(postID, tagID int64) error {
	_, err := db.Delete(t.DB, t.DBType, "post_tags", "post_id = ? AND tag_id = ?", postID, tagID)
	return err
}

// PostTagIDs 取某文章的全部标签 ID。
func (t *Taxonomy) PostTagIDs(postID int64) ([]int64, error) {
	rows, err := db.Query(t.DB, t.DBType, "SELECT tag_id FROM post_tags WHERE post_id = ?", postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// ResolveTagIDs 按名称或 slug 解析标签 ID，不存在则创建。
// 前端可直接传标签名数组，由后端负责归一化。
func (t *Taxonomy) ResolveTagIDs(names []string) ([]int64, error) {
	seen := map[string]bool{}
	out := []int64{}
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		slug := Slugify(name, "tag")
		id, err := t.CreateTag(name, slug, "")
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// Slugify 将标题转为 URL 友好的 slug。
//
// 保留字母（含 CJK）与数字，其余统一转为连字符。
// 注意：不能用 `r < 0x3000` 之类的范围判断来排除 CJK——
// 那会把所有汉字过滤掉，中文标题只能得到时间戳 slug，
// 且每次生成都不同（表现为标签重复堆积）。
// 结果为空时（纯符号输入）用 prefix + 时间戳兜底。
func Slugify(text, prefix string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(text)) {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	slug := strings.Trim(collapseDashes(b.String()), "-")
	if slug == "" {
		return prefix + "-" + strconv.FormatInt(nowMilli(), 36)
	}
	// 超长截断，避免超出数据库列宽
	if len(slug) > 180 {
		slug = strings.Trim(slug[:180], "-")
	}
	return slug
}

func collapseDashes(s string) string {
	var b strings.Builder
	prevDash := false
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			if prevDash {
				continue
			}
			prevDash = true
		} else {
			prevDash = false
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// UniqueSlug 确保 slug 唯一：占用时追加递增后缀。
func (t *Taxonomy) UniqueSlug(table, base string) (string, error) {
	candidate := base
	for i := 2; i < 200; i++ {
		n, err := db.Count(t.DB, t.DBType,
			"SELECT COUNT(*) FROM "+table+" WHERE slug = ?", candidate)
		if err != nil {
			return "", err
		}
		if n == 0 {
			return candidate, nil
		}
		candidate = base + "-" + strconv.Itoa(i)
	}
	return candidate, nil
}

// SortTags 按名称排序，保证输出稳定。
func SortTags(tags []models.Tag) {
	sort.Slice(tags, func(i, j int) bool { return tags[i].Name < tags[j].Name })
}
