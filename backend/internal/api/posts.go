package api

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"narratpage/internal/auth"
	"narratpage/internal/db"
	"narratpage/internal/httpx"
	"narratpage/internal/models"
	"narratpage/internal/repo"
)

// alias 便于在 handler 中引用仓储的 slug 生成逻辑
var taxonomy = struct{ Slugify func(string, string) string }{
	Slugify: repo.Slugify,
}

// ---------- 查询 ----------

// listPosts GET /api/posts
// 公开可访问；登录后按角色放宽可见范围（作者见自己草稿，编辑见全部）。
func listPosts(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := buildPostQuery(r, models.TypePost)
		result, err := deps.Posts.ListPosts(q, viewerFrom(r))
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, result)
	}
}

// getPost GET /api/posts/{slug}
//
// 路径参数既接受 slug 也接受数字 ID：后台编辑页需要按 ID 取草稿，
// 而公开详情页按 slug 取。为避免 ServeMux 出现
// `/api/posts/id/{id}` 与 `/api/posts/{id}/revisions` 模式冲突，
// 这里统一走单段参数并在内部判别。
func getPost(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("slug")
		viewer := viewerFrom(r)

		var post *models.Post
		var err error
		if id, convErr := strconv.ParseInt(key, 10, 64); convErr == nil {
			// 数字 ID 查询同样受可见性约束，草稿不会因此泄露
			post, err = deps.Posts.GetPostByID(id, viewer)
		} else {
			post, err = deps.Posts.GetPostBySlug(key, viewer)
		}
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		// 私密文章仅作者与编辑可见
		if post.Status == models.StatusPrivate && !viewer.IsAdmin &&
			(post.AuthorID == nil || viewer.UserID != *post.AuthorID) {
			notFound(w, "文章不存在")
			return
		}
		// 访问密码保护：正文不返回，前端凭 cookie 后续取
		if post.HasPassword {
			if !r.URL.Query().Has("unlocked") {
				post.Content = ""
			} else {
				post.Views = deps.Posts.IncrementViews(post.ID)
			}
		} else {
			post.Views = deps.Posts.IncrementViews(post.ID)
		}
		post.Password = ""
		httpx.WriteJSON(w, http.StatusOK, post)
	}
}

// getNeighbors GET /api/posts/{slug}/neighbors
func getNeighbors(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		post, err := deps.Posts.GetPostBySlug(slug, viewerFrom(r))
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		prev, next, err := deps.Posts.Neighbors(post.ID)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"prev": prev, "next": next})
	}
}

// ---------- 写入 ----------

type postInput struct {
	Title      *string            `json:"title"`
	Slug       *string            `json:"slug"`
	Summary    *string            `json:"summary"`
	Content    *string            `json:"content"`
	CoverURL   *string            `json:"cover_url"`
	CategoryID *int64             `json:"category_id"`
	Category   *string            `json:"category"`
	Status     *string            `json:"status"`
	Type       *string            `json:"type"`
	Sticky     *bool              `json:"sticky"`
	Password   *string            `json:"password"`
	MenuOrder  *int               `json:"menu_order"`
	Template   *string            `json:"template"`
	Tags       *[]string          `json:"tags"`
	TagIDs     *[]int64           `json:"tag_ids"`
	Meta       *map[string]string `json:"meta"`
}

const (
	maxTitle   = 200
	maxSummary = 500
)

