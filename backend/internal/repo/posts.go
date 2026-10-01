// Package repo 提供内容查询与写入的仓储层。
//
// 把「哪些内容对谁可见」这类规则集中在此，避免各 handler 各自拼
// WHERE 条件而出现权限泄露。
package repo

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"narratpage/internal/db"
	"narratpage/internal/models"
)

// Repo 仓储。
type Repo struct {
	DB     *sql.DB
	DBType string
}

// New 创建仓储。
func New(database *sql.DB, dbType string) *Repo {
	return &Repo{DB: database, DBType: dbType}
}

func (r *Repo) q(query string) string { return db.RewritePlaceholders(r.DBType, query) }

// ---------- 可见性规则 ----------

// Viewer 描述请求者身份，用于可见性过滤。
type Viewer struct {
	UserID       int64
	Role         string
	IsAdmin      bool
	CanSeeDrafts bool
}

// PublicViewer 游客视角：仅已发布且已到发布时间的公开内容。
func PublicViewer() Viewer { return Viewer{} }

// AdminViewer 管理员视角：可见全部未删除内容。
func AdminViewer(userID int64, role string) Viewer {
	return Viewer{
		UserID: userID, Role: role, IsAdmin: true, CanSeeDrafts: true,
	}
}

// visibleClause 返回内容可见性的 SQL 片段与参数。
//
// 规则（与 WordPress 的公开查询语义对齐）：
//   - 回收站内容任何角色都看不到（需显式 ?trash=1 查询）
//   - 游客/订阅者：仅 status=published 且 scheduled_at 已到期
//   - 作者：可看自己的全部未删除内容
//   - 编辑/管理员：可看全部未删除内容
func (r *Repo) visibleClause(v Viewer, alias string) (string, []any) {
	if alias != "" {
		alias += "."
	}
	var conds []string
	var args []any

	conds = append(conds, alias+"deleted_at IS NULL")

	switch {
	case v.IsAdmin:
		// 可见全部（含草稿），但排除回收站
	case v.CanSeeDrafts:
		conds = append(conds, "("+alias+"status = 'published' OR "+alias+"author_id = ?)")
		args = append(args, v.UserID)
	default:
		conds = append(conds, alias+"status = 'published'")
		// 定时发布：到点前不公开
		conds = append(conds, "("+alias+"scheduled_at IS NULL OR "+alias+"scheduled_at <= ?)")
		args = append(args, db.NowISO())
	}
	return "(" + strings.Join(conds, " AND ") + ")", args
}

// ---------- 列表查询 ----------

// PostQuery 列表查询条件。
type PostQuery struct {
	Type     models.ContentType
	Status   string // 显式状态过滤（后台用）
	Category string // 分类 slug
	Tag      string // 标签 slug
	Author   int64  // 作者 ID
	Search   string
	Year     int
	Month    int
	Sticky   bool
	Page     int
	PageSize int
	Order    string // "" | "asc" | "meta"
	Exclude  []int64
}

// postColumns 内容查询的公共列。
const postColumns = `p.id, p.title, p.slug, p.summary, p.content, p.cover_url,
	p.category_id, p.status, p.type, p.sticky, p.password, p.menu_order,
	p.author_id, p.views, p.template, p.created_at, p.updated_at,
	p.published_at, p.scheduled_at, p.deleted_at`

