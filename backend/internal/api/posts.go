package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"narratpage/internal/auth"
	"narratpage/internal/db"
	"narratpage/internal/httpx"
)

// ---------- 响应结构（JSON 字段与 Node 版逐字对齐） ----------

type postCategory struct {
	CategoryID   *int64  `json:"category_id"`
	CategoryName *string `json:"category_name"`
	CategorySlug *string `json:"category_slug"`
}

type postListItem struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Slug        string  `json:"slug"`
	Summary     string  `json:"summary"`
	CoverURL    string  `json:"cover_url"`
	Status      string  `json:"status"`
	Views       int     `json:"views"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	PublishedAt *string `json:"published_at"`
	postCategory
}

type postDetail struct {
	postListItem
	Content string `json:"content"`
}

type postListResponse struct {
	Items      []postListItem `json:"items"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
	TotalPages int            `json:"totalPages"`
}

type neighborPost struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

type neighborsResponse struct {
	Prev *neighborPost `json:"prev"`
	Next *neighborPost `json:"next"`
}

// ---------- handlers ----------

// GET /api/posts?page&pageSize&category&q&status=all(仅管理员)
func listPosts(dbType string) func(*sql.DB) http.HandlerFunc {
	rewrite := func(sql string) string { return db.RewritePlaceholders(dbType, sql) }
	return func(db *sql.DB) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			page := httpx.QueryInt(r, "page", 1)
			if page < 1 {
				page = 1
			}
			pageSize := httpx.QueryInt(r, "pageSize", 10)
			if pageSize > 50 {
				pageSize = 50
			}
			if pageSize < 1 {
				pageSize = 1
			}
			q := r.URL.Query()
			category := q.Get("category")
			search := q.Get("q")

			isAdmin := auth.UserFromContext(r.Context()) != nil
			wantAll := q.Get("status") == "all" && isAdmin

			where := []string{"1 = 1"}
			if !wantAll {
				where = []string{"p.status = 'published'"}
			}
			var params []any
			if category != "" {
				where = append(where, "c.slug = ?")
				params = append(params, category)
			}
			if search != "" {
				where = append(where, `(p.title LIKE ? ESCAPE '\' OR p.summary LIKE ? ESCAPE '\')`)
				like := "%" + escapeLike(search) + "%"
				params = append(params, like, like)
			}
			whereSQL := "WHERE " + strings.Join(where, " AND ")

			var total int
			if err := db.QueryRow(
				rewrite(`SELECT COUNT(*) AS n FROM posts p LEFT JOIN categories c ON c.id = p.category_id `+whereSQL),
				params...,
			).Scan(&total); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
				return
			}

			rows, err := db.Query(
				rewrite(
					`SELECT p.id, p.title, p.slug, p.summary, p.cover_url, p.status, p.views,
					 p.created_at, p.updated_at, p.published_at,
					 c.id AS category_id, c.name AS category_name, c.slug AS category_slug
					 FROM posts p LEFT JOIN categories c ON c.id = p.category_id `+whereSQL+
						` ORDER BY COALESCE(p.published_at, p.created_at) DESC, p.id DESC LIMIT ? OFFSET ?`,
				),
				append(params, pageSize, (page-1)*pageSize)...,
			)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
				return
			}
			defer rows.Close()

			items := []postListItem{}
			for rows.Next() {
				var p postListItem
				if err := rows.Scan(
					&p.ID, &p.Title, &p.Slug, &p.Summary, &p.CoverURL, &p.Status, &p.Views,
					&p.CreatedAt, &p.UpdatedAt, &p.PublishedAt,
					&p.CategoryID, &p.CategoryName, &p.CategorySlug,
				); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
					return
				}
				items = append(items, p)
			}

			httpx.WriteJSON(w, http.StatusOK, postListResponse{
				Items:      items,
				Total:      total,
				Page:       page,
				PageSize:   pageSize,
				TotalPages: (total + pageSize - 1) / pageSize,
			})
		}
	}
}