// validate 校验并返回首个错误。
//
// partial 表示「只校验显式提供的字段」，未提供的字段沿用原值。
// 更新接口统一按 partial 处理：即便请求体带 title，也要求非空，
// 避免出现「把标题改成空串」这类破坏性写入。
func (in *postInput) validate(partial bool) string {
	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if title == "" {
			return "标题必填"
		}
		if len(title) > maxTitle {
			return "标题最长 200 字符"
		}
	} else if !partial {
		return "标题必填"
	}
	if in.Summary != nil && len(strings.TrimSpace(*in.Summary)) > maxSummary {
		return "摘要最长 500 字符"
	}
	if in.Status != nil {
		switch models.ContentStatus(*in.Status) {
		case models.StatusDraft, models.StatusPublished, models.StatusPending,
			models.StatusPrivate, models.StatusTrash:
		default:
			return "非法状态"
		}
	}
	if in.Type != nil {
		switch models.ContentType(*in.Type) {
		case models.TypePost, models.TypePage:
		default:
			return "非法内容类型"
		}
	}
	if in.Password != nil && len(*in.Password) > 128 {
		return "访问密码最长 128 字符"
	}
	if in.Meta != nil {
		for k := range *in.Meta {
			if len(k) > 128 {
				return "元数据键名最长 128 字符"
			}
		}
	}
	return ""
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// resolveCategoryID 解析分类：支持传 ID 或 slug。
func resolveCategoryID(deps Deps, id *int64, slug *string) (*int64, error) {
	if id != nil && *id > 0 {
		if _, err := deps.Taxonomy.CategoryByID(*id); err != nil {
			return nil, err
		}
		return id, nil
	}
	if slug != nil && strings.TrimSpace(*slug) != "" {
		c, err := deps.Taxonomy.CategoryBySlug(strings.TrimSpace(*slug))
		if err != nil {
			return nil, err
		}
		return &c.ID, nil
	}
	return nil, nil
}

// createPost POST /api/posts
func createPost(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body postInput
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		if msg := body.validate(false); msg != "" {
			badRequest(w, msg)
			return
		}

		claims := auth.UserFromContext(r.Context())
		catID, err := resolveCategoryID(deps, body.CategoryID, body.Category)
		if err != nil {
			badRequest(w, "分类不存在")
			return
		}

		title := strings.TrimSpace(derefStr(body.Title))
		slug := strings.TrimSpace(derefStr(body.Slug))
		if slug == "" {
			slug = taxonomy.Slugify(title, "post")
		}
		slug, err = deps.Taxonomy.UniqueSlug("posts", slug)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}

		typ := models.TypePost
		if body.Type != nil {
			typ = models.ContentType(*body.Type)
		}
		// 页面归入页面分类体系
		isPage := 0
		if typ == models.TypePage {
			isPage = 1
		}

		status := models.StatusDraft
		if body.Status != nil {
			status = models.ContentStatus(*body.Status)
		}
		// 无发布权限时强制为草稿或待审
		if !auth.Can(r.Context(), auth.CapPublishPost) {
			switch status {
			case models.StatusPublished:
				status = models.StatusPending
			case models.StatusPrivate:
				status = models.StatusDraft
			}
		}
		if typ == models.TypePage && !auth.Can(r.Context(), auth.CapManagePages) {
			httpx.WriteError(w, http.StatusForbidden, "没有权限创建页面")
			return
		}

		now := db.NowISO()
		publishedAt := any(nil)
		if status == models.StatusPublished {
			publishedAt = now
		}
		sticky := 0
		if body.Sticky != nil && *body.Sticky {
			if auth.Can(r.Context(), auth.CapPublishPost) {
				sticky = 1
			}
		}
		menuOrder := 0
		if body.MenuOrder != nil {
			menuOrder = *body.MenuOrder
		}
		authorID := any(nil)
		if claims != nil {
			authorID = claims.Sub
		}

		cols := []string{"title", "slug", "summary", "content", "cover_url",
			"category_id", "status", "type", "sticky", "password", "menu_order",
			"author_id", "created_at", "updated_at", "published_at"}
		vals := []any{title, slug, strings.TrimSpace(derefStr(body.Summary)),
			derefStr(body.Content), strings.TrimSpace(derefStr(body.CoverURL)),
			catID, string(status), string(typ), sticky,
			strings.TrimSpace(derefStr(body.Password)), menuOrder,
			authorID, now, now, publishedAt}

		// 页面需要记录所属分类体系
		if isPage == 1 {
			cols = append(cols, "template")
			vals = append(vals, strings.TrimSpace(derefStr(body.Template)))
		}

		id, err := db.Insert(deps.DB, deps.DBType, "posts", cols, vals)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}

		applyPostRelations(deps, id, &body)

		httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": id, "slug": slug})
	}
}