func (r *Repo) scanPost(rows interface{ Scan(...any) error }) (*models.Post, error) {
	var p models.Post
	var typ, status string
	err := rows.Scan(
		&p.ID, &p.Title, &p.Slug, &p.Summary, &p.Content, &p.CoverURL,
		&p.CategoryID, &status, &typ, &p.Sticky, &p.Password, &p.MenuOrder,
		&p.AuthorID, &p.Views, &p.Template, &p.CreatedAt, &p.UpdatedAt,
		&p.PublishedAt, &p.ScheduledAt, &p.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	p.Status = models.ContentStatus(status)
	p.Type = models.ContentType(typ)
	p.HasPassword = p.Password != ""
	p.Password = ""
	return &p, nil
}

// ListPosts 分页查询内容列表。
func (r *Repo) ListPosts(q PostQuery, v Viewer) (models.Paged[models.Post], error) {
	where := []string{}
	var args []any

	vis, visArgs := r.visibleClause(v, "p")
	where = append(where, vis)
	args = append(args, visArgs...)

	if q.Type != "" {
		where = append(where, "p.type = ?")
		args = append(args, string(q.Type))
	}
	if q.Status != "" {
		// 回收站需显式请求
		if q.Status == string(models.StatusTrash) {
			where = []string{"p.deleted_at IS NOT NULL", "p.status = ?"}
			args = []any{q.Status}
			if q.Type != "" {
				where = append(where, "p.type = ?")
				args = append(args, string(q.Type))
			}
		} else {
			where = append(where, "p.status = ?")
			args = append(args, q.Status)
		}
	}
	if q.Category != "" {
		where = append(where, "c.slug = ?")
		args = append(args, q.Category)
	}
	if q.Tag != "" {
		// 按标签筛选：与作者/分类条件之间是「且」关系，
		// 与关键词搜索也是「且」。同一标签的多值用逗号分隔表示「或」。
		tagClauses := []string{}
		for _, slug := range strings.Split(q.Tag, ",") {
			slug = strings.TrimSpace(slug)
			if slug == "" {
				continue
			}
			tagClauses = append(tagClauses,
				"p.id IN (SELECT pt.post_id FROM post_tags pt "+
					"JOIN tags t ON t.id = pt.tag_id WHERE t.slug = ?)")
			args = append(args, slug)
		}
		if len(tagClauses) > 0 {
			where = append(where, "("+strings.Join(tagClauses, " OR ")+")")
		}
	}
	if q.Author > 0 {
		where = append(where, "p.author_id = ?")
		args = append(args, q.Author)
	}
	if q.Year > 0 {
		if q.Month > 0 {
			where = append(where, "p.created_at LIKE ?")
			args = append(args, fmt.Sprintf("%04d-%02d-%%", q.Year, q.Month))
		} else {
			where = append(where, "p.created_at LIKE ?")
			args = append(args, fmt.Sprintf("%04d-%%", q.Year))
		}
	}
	if q.Sticky {
		where = append(where, "p.sticky = ?")
		args = append(args, r.dialectBool(1))
	}
	if q.Search != "" {
		// 搜索标题、摘要、正文；LIKE 通配符需转义为字面量
		like := "%" + escapeLike(q.Search) + "%"
		where = append(where, "(p.title LIKE ? ESCAPE '!' OR p.summary LIKE ? ESCAPE '!' OR p.content LIKE ? ESCAPE '!')")
		args = append(args, like, like, like)
	}
	if len(q.Exclude) > 0 {
		ph := strings.TrimSuffix(strings.Repeat("?, ", len(q.Exclude)), ", ")
		where = append(where, "p.id NOT IN ("+ph+")")
		for _, id := range q.Exclude {
			args = append(args, id)
		}
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	total, err := db.Count(r.DB, r.DBType,
		"SELECT COUNT(*) FROM posts p LEFT JOIN categories c ON c.id = p.category_id "+whereSQL,
		args...)
	if err != nil {
		return models.Paged[models.Post]{}, err
	}

	order := r.orderClause(q)
	query := "SELECT " + postColumns +
		" FROM posts p LEFT JOIN categories c ON c.id = p.category_id " +
		whereSQL + " ORDER BY " + order

	page, pageSize := q.Page, q.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	query += r.limitOffset(page, pageSize)

	rows, err := db.Query(r.DB, r.DBType, query, args...)
	if err != nil {
		return models.Paged[models.Post]{}, err
	}
	defer rows.Close()

	items := []models.Post{}
	ids := []int64{}
	for rows.Next() {
		p, err := r.scanPost(rows)
		if err != nil {
			return models.Paged[models.Post]{}, err
		}
		items = append(items, *p)
		ids = append(ids, p.ID)
	}

	r.attachRelations(items, ids)
	return models.NewPaged(items, total, page, pageSize), nil
}

func (r *Repo) orderClause(q PostQuery) string {
	switch q.Order {
	case "asc":
		return "COALESCE(p.published_at, p.created_at) ASC, p.id ASC"
	case "title":
		return "p.title ASC"
	case "meta":
		// 页面排序
		return "p.menu_order ASC, p.title ASC"
	case "sticky":
		// 置顶优先，其余按时间倒序。sticky 为布尔列，跨库用 1/0 排序可行
		return "p.sticky DESC, COALESCE(p.published_at, p.created_at) DESC, p.id DESC"
	default:
		return "COALESCE(p.published_at, p.created_at) DESC, p.id DESC"
	}
}

func (r *Repo) limitOffset(page, pageSize int) string {
	offset := (page - 1) * pageSize
	switch {
	case offset <= 0:
		return fmt.Sprintf(" LIMIT %d", pageSize)
	default:
		return fmt.Sprintf(" LIMIT %d OFFSET %d", pageSize, offset)
	}
}

func (r *Repo) dialectBool(v int) any {
	if r.DBType == "pgsql" {
		return v == 1
	}
	return v
}

func escapeLike(v string) string { return dialectEscape(v) }

// ---------- 关联数据 ----------

// attachRelations 批量补充分类、作者与标签，避免 N+1 查询。
func (r *Repo) attachRelations(items []models.Post, ids []int64) {
	if len(ids) == 0 {
		return
	}
	ph := strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ")

	// 分类
	cats, err := r.categoryMap(ids)
	if err == nil {
		for i := range items {
			if items[i].CategoryID == nil {
				continue
			}
			if c, ok := cats[*items[i].CategoryID]; ok {
				name, slug := c.Name, c.Slug
				items[i].CategoryName = &name
				items[i].CategorySlug = &slug
			}
		}
	}

	// 作者
	authors, err := r.authorMap(ids)
	if err == nil {
		for i := range items {
			if items[i].AuthorID == nil {
				continue
			}
			if a, ok := authors[*items[i].AuthorID]; ok {
				av := a
				items[i].Author = &av
			}
		}
	}

	// 标签
	tags, err := r.tagMap(ids)
	if err == nil {
		for i := range items {
			if t, ok := tags[items[i].ID]; ok {
				items[i].Tags = t
			}
		}
	}
	_ = ph
}

func (r *Repo) categoryMap(ids []int64) (map[int64]models.Category, error) {
	ph := strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := db.Query(r.DB, r.DBType,
		"SELECT c.id, c.name, c.slug, COALESCE(c.description, '') FROM categories c "+
			"JOIN posts p ON p.category_id = c.id WHERE p.id IN ("+ph+")", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]models.Category{}
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.Description); err != nil {
			return nil, err
		}
		out[c.ID] = c
	}
	return out, nil
}

