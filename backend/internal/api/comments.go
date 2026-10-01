package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"narratpage/internal/auth"
	"narratpage/internal/db"
	"narratpage/internal/httpx"
	"narratpage/internal/models"
	"narratpage/internal/ratelimit"
	"narratpage/internal/repo"
)

const (
	maxCommentAuthor  = 64
	maxCommentEmail   = 255
	maxCommentContent = 3000
	maxCommentWebsite = 255
	// 同一文章最短评论间隔，抑制灌水
	commentCooldown = 30 * time.Second
)

// commentLimiter 评论提交限流（与登录限流分离，避免互相影响配额）。
var commentLimiter = newCommentLimiter()

// resetCommentLimiter 供测试隔离：限流器为包级单例，
// 多个测试共用同一实例会互相干扰。
func resetCommentLimiter() { commentLimiter = newCommentLimiter() }

type commentInput struct {
	PostID   int64   `json:"postId"`
	ParentID *int64  `json:"parentId"`
	Author   *string `json:"author"`
	Email    *string `json:"email"`
	Website  *string `json:"website"`
	Content  *string `json:"content"`
	// 蜜罐字段：正常用户不会填写，机器人往往无视隐藏字段
	Honeypot *string `json:"website_confirm"`
}

// listComments GET /api/comments
func listComments(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		postID, _ := strconv.ParseInt(q.Get("postId"), 10, 64)
		page, _ := strconv.Atoi(q.Get("page"))
		viewer := viewerFrom(r)
		// 嵌套模式：一次取回该文章全部已通过评论，前端自行组装
		nested := q.Get("nested") == "1"

		cq := repo.CommentQuery{
			PostID: postID,
			Status: q.Get("status"),
			Search: strings.TrimSpace(q.Get("q")),
			Page:   page,
		}
		if nested {
			cq.Limit = 200
		}

		result, err := deps.Comments.List(cq, viewer)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		items := result.Items
		if viewer.IsAdmin && postID == 0 {
			items, _ = deps.Comments.WithPostTitles(items)
		}
		if nested {
			items = repo.BuildTree(items)
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"items":      items,
			"total":      result.Total,
			"page":       result.Page,
			"pageSize":   result.PageSize,
			"totalPages": result.TotalPages,
		})
	}
}

// createComment POST /api/comments
func createComment(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		site := deps.Options.Site()
		if !site.CommentsEnabled {
			badRequest(w, "评论已关闭")
			return
		}

		// 蜜罐命中即静默丢弃：不给机器人反馈，但记录为已接受
		var body commentInput
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		if body.Honeypot != nil && strings.TrimSpace(*body.Honeypot) != "" {
			httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": 0, "status": "pending"})
			return
		}

		postID := body.PostID
		if postID <= 0 {
			// 也支持按 slug 提交
			if slug := r.URL.Query().Get("slug"); slug != "" {
				post, err := deps.Posts.GetPostBySlug(slug, repo.PublicViewer())
				if err != nil {
					badRequest(w, "文章不存在")
					return
				}
				postID = post.ID
			}
		}
		if postID <= 0 {
			badRequest(w, "缺少文章标识")
			return
		}

		// 目标文章必须存在且可评论
		post, err := deps.Posts.GetPostByID(postID, repo.PublicViewer())
		if err != nil {
			badRequest(w, "文章不存在")
			return
		}
		if post.Type != models.TypePost {
			badRequest(w, "该内容不支持评论")
			return
		}

		// 内容校验放在限流之前：参数错误是客户端问题，
		// 不应消耗提交配额，否则一次调试就会被限流挡住。
		author := strings.TrimSpace(derefStr(body.Author))
		content := strings.TrimSpace(derefStr(body.Content))
		email := strings.TrimSpace(derefStr(body.Email))
		website := strings.TrimSpace(derefStr(body.Website))

		if author == "" || len(author) > maxCommentAuthor {
			badRequest(w, "昵称必填且最长 64 字符")
			return
		}
		if content == "" || len(content) > maxCommentContent {
			badRequest(w, "评论内容必填且最长 3000 字符")
			return
		}
		if len(email) > maxCommentEmail {
			badRequest(w, "邮箱过长")
			return
		}
		if email != "" && !strings.Contains(email, "@") {
			badRequest(w, "邮箱格式不正确")
			return
		}
		// 外链只允许 http/https，避免 javascript: 伪协议
		if website != "" {
			if len(website) > maxCommentWebsite {
				badRequest(w, "网址过长")
				return
			}
			if !strings.HasPrefix(website, "http://") && !strings.HasPrefix(website, "https://") {
				badRequest(w, "网址须以 http:// 或 https:// 开头")
				return
			}
		}

		if !commentLimiter.Allow(r) {
			httpx.WriteError(w, http.StatusTooManyRequests, "评论提交过于频繁，请稍后再试")
			return
		}

		// 冷却：同 IP 对同一文章的最短间隔，仅作用于顶层评论。
		//
		// 回复不参与冷却判定——否则「刚发完主楼就回复自己的楼层」
		// 这种最正常的行为会被拦下。回复的滥用由上面的
		// 频率限流器统一约束。
		isReply := body.ParentID != nil && *body.ParentID > 0
		ip := ratelimit.ClientIP(r, deps.TrustProxy)
		if !isReply {
			since := time.Now().UTC().Add(-commentCooldown).
				Format("2006-01-02T15:04:05.000Z")
			if tooRecent, _ := dbCountWhere(deps,
				"SELECT COUNT(*) FROM comments WHERE post_id = ? AND created_at > ? AND ip_hash = ?",
				postID, since, hashIP(ip)); tooRecent > 0 {
				httpx.WriteError(w, http.StatusTooManyRequests, "评论提交过于频繁，请稍后再试")
				return
			}
		}

		// 父评论必须存在且属于同一篇文章
		if body.ParentID != nil && *body.ParentID > 0 {
			parent, err := deps.Comments.ByID(*body.ParentID)
			if err != nil || parent.PostID != postID {
				badRequest(w, "回复的评论不存在")
				return
			}
			// 仅支持二级：回复的父评论的父级归一到顶层
			if parent.ParentID != nil {
				body.ParentID = parent.ParentID
			}
		}

		claims := auth.UserFromContext(r.Context())
		var authorID *int64
		if claims != nil {
			authorID = &claims.Sub
		}

		status := "pending"
		if claims != nil && !site.CommentModeration {
			// 已登录用户且站点未开启审核时直接通过
			status = "approved"
		}

		c := &models.Comment{
			PostID:   postID,
			ParentID: body.ParentID,
			AuthorID: authorID,
			Author:   author,
			Email:    email,
			Website:  website,
			Content:  content,
			Status:   status,
		}
		id, err := deps.Comments.Create(c, hashIP(ip))
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": id, "status": status})
	}
}