// updatePost PUT|PATCH /api/posts/{id}
func updatePost(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		current, err := deps.Posts.GetPostRawByID(id)
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		if !canEditPost(r, current) {
			httpx.WriteError(w, http.StatusForbidden, "没有权限编辑此文章")
			return
		}

		var body postInput
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		// PUT 与 PATCH 同为「合并更新」：未提供的字段沿用原值。
		// 这样单独改状态或分类时无需重传整个对象，
		// 避免部分字段被意外清空。
		if msg := body.validate(true); msg != "" {
			badRequest(w, msg)
			return
		}

		sets := map[string]any{}
		if body.Title != nil {
			sets["title"] = strings.TrimSpace(*body.Title)
		}
		if body.Summary != nil {
			sets["summary"] = strings.TrimSpace(*body.Summary)
		}
		if body.Content != nil {
			sets["content"] = *body.Content
		}
		if body.CoverURL != nil {
			sets["cover_url"] = strings.TrimSpace(*body.CoverURL)
		}
		if body.Password != nil {
			sets["password"] = strings.TrimSpace(*body.Password)
		}
		if body.Template != nil {
			sets["template"] = strings.TrimSpace(*body.Template)
		}
		if body.MenuOrder != nil {
			sets["menu_order"] = *body.MenuOrder
		}
		if body.Sticky != nil {
			if auth.Can(r.Context(), auth.CapPublishPost) {
				if *body.Sticky {
					sets["sticky"] = 1
				} else {
					sets["sticky"] = 0
				}
			}
		}
		if body.CategoryID != nil || body.Category != nil {
			catID, err := resolveCategoryID(deps, body.CategoryID, body.Category)
			if err != nil {
				badRequest(w, "分类不存在")
				return
			}
			sets["category_id"] = catID
		}
		// slug 支持显式修改，但需保证唯一
		if body.Slug != nil && strings.TrimSpace(*body.Slug) != "" {
			newSlug := strings.TrimSpace(*body.Slug)
			if newSlug != current.Slug {
				unique, err := deps.Taxonomy.UniqueSlug("posts", newSlug)
				if err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
					return
				}
				sets["slug"] = unique
			}
		}

		// 状态迁移
		if body.Status != nil {
			next := models.ContentStatus(*body.Status)
			if !auth.Can(r.Context(), auth.CapPublishPost) {
				switch next {
				case models.StatusPublished:
					next = models.StatusPending
				case models.StatusPrivate:
					next = models.StatusDraft
				}
			}
			sets["status"] = string(next)
			// 首次发布记录发布时间
			if next == models.StatusPublished && current.Status != models.StatusPublished {
				sets["published_at"] = db.NowISO()
			}
		} else if current.Status != models.StatusPublished &&
			sets["content"] != nil && current.Status == models.StatusDraft {
			// 内容变更但未指定状态时保持原状，不隐式发布
			_ = sets
		}
		sets["updated_at"] = db.NowISO()

		// 写历史版本：仅在正文变化时记录，避免每次点击都堆积
		if body.Content != nil && *body.Content != current.Content {
			authorID := auth.UserFromContext(r.Context())
			var aid *int64
			if authorID != nil {
				aid = &authorID.Sub
			}
			_, _ = deps.Revisions.Save(id, aid, current.Title, current.Content, current.Summary)
			_ = deps.Revisions.Prune(id, 30)
		}

		n, err := db.Update(deps.DB, deps.DBType, "posts", sets, "id = ?", id)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		if n == 0 {
			notFound(w, "文章不存在")
			return
		}

		applyPostRelations(deps, id, &body)
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id})
	}
}

// applyPostRelations 应用标签与元数据。
//
// 标签语义：tags 为「全量替换」，tag_ids 为「按 ID 精确设置」，
// 二者都不传则保持原有关联不变。
func applyPostRelations(deps Deps, id int64, body *postInput) {
	switch {
	case body.TagIDs != nil:
		if err := deps.Taxonomy.SetPostTags(id, *body.TagIDs); err != nil {
			log.Printf("[api] 设置文章标签失败 post=%d: %v", id, err)
		}
	case body.Tags != nil:
		tagIDs, err := deps.Taxonomy.ResolveTagIDs(*body.Tags)
		if err != nil {
			log.Printf("[api] 解析标签失败 post=%d: %v", id, err)
			break
		}
		if err := deps.Taxonomy.SetPostTags(id, tagIDs); err != nil {
			log.Printf("[api] 设置文章标签失败 post=%d: %v", id, err)
		}
	}
	if body.Meta != nil {
		for k, v := range *body.Meta {
			if err := deps.PostMeta.Set(id, k, v); err != nil {
				log.Printf("[api] 写入元数据失败 post=%d key=%s: %v", id, k, err)
			}
		}
	}
}