func (r *Repo) authorMap(ids []int64) (map[int64]models.Author, error) {
	ph := strings.TrimSuffix(strings.Repeat("?, ", len(ids)), ", ")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := db.Query(r.DB, r.DBType,
		"SELECT DISTINCT u.id, u.username, u.display_name, u.avatar_url "+
			"FROM users u JOIN posts p ON p.author_id = u.id WHERE p.id IN ("+ph+")", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]models.Author{}
	for rows.Next() {
		var a models.Author
		if err := rows.Scan(&a.ID, &a.Username, &a.DisplayName, &a.AvatarURL); err != nil {
			return nil, err
		}
		if a.DisplayName == "" {
			a.DisplayName = a.Username
		}
		out[a.ID] = a
	}
	return out, nil
}

func (r *Repo) tagMap(postIDs []int64) (map[int64][]models.Tag, error) {
	ph := strings.TrimSuffix(strings.Repeat("?, ", len(postIDs)), ", ")
	args := make([]any, 0, len(postIDs))
	for _, id := range postIDs {
		args = append(args, id)
	}
	rows, err := db.Query(r.DB, r.DBType,
		"SELECT pt.post_id, t.id, t.name, t.slug FROM post_tags pt "+
			"JOIN tags t ON t.id = pt.tag_id WHERE pt.post_id IN ("+ph+") ORDER BY t.name", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]models.Tag{}
	for rows.Next() {
		var pid int64
		var t models.Tag
		if err := rows.Scan(&pid, &t.ID, &t.Name, &t.Slug); err != nil {
			return nil, err
		}
		out[pid] = append(out[pid], t)
	}
	return out, nil
}

// ---------- 单条查询 ----------

// GetPostBySlug 按 slug 取内容（含正文）。
func (r *Repo) GetPostBySlug(slug string, v Viewer) (*models.Post, error) {
	vis, visArgs := r.visibleClause(v, "p")
	args := append([]any{slug}, visArgs...)
	row := db.QueryRow(r.DB, r.DBType,
		"SELECT "+postColumns+" FROM posts p WHERE p.slug = ? AND "+vis, args...)
	p, err := r.scanPost(row)
	if err != nil {
		return nil, err
	}
	items := []models.Post{*p}
	r.attachRelations(items, []int64{p.ID})
	p = &items[0]
	return p, nil
}

// GetPostByID 按 ID 取内容。
func (r *Repo) GetPostByID(id int64, v Viewer) (*models.Post, error) {
	vis, visArgs := r.visibleClause(v, "p")
	args := append([]any{id}, visArgs...)
	row := db.QueryRow(r.DB, r.DBType,
		"SELECT "+postColumns+" FROM posts p WHERE p.id = ? AND "+vis, args...)
	p, err := r.scanPost(row)
	if err != nil {
		return nil, err
	}
	items := []models.Post{*p}
	r.attachRelations(items, []int64{p.ID})
	return &items[0], nil
}

// GetPostRawByID 不做可见性过滤（内部使用，如修订对比）。
func (r *Repo) GetPostRawByID(id int64) (*models.Post, error) {
	row := db.QueryRow(r.DB, r.DBType,
		"SELECT "+postColumns+" FROM posts p WHERE p.id = ?", id)
	return r.scanPost(row)
}