// moderateComment PUT /api/comments/{id}
func moderateComment(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "评论不存在")
			return
		}
		var body struct {
			Status *string `json:"status"`
		}
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		status := derefStr(body.Status)
		switch status {
		case "pending", "approved", "rejected", "spam":
		default:
			badRequest(w, "非法状态")
			return
		}
		n, err := deps.Comments.UpdateStatus(id, status)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		if n == 0 {
			notFound(w, "评论不存在")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "status": status})
	}
}

// deleteComment DELETE /api/comments/{id}
func deleteComment(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "评论不存在")
			return
		}
		n, err := deps.Comments.Delete(id)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		if n == 0 {
			notFound(w, "评论不存在")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// batchComments POST /api/comments/batch
func batchComments(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			IDs    []int64 `json:"ids"`
			Action string  `json:"action"`
		}
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		if len(body.IDs) == 0 {
			badRequest(w, "未选择评论")
			return
		}
		if len(body.IDs) > 500 {
			badRequest(w, "单次最多处理 500 条")
			return
		}

		var n int64
		var err error
		switch body.Action {
		case "approve":
			n, err = deps.Comments.BulkUpdateStatus(body.IDs, "approved")
		case "reject":
			n, err = deps.Comments.BulkUpdateStatus(body.IDs, "rejected")
		case "spam":
			n, err = deps.Comments.BulkUpdateStatus(body.IDs, "spam")
		case "delete":
			n, err = deps.Comments.BulkDelete(body.IDs)
		default:
			badRequest(w, "未知操作")
			return
		}
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "affected": n})
	}
}

// commentCounts GET /api/comments/counts
func commentCounts(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(deps.DB, deps.DBType,
			"SELECT status, COUNT(*) FROM comments GROUP BY status")
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		defer rows.Close()
		counts := map[string]int{"pending": 0, "approved": 0, "rejected": 0, "spam": 0}
		for rows.Next() {
			var status string
			var n int
			if err := rows.Scan(&status, &n); err != nil {
				continue
			}
			counts[status] = n
		}
		httpx.WriteJSON(w, http.StatusOK, counts)
	}
}

// dbCountWhere 便捷计数。
func dbCountWhere(deps Deps, query string, args ...any) (int, error) {
	return db.Count(deps.DB, deps.DBType, query, args...)
}

// hashIP 对 IP 做加盐哈希后再存储：满足反垃圾需求，同时不保留原始 IP。
func hashIP(ip string) string {
	if ip == "" {
		return ""
	}
	return shortHash(ip + "narratpage-salt")
}