// deletePost DELETE /api/posts/{id}
func deletePost(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		post, err := deps.Posts.GetPostRawByID(id)
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		if !canEditPost(r, post) {
			httpx.WriteError(w, http.StatusForbidden, "没有权限删除此文章")
			return
		}
		n, err := db.Delete(deps.DB, deps.DBType, "posts", "id = ?", id)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		if n == 0 {
			notFound(w, "文章不存在")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// trashPost POST /api/posts/{id}/trash — 移入回收站
func trashPost(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		post, err := deps.Posts.GetPostRawByID(id)
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		if !canEditPost(r, post) {
			httpx.WriteError(w, http.StatusForbidden, "没有权限操作此文章")
			return
		}
		now := db.NowISO()
		n, err := db.Update(deps.DB, deps.DBType, "posts", map[string]any{
			"status":     string(models.StatusTrash),
			"deleted_at": now,
			"updated_at": now,
		}, "id = ?", id)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		if n == 0 {
			notFound(w, "文章不存在")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// restorePost POST /api/posts/{id}/restore — 从回收站恢复
func restorePost(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		post, err := deps.Posts.GetPostRawByID(id)
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		if !canEditPost(r, post) {
			httpx.WriteError(w, http.StatusForbidden, "没有权限操作此文章")
			return
		}
		n, err := db.Update(deps.DB, deps.DBType, "posts", map[string]any{
			"status":     string(models.StatusDraft),
			"deleted_at": nil,
			"updated_at": db.NowISO(),
		}, "id = ?", id)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		if n == 0 {
			notFound(w, "文章不存在")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// purgeTrash DELETE /api/posts/trash/purge — 清空回收站
func purgeTrash(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := db.Delete(deps.DB, deps.DBType, "posts", "deleted_at IS NOT NULL")
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": n})
	}
}

type batchRequest struct {
	IDs    []int64 `json:"ids"`
	Action string  `json:"action"`
}

// batchPosts POST /api/posts/batch
// 支持批量发布/转草稿/置顶/移入回收站。
func batchPosts(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req batchRequest
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return
		}
		if len(req.IDs) == 0 {
			badRequest(w, "未选择内容")
			return
		}
		if len(req.IDs) > 200 {
			badRequest(w, "单次最多处理 200 条")
			return
		}

		sets := map[string]any{"updated_at": db.NowISO()}
		switch req.Action {
		case "publish":
			if !auth.Can(r.Context(), auth.CapPublishPost) {
				httpx.WriteError(w, http.StatusForbidden, "没有权限发布内容")
				return
			}
			sets["status"] = string(models.StatusPublished)
			sets["published_at"] = db.NowISO()
		case "draft":
			sets["status"] = string(models.StatusDraft)
		case "sticky":
			if !auth.Can(r.Context(), auth.CapPublishPost) {
				httpx.WriteError(w, http.StatusForbidden, "没有权限置顶内容")
				return
			}
			sets["sticky"] = 1
		case "unstick":
			sets["sticky"] = 0
		case "trash":
			sets["status"] = string(models.StatusTrash)
			sets["deleted_at"] = db.NowISO()
		default:
			badRequest(w, "未知操作")
			return
		}

		ok, skipped := 0, 0
		for _, id := range req.IDs {
			post, err := deps.Posts.GetPostRawByID(id)
			if err != nil || !canEditPost(r, post) {
				skipped++
				continue
			}
			if n, err := db.Update(deps.DB, deps.DBType, "posts", sets, "id = ?", id); err == nil && n > 0 {
				ok++
			} else {
				skipped++
			}
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": ok, "skipped": skipped,
		})
	}
}