// IncrementViews 原子自增阅读量并返回新值。
// MySQL 不支持 UPDATE ... RETURNING，故用「自增后再查」的两步写法，
// 该值仅用于展示，允许极小概率的并发偏差。
func (r *Repo) IncrementViews(id int64) int {
	_, _ = db.Exec(r.DB, r.DBType,
		"UPDATE posts SET views = views + 1 WHERE id = ?", id)
	var views int
	_ = db.QueryRow(r.DB, r.DBType,
		"SELECT views FROM posts WHERE id = ?", id).Scan(&views)
	return views
}

// ---------- 邻居导航 ----------

// Neighbors 返回上下篇文章。
// prev 为更新的（时间更晚），next 为更旧的，与现有前端契约一致。
func (r *Repo) Neighbors(currentID int64) (prev, next *models.Neighbor, err error) {
	var publishedAt, createdAt sql.NullString
	e := db.QueryRow(r.DB, r.DBType,
		"SELECT published_at, created_at FROM posts WHERE id = ? AND status = 'published' AND deleted_at IS NULL",
		currentID).Scan(&publishedAt, &createdAt)
	if e != nil {
		return nil, nil, e
	}

	pa := publishedAt.String
	ca := createdAt.String
	if pa == "" {
		pa = ca
	}
	if ca == "" {
		ca = pa
	}

	prev, err = r.neighbor(currentID, pa, ca, false)
	if err != nil {
		return nil, nil, err
	}
	next, err = r.neighbor(currentID, pa, ca, true)
	return prev, next, err
}

func (r *Repo) neighbor(currentID int64, publishedAt, createdAt string, older bool) (*models.Neighbor, error) {
	op := ">"
	order := "COALESCE(published_at, created_at) ASC, id ASC"
	if older {
		op = "<"
		order = "COALESCE(published_at, created_at) DESC, id DESC"
	}
	var n models.Neighbor
	err := db.QueryRow(r.DB, r.DBType,
		"SELECT slug, title FROM posts WHERE status = 'published' AND deleted_at IS NULL "+
			"AND (COALESCE(published_at, created_at), id) "+op+" (COALESCE(?, ?), ?) "+
			"ORDER BY "+order+" LIMIT 1",
		publishedAt, createdAt, currentID).Scan(&n.Slug, &n.Title)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// ---------- 归档统计 ----------

// ArchiveCount 归档计数。
type ArchiveCount struct {
	Slug  string `json:"slug"`
	Count int    `json:"count"`
}

// MonthlyCount 按月计数。
type MonthlyCount struct {
	Month string `json:"month"`
	Count int    `json:"count"`
}

// CountByYear 统计各年份文章数。
func (r *Repo) CountByYear(v Viewer) ([]ArchiveCount, error) {
	vis, visArgs := r.visibleClause(v, "p")
	query := "SELECT substr(p.created_at, 1, 4) AS y, COUNT(*) FROM posts p WHERE " + vis +
		" AND p.type = 'post' GROUP BY y ORDER BY y DESC"
	return r.archiveQuery(query, visArgs)
}

// CountByMonth 统计各月份文章数（最近 months 个月）。
func (r *Repo) CountByMonth(v Viewer, months int) ([]MonthlyCount, error) {
	vis, visArgs := r.visibleClause(v, "p")
	query := "SELECT substr(p.created_at, 1, 7) AS ym, COUNT(*) FROM posts p WHERE " + vis +
		" AND p.type = 'post' GROUP BY ym ORDER BY ym DESC"
	rows, err := db.Query(r.DB, r.DBType, query, visArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MonthlyCount{}
	for rows.Next() {
		var m MonthlyCount
		if err := rows.Scan(&m.Month, &m.Count); err != nil {
			return nil, err
		}
		out = append(out, m)
		if months > 0 && len(out) >= months {
			break
		}
	}
	return out, nil
}

func (r *Repo) archiveQuery(query string, args []any) ([]ArchiveCount, error) {
	rows, err := db.Query(r.DB, r.DBType, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ArchiveCount{}
	for rows.Next() {
		var a ArchiveCount
		if err := rows.Scan(&a.Slug, &a.Count); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// ---------- 时间辅助 ----------

// NowISO 当前时间（UTC ISO-8601）。
func NowISO() string { return db.NowISO() }

// FutureISO 返回 n 分钟后的时间戳。
func FutureISO(minutes int) string {
	return time.Now().UTC().Add(time.Duration(minutes) * time.Minute).
		Format("2006-01-02T15:04:05.000Z")
}