// GET /api/posts/id/:id（后台编辑用，含正文）
func getPostByID(dbType string) func(*sql.DB) http.HandlerFunc {
	rewrite := func(sql string) string { return db.RewritePlaceholders(dbType, sql) }
	return func(db *sql.DB) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
			if err != nil {
				httpx.WriteError(w, http.StatusNotFound, "文章不存在")
				return
			}
			post, err := scanPostDetail(db.QueryRow(
				rewrite(
					`SELECT p.id, p.title, p.slug, p.summary, p.content, p.cover_url, p.status, p.views,
					 p.created_at, p.updated_at, p.published_at, p.category_id,
					 c.name AS category_name, c.slug AS category_slug
					 FROM posts p LEFT JOIN categories c ON c.id = p.category_id WHERE p.id = ?`),
				id,
			))
			if err == sql.ErrNoRows {
				httpx.WriteError(w, http.StatusNotFound, "文章不存在")
				return
			}
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
				return
			}
			httpx.WriteJSON(w, http.StatusOK, post)
		}
	}
}

// GET /api/posts/:slug（详情含正文，阅读数 +1）
func getPostBySlug(dbType string) func(*sql.DB) http.HandlerFunc {
	rewrite := func(sql string) string { return db.RewritePlaceholders(dbType, sql) }
	return func(db *sql.DB) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			slug := r.PathValue("slug")
			isAdmin := auth.UserFromContext(r.Context()) != nil
			sqlText := rewrite(
				`SELECT p.id, p.title, p.slug, p.summary, p.content, p.cover_url, p.status, p.views,
				 p.created_at, p.updated_at, p.published_at, p.category_id,
				 c.name AS category_name, c.slug AS category_slug
				 FROM posts p LEFT JOIN categories c ON c.id = p.category_id WHERE p.slug = ?`)
			if !isAdmin {
				sqlText += " AND p.status = 'published'"
			}
			post, err := scanPostDetail(db.QueryRow(sqlText, slug))
			if err == sql.ErrNoRows {
				httpx.WriteError(w, http.StatusNotFound, "文章不存在")
				return
			}
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
				return
			}
			// 先落库自增，再以库值为准响应（与 Node 版一致的自增语义）
			var views int
			if err := db.QueryRow(
				rewrite(`UPDATE posts SET views = views + 1 WHERE id = ? RETURNING views`),
				post.ID,
			).Scan(&views); err == nil {
				post.Views = views
			}
			httpx.WriteJSON(w, http.StatusOK, post)
		}
	}
}

// GET /api/posts/neighbors/:slug — 上下篇（同秒发布用 id 兜底形成全序）
func getNeighbors(dbType string) func(*sql.DB) http.HandlerFunc {
	rewrite := func(sql string) string { return db.RewritePlaceholders(dbType, sql) }
	return func(db *sql.DB) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			slug := r.PathValue("slug")
			var id int64
			var publishedAt, createdAt *string
			err := db.QueryRow(
				rewrite(`SELECT id, published_at, created_at FROM posts WHERE slug = ? AND status = 'published'`),
				slug,
			).Scan(&id, &publishedAt, &createdAt)
			if err == sql.ErrNoRows {
				httpx.WriteError(w, http.StatusNotFound, "文章不存在")
				return
			}
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
				return
			}
			// prev = 发布时间更晚（更新）；next = 更早（更旧）
			prev := queryNeighbor(rewrite, db, id, publishedAt, createdAt, false)
			next := queryNeighbor(rewrite, db, id, publishedAt, createdAt, true)
			httpx.WriteJSON(w, http.StatusOK, neighborsResponse{Prev: prev, Next: next})
		}
	}
}

