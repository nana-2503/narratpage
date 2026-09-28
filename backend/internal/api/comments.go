package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"narratpage/internal/auth"
	"narratpage/internal/db"
	"narratpage/internal/httpx"
)

const (
	maxAuthor  = 30
	maxContent = 1000
)

type commentItem struct {
	ID        int64   `json:"id"`
	PostID    int64   `json:"post_id"`
	Author    string  `json:"author"`
	Content   string  `json:"content"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
	PostTitle *string `json:"post_title,omitempty"`
}

type commentInput struct {
	PostID  *int64  `json:"postId"`
	Author  *string `json:"author"`
	Content *string `json:"content"`
}

// GET /api/comments?postId&status
// 仅管理员显式传 status 时按状态筛选，其余一律只看已通过
func listComments(dbType string) func(*sql.DB) http.HandlerFunc {
	rewrite := func(sql string) string { return db.RewritePlaceholders(dbType, sql) }
	return func(db *sql.DB) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			isAdmin := auth.UserFromContext(r.Context()) != nil

			where := []string{}
			var params []any
			if postID := q.Get("postId"); postID != "" {
				where = append(where, "cm.post_id = ?")
				params = append(params, postID)
			}
			if isAdmin && q.Get("status") != "" {
				where = append(where, "cm.status = ?")
				params = append(params, q.Get("status"))
			} else {
				where = append(where, "cm.status = 'approved'")
			}
			whereSQL := ""
			for i, c := range where {
				if i > 0 {
					whereSQL += " AND "
				}
				whereSQL += c
			}

			rows, err := db.Query(
				rewrite(
					`SELECT cm.id, cm.post_id, cm.author, cm.content, cm.status, cm.created_at,
					 p.title AS post_title
					 FROM comments cm JOIN posts p ON p.id = cm.post_id
					 WHERE `+whereSQL+` ORDER BY cm.created_at DESC LIMIT 200`),
				params...,
			)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
				return
			}
			defer rows.Close()
			items := []commentItem{}
			for rows.Next() {
				var c commentItem
				if err := rows.Scan(&c.ID, &c.PostID, &c.Author, &c.Content, &c.Status, &c.CreatedAt, &c.PostTitle); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
					return
				}
				items = append(items, c)
			}
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
		}
	}
}

// POST /api/comments（公开提交，待审核）
func createComment(dbType string) func(*sql.DB) http.HandlerFunc {
	rewrite := func(sql string) string { return db.RewritePlaceholders(dbType, sql) }
	now := func() string { return db.NowISO() }
	return func(db *sql.DB) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var body commentInput
			if err := httpx.DecodeJSON(w, r, &body); err != nil {
				return
			}
			if body.PostID == nil || !postExists(rewrite, db, *body.PostID) {
				httpx.WriteError(w, http.StatusBadRequest, "文章不存在")
				return
			}
			author := strings.TrimSpace(deref(body.Author))
			content := strings.TrimSpace(deref(body.Content))
			if author == "" || len(author) > maxAuthor {
				httpx.WriteError(w, http.StatusBadRequest, "昵称必填且最长 "+strconv.Itoa(maxAuthor)+" 字符")
				return
			}
			if content == "" || len(content) > maxContent {
				httpx.WriteError(w, http.StatusBadRequest, "评论内容必填且最长 "+strconv.Itoa(maxContent)+" 字符")
				return
			}
			var id int64
			err := db.QueryRow(
				rewrite(`INSERT INTO comments (post_id, author, content, status, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`),
				*body.PostID, author, content, "pending", now(),
			).Scan(&id)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
				return
			}
			httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": id, "status": "pending"})
		}
	}
}

func postExists(rewrite func(string) string, db *sql.DB, id int64) bool {
	var n int
	return db.QueryRow(
		rewrite(`SELECT id FROM posts WHERE id = ?`), id,
	).Scan(&n) == nil
}

type moderateInput struct {
	Status *string `json:"status"`
}

// PUT /api/comments/:id
func moderateComment(dbType string) func(*sql.DB) http.HandlerFunc {
	rewrite := func(sql string) string { return db.RewritePlaceholders(dbType, sql) }
	return func(db *sql.DB) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
			if err != nil {
				httpx.WriteError(w, http.StatusNotFound, "评论不存在")
				return
			}
			var n int
			if err := db.QueryRow(
				rewrite(`SELECT id FROM comments WHERE id = ?`), id,
			).Scan(&n); err == sql.ErrNoRows {
				httpx.WriteError(w, http.StatusNotFound, "评论不存在")
				return
			} else if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
				return
			}
			var body moderateInput
			if err := httpx.DecodeJSON(w, r, &body); err != nil {
				return
			}
			status := deref(body.Status)
			if status != "pending" && status != "approved" && status != "rejected" {
				httpx.WriteError(w, http.StatusBadRequest, "非法状态")
				return
			}
			if _, err := db.Exec(
				rewrite(`UPDATE comments SET status = ? WHERE id = ?`),
				status, id,
			); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
				return
			}
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "status": status})
		}
	}
}

// DELETE /api/comments/:id
func deleteComment(dbType string) func(*sql.DB) http.HandlerFunc {
	rewrite := func(sql string) string { return db.RewritePlaceholders(dbType, sql) }
	return func(db *sql.DB) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
			if err != nil {
				httpx.WriteError(w, http.StatusNotFound, "评论不存在")
				return
			}
			res, err := db.Exec(
				rewrite(`DELETE FROM comments WHERE id = ?`), id,
			)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
				return
			}
			if n, _ := res.RowsAffected(); n == 0 {
				httpx.WriteError(w, http.StatusNotFound, "评论不存在")
				return
			}
			httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
		}
	}
}


