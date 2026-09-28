package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"narratpage/internal/httpx"
)

type categoryItem struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	PostCount int    `json:"post_count"`
}

// GET /api/categories（公开，含已发布文章数）
func listCategories(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(
			`SELECT c.id, c.name, c.slug,
			        (SELECT COUNT(*) FROM posts p WHERE p.category_id = c.id AND p.status = 'published') AS post_count
			 FROM categories c ORDER BY c.id`,
		)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		defer rows.Close()
		items := []categoryItem{}
		for rows.Next() {
			var c categoryItem
			if err := rows.Scan(&c.ID, &c.Name, &c.Slug, &c.PostCount); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
				return
			}
			items = append(items, c)
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

type categoryInput struct {
	Name *string `json:"name"`
	Slug *string `json:"slug"`
}

// POST /api/categories
func createCategory(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body categoryInput
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		name := strings.TrimSpace(deref(body.Name))
		if name == "" {
			httpx.WriteError(w, http.StatusBadRequest, "分类名称必填")
			return
		}
		if len(name) > 50 {
			httpx.WriteError(w, http.StatusBadRequest, "分类名最长 50 字符")
			return
		}
		slug := strings.TrimSpace(deref(body.Slug))
		if slug == "" {
			slug = slugify(name, "cat")
		}
		if slugTaken(db, slug, 0) {
			httpx.WriteError(w, http.StatusConflict, "slug 已存在")
			return
		}
		res, err := db.Exec(`INSERT INTO categories (name, slug) VALUES (?, ?)`, name, slug)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		id, _ := res.LastInsertId()
		httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": id, "name": name, "slug": slug})
	}
}

// PUT /api/categories/:id（未传 slug 时保留原 slug，外链不失效）
func updateCategory(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			httpx.WriteError(w, http.StatusNotFound, "分类不存在")
			return
		}
		var curName, curSlug string
		err = db.QueryRow(`SELECT name, slug FROM categories WHERE id = ?`, id).Scan(&curName, &curSlug)
		if err == sql.ErrNoRows {
			httpx.WriteError(w, http.StatusNotFound, "分类不存在")
			return
		}
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		var body categoryInput
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		name := strings.TrimSpace(deref(body.Name))
		if name == "" {
			name = curName
		}
		slug := strings.TrimSpace(deref(body.Slug))
		if slug == "" {
			slug = curSlug
		}
		if slugTaken(db, slug, id) {
			httpx.WriteError(w, http.StatusConflict, "slug 已存在")
			return
		}
		if _, err := db.Exec(`UPDATE categories SET name = ?, slug = ? WHERE id = ?`, name, slug, id); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
	}
}

// DELETE /api/categories/:id（文章经外键 ON DELETE SET NULL 变未分类）
func deleteCategory(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			httpx.WriteError(w, http.StatusNotFound, "分类不存在")
			return
		}
		res, err := db.Exec(`DELETE FROM categories WHERE id = ?`, id)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			httpx.WriteError(w, http.StatusNotFound, "分类不存在")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func slugTaken(db *sql.DB, slug string, exceptID int64) bool {
	var n int64
	return db.QueryRow(
		`SELECT id FROM categories WHERE slug = ? AND id != ?`, slug, exceptID,
	).Scan(&n) == nil
}