func queryNeighbor(rewrite func(string) string, db *sql.DB, id int64, publishedAt, createdAt *string, older bool) *neighborPost {
	op := ">"
	order := "COALESCE(published_at, created_at) ASC, id ASC"
	if older {
		op = "<"
		order = "COALESCE(published_at, created_at) DESC, id DESC"
	}
	row := db.QueryRow(
		rewrite(
			`SELECT slug, title FROM posts
			 WHERE status = 'published'
			   AND (COALESCE(published_at, created_at), id) `+op+` (COALESCE(?, ?), ?)
			 ORDER BY `+order+` LIMIT 1`),
		publishedAt, createdAt, id,
	)
	var n neighborPost
	if err := row.Scan(&n.Slug, &n.Title); err != nil {
		return nil
	}
	return &n
}

// ---------- 创建 / 更新 ----------

type postInput struct {
	Title      *string `json:"title"`
	Summary    *string `json:"summary"`
	Content    *string `json:"content"`
	CoverURL   *string `json:"cover_url"`
	CategoryID *int64  `json:"category_id"`
	Status     *string `json:"status"`
}

// validateAndNormalize 校验并规范化；partial=true 时忽略未提供的字段。
func validateAndNormalize(body *postInput, partial bool) (map[string]any, []string) {
	patch := map[string]any{}
	var errors []string

	if !partial || body.Title != nil {
		title := strings.TrimSpace(deref(body.Title))
		if title == "" {
			errors = append(errors, "标题必填")
		} else if len(title) > 200 {
			errors = append(errors, "标题最长 200 字符")
		}
		patch["title"] = title
	}
	if !partial || body.Summary != nil {
		summary := strings.TrimSpace(deref(body.Summary))
		if len(summary) > 500 {
			errors = append(errors, "摘要最长 500 字符")
		}
		patch["summary"] = summary
	}
	if !partial || body.Content != nil {
		patch["content"] = deref(body.Content)
	}
	if !partial || body.CoverURL != nil {
		patch["cover_url"] = strings.TrimSpace(deref(body.CoverURL))
	}
	if !partial || body.CategoryID != nil {
		var catID any
		if body.CategoryID != nil && *body.CategoryID != 0 {
			catID = *body.CategoryID
		}
		patch["category_id"] = catID
	}
	if !partial || body.Status != nil {
		status := deref(body.Status)
		if status == "" {
			status = "draft"
		}
		if status != "draft" && status != "published" {
			errors = append(errors, "非法状态")
		}
		patch["status"] = status
	}
	return patch, errors
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func categoryExists(rewrite func(string) string, db *sql.DB, id int64) bool {
	var n int
	return db.QueryRow(
		rewrite(`SELECT id FROM categories WHERE id = ?`), id,
	).Scan(&n) == nil
}

// slugify 标题 → URL 友好 slug（纯符号输入时追加时间戳保证可写）
func slugify(text, prefix string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (r >= 0x4e00 && r <= 0x9fa5) {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return prefix + "-" + strconv.FormatInt(time.Now().UnixMilli(), 36)
	}
	return slug
}

func uniqueSlug(rewrite func(string) string, db *sql.DB, slug string) string {
	var n int
	if err := db.QueryRow(
		rewrite(`SELECT id FROM posts WHERE slug = ?`), slug,
	).Scan(&n); err != nil {
		return slug // 未占用
	}
	return slug + "-" + strconv.FormatInt(time.Now().UnixMilli(), 36)
}

// POST /api/posts
func createPost(dbType string) func(*sql.DB) http.HandlerFunc {
	rewrite := func(sql string) string { return db.RewritePlaceholders(dbType, sql) }
	now := func() string { return db.NowISO() }
	return func(db *sql.DB) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var body postInput
			if err := httpx.DecodeJSON(w, r, &body); err != nil {
				return
			}
			patch, errors := validateAndNormalize(&body, false)
			if len(errors) > 0 {
				httpx.WriteError(w, http.StatusBadRequest, errors[0])
				return
			}
			if patch["category_id"] != nil && !categoryExists(rewrite, db, patch["category_id"].(int64)) {
				httpx.WriteError(w, http.StatusBadRequest, "分类不存在")
				return
			}
			slug := uniqueSlug(rewrite, db, slugify(patch["title"].(string), "post"))

			var publishedAt any
			if patch["status"] == "published" {
				publishedAt = now()
			}
			var id int64
			err := db.QueryRow(
				rewrite(
					`INSERT INTO posts (title, slug, summary, content, cover_url, category_id, status, published_at, created_at, updated_at)
					 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`),
				patch["title"], slug, patch["summary"], patch["content"],
				patch["cover_url"], patch["category_id"], patch["status"], publishedAt,
				now(), now(),
			).Scan(&id)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
				return
			}
			httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": id, "slug": slug})
		}
	}
}

// PUT /api/posts/:id
func updatePost(dbType string) func(*sql.DB) http.HandlerFunc {
	rewrite := func(sql string) string { return db.RewritePlaceholders(dbType, sql) }
	now := func() string { return db.NowISO() }
	return func(db *sql.DB) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
			if err != nil {
				httpx.WriteError(w, http.StatusNotFound, "文章不存在")
				return
			}
			var curStatus string
			var curPublishedAt *string
			err = db.QueryRow(
				rewrite(`SELECT status, published_at FROM posts WHERE id = ?`), id,
			).Scan(&curStatus, &curPublishedAt)
			if err == sql.ErrNoRows {
				httpx.WriteError(w, http.StatusNotFound, "文章不存在")
				return
			}
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
				return
			}

			var body postInput
			if err := httpx.DecodeJSON(w, r, &body); err != nil {
				return
			}
			patch, errors := validateAndNormalize(&body, true)
			if len(errors) > 0 {
				httpx.WriteError(w, http.StatusBadRequest, errors[0])
				return
			}
			if patch["category_id"] != nil && !categoryExists(rewrite, db, patch["category_id"].(int64)) {
				httpx.WriteError(w, http.StatusBadRequest, "分类不存在")
				return
			}

			// 读取当前值，未提供的字段保持原值
			next := map[string]any{}
			for _, col := range []string{"title", "summary", "content", "cover_url", "category_id", "status"} {
				if v, ok := patch[col]; ok {
					next[col] = v
				} else {
					var cur any
					db.QueryRow(
						rewrite(`SELECT `+col+` FROM posts WHERE id = ?`), id,
					).Scan(&cur)
					next[col] = cur
				}
			}

			// 从草稿首次发布时记录发布时间
			publishedAt := curPublishedAt
			if next["status"] == "published" && curStatus != "published" {
				pa := now()
				publishedAt = &pa
			}

			if _, err := db.Exec(
				rewrite(
					`UPDATE posts SET title = ?, summary = ?, content = ?, cover_url = ?, category_id = ?,
					   status = ?, published_at = ?, updated_at = ?
					 WHERE id = ?`),
				next["title"], next["summary"], next["content"], next["cover_url"], next["category_id"],
				next["status"], publishedAt, now(), id,
			); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
				return
			}
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
		}
	}
}

// DELETE /api/posts/:id
func deletePost(dbType string) func(*sql.DB) http.HandlerFunc {
	rewrite := func(sql string) string { return db.RewritePlaceholders(dbType, sql) }
	return func(db *sql.DB) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
			if err != nil {
				httpx.WriteError(w, http.StatusNotFound, "文章不存在")
				return
			}
			res, err := db.Exec(
				rewrite(`DELETE FROM posts WHERE id = ?`), id,
			)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
				return
			}
			if n, _ := res.RowsAffected(); n == 0 {
				httpx.WriteError(w, http.StatusNotFound, "文章不存在")
				return
			}
			httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
		}
	}
}

// ---------- 辅助 ----------

func scanPostDetail(row *sql.Row) (*postDetail, error) {
	var p postDetail
	err := row.Scan(
		&p.ID, &p.Title, &p.Slug, &p.Summary, &p.Content, &p.CoverURL, &p.Status, &p.Views,
		&p.CreatedAt, &p.UpdatedAt, &p.PublishedAt,
		&p.CategoryID, &p.CategoryName, &p.CategorySlug,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func escapeLike(v string) string {
	var b strings.Builder
	for _, r := range v {
		if r == '\\' || r == '%' || r == '_' {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
