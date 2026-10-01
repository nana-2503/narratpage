package api

import (
	"net/http"
	"strconv"
	"strings"

	"narratpage/internal/auth"
	"narratpage/internal/db"
	"narratpage/internal/httpx"
	"narratpage/internal/models"
	"narratpage/internal/repo"
)

// ---------- 页面 ----------

// listPages GET /api/pages
func listPages(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := buildPostQuery(r, models.TypePage)
		// 页面默认按菜单顺序排序
		if q.Order == "" {
			q.Order = "meta"
		}
		result, err := deps.Posts.ListPosts(q, viewerFrom(r))
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, result)
	}
}

// getPage GET /api/pages/{slug}
func getPage(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		post, err := deps.Posts.GetPostBySlug(slug, viewerFrom(r))
		if err != nil {
			notFound(w, "页面不存在")
			return
		}
		// 路径命中页面，未命中则回退到同 slug 的文章
		if post.Type != models.TypePage {
			notFound(w, "页面不存在")
			return
		}
		post.Views = deps.Posts.IncrementViews(post.ID)
		httpx.WriteJSON(w, http.StatusOK, post)
	}
}

// ---------- 分类 ----------

type categoryInput struct {
	Name        *string `json:"name"`
	Slug        *string `json:"slug"`
	Description *string `json:"description"`
	IsPage      *int    `json:"is_page"`
}

// listCategories GET /api/categories
func listCategories(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isPage := 0
		if r.URL.Query().Get("page") == "1" {
			isPage = 1
		}
		items, err := deps.Taxonomy.ListCategories(isPage, viewerFrom(r))
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

// createCategory POST /api/categories
func createCategory(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body categoryInput
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		name := strings.TrimSpace(derefStr(body.Name))
		if name == "" {
			badRequest(w, "分类名称必填")
			return
		}
		if len(name) > 50 {
			badRequest(w, "分类名最长 50 字符")
			return
		}
		slug := strings.TrimSpace(derefStr(body.Slug))
		if slug == "" {
			slug = repo.Slugify(name, "cat")
		}
		if taken, _ := deps.Taxonomy.CategorySlugTaken(slug, 0); taken {
			httpx.WriteError(w, http.StatusConflict, "slug 已存在")
			return
		}
		isPage := 0
		if body.IsPage != nil {
			isPage = *body.IsPage
		}
		id, err := deps.Taxonomy.CreateCategory(name, slug, derefStr(body.Description), isPage)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusCreated, map[string]any{
			"id": id, "name": name, "slug": slug,
		})
	}
}

// updateCategory PUT /api/categories/{id}
// 未传 slug 时保留原 slug，避免既有链接失效。
func updateCategory(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "分类不存在")
			return
		}
		current, err := deps.Taxonomy.CategoryByID(id)
		if err != nil {
			notFound(w, "分类不存在")
			return
		}
		var body categoryInput
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		name := strings.TrimSpace(derefStr(body.Name))
		if name == "" {
			name = current.Name
		}
		slug := strings.TrimSpace(derefStr(body.Slug))
		if slug == "" {
			slug = current.Slug
		}
		if taken, _ := deps.Taxonomy.CategorySlugTaken(slug, id); taken {
			httpx.WriteError(w, http.StatusConflict, "slug 已存在")
			return
		}
		desc := current.Description
		if body.Description != nil {
			desc = *body.Description
		}
		if err := deps.Taxonomy.UpdateCategory(id, name, slug, desc); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "slug": slug})
	}
}

// deleteCategory DELETE /api/categories/{id}
func deleteCategory(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "分类不存在")
			return
		}
		n, err := deps.Taxonomy.DeleteCategory(id)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		if n == 0 {
			notFound(w, "分类不存在")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// reorderCategories POST /api/categories/reorder
func reorderCategories(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			IDs []int64 `json:"ids"`
		}
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		if err := deps.Taxonomy.ReorderCategories(body.IDs); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// ---------- 标签 ----------

type tagInput struct {
	Name        *string `json:"name"`
	Slug        *string `json:"slug"`
	Description *string `json:"description"`
}

// listTags GET /api/tags
func listTags(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := deps.Taxonomy.ListTags(r.URL.Query().Get("q"))
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

// createTag POST /api/tags
func createTag(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body tagInput
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		name := strings.TrimSpace(derefStr(body.Name))
		if name == "" {
			badRequest(w, "标签名称必填")
			return
		}
		if len(name) > 50 {
			badRequest(w, "标签名最长 50 字符")
			return
		}
		slug := strings.TrimSpace(derefStr(body.Slug))
		if slug == "" {
			slug = repo.Slugify(name, "tag")
		}
		id, err := deps.Taxonomy.CreateTag(name, slug, derefStr(body.Description))
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusCreated, map[string]any{
			"id": id, "name": name, "slug": slug,
		})
	}
}

// updateTag PUT /api/tags/{id}
func updateTag(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "标签不存在")
			return
		}
		var body tagInput
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		name := strings.TrimSpace(derefStr(body.Name))
		if name == "" {
			badRequest(w, "标签名称必填")
			return
		}
		slug := strings.TrimSpace(derefStr(body.Slug))
		if slug == "" {
			slug = repo.Slugify(name, "tag")
		}
		if taken, _ := deps.Taxonomy.TagSlugTaken(slug, id); taken {
			httpx.WriteError(w, http.StatusConflict, "slug 已存在")
			return
		}
		if err := deps.Taxonomy.UpdateTag(id, name, slug, derefStr(body.Description)); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": id, "slug": slug})
	}
}

// deleteTag DELETE /api/tags/{id}
func deleteTag(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "标签不存在")
			return
		}
		n, err := deps.Taxonomy.DeleteTag(id)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		if n == 0 {
			notFound(w, "标签不存在")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// mergeTags POST /api/tags/merge
// 将 source 的所有文章关联迁移到 target，然后删除 source。
func mergeTags(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Source int64 `json:"source"`
			Target int64 `json:"target"`
		}
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		if body.Source == 0 || body.Target == 0 || body.Source == body.Target {
			badRequest(w, "请选择两个不同的标签")
			return
		}
		// 逐条迁移，冲突自动忽略（InsertOrIgnore）
		postIDs, err := db.Query(deps.DB, deps.DBType,
			"SELECT post_id FROM post_tags WHERE tag_id = ?", body.Source)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		var ids []int64
		for postIDs.Next() {
			var id int64
			if err := postIDs.Scan(&id); err == nil {
				ids = append(ids, id)
			}
		}
		postIDs.Close()

		moved := 0
		for _, pid := range ids {
			if ok, err := db.InsertOrIgnore(deps.DB, deps.DBType, "post_tags",
				[]string{"post_id", "tag_id"}, []any{pid, body.Target}); err == nil && ok {
				moved++
			}
		}
		if _, err := deps.Taxonomy.DeleteTag(body.Source); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "moved": moved})
	}
}

// ---------- 文章标签关联 ----------

// addPostTag POST /api/posts/{id}/tags
func addPostTag(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		var body struct {
			TagID   int64  `json:"tag_id"`
			TagName string `json:"tag_name"`
		}
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		tagID := body.TagID
		if tagID == 0 && strings.TrimSpace(body.TagName) != "" {
			ids, err := deps.Taxonomy.ResolveTagIDs([]string{body.TagName})
			if err != nil || len(ids) == 0 {
				badRequest(w, "标签无效")
				return
			}
			tagID = ids[0]
		}
		if tagID == 0 {
			badRequest(w, "请指定标签")
			return
		}
		if err := deps.Taxonomy.AddPostTag(postID, tagID); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// removePostTag DELETE /api/posts/{id}/tags/{tagId}
func removePostTag(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		tagID, err := strconv.ParseInt(r.PathValue("tagId"), 10, 64)
		if err != nil {
			notFound(w, "标签不存在")
			return
		}
		if err := deps.Taxonomy.RemovePostTag(postID, tagID); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// ---------- 元数据 ----------

// getPostMeta GET /api/posts/{id}/meta
func getPostMeta(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		items, err := deps.PostMeta.All(id)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

// setPostMeta PUT /api/posts/{id}/meta
func setPostMeta(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		var body map[string]string
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		if len(body) == 0 {
			badRequest(w, "没有要写入的元数据")
			return
		}
		if len(body) > 50 {
			badRequest(w, "单次最多写入 50 个元数据项")
			return
		}
		for k, v := range body {
			if err := deps.PostMeta.Set(id, k, v); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
				return
			}
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// deletePostMeta DELETE /api/posts/{id}/meta/{key}
func deletePostMeta(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		if err := deps.PostMeta.DeleteKey(id, r.PathValue("key")); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// ---------- 历史版本 ----------

// listRevisions GET /api/posts/{id}/revisions
func listRevisions(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		items, err := deps.Revisions.List(id, 30)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

// saveRevision POST /api/posts/{id}/revisions
func saveRevision(deps Deps) http.HandlerFunc {
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
		claims := auth.UserFromContext(r.Context())
		var aid *int64
		if claims != nil {
			aid = &claims.Sub
		}
		revID, err := deps.Revisions.Save(id, aid, post.Title, post.Content, post.Summary)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		_ = deps.Revisions.Prune(id, 30)
		httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": revID})
	}
}

// restoreRevision POST /api/posts/{id}/revisions/{rev}/restore
func restoreRevision(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		revID, err := strconv.ParseInt(r.PathValue("rev"), 10, 64)
		if err != nil {
			notFound(w, "版本不存在")
			return
		}
		post, err := deps.Posts.GetPostRawByID(postID)
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		if !canEditPost(r, post) {
			httpx.WriteError(w, http.StatusForbidden, "没有权限操作此文章")
			return
		}
		rev, err := deps.Revisions.ByID(revID)
		if err != nil {
			notFound(w, "版本不存在")
			return
		}
		// 版本必须属于该文章，防止越权读取
		if rev.PostID != postID {
			notFound(w, "版本不存在")
			return
		}
		// 回滚前先为当前内容存一份，避免回滚本身丢失最新版
		claims := auth.UserFromContext(r.Context())
		var aid *int64
		if claims != nil {
			aid = &claims.Sub
		}
		_, _ = deps.Revisions.Save(postID, aid, post.Title, post.Content, post.Summary)

		n, err := db.Update(deps.DB, deps.DBType, "posts", map[string]any{
			"title":      rev.Title,
			"content":    rev.Content,
			"updated_at": db.NowISO(),
		}, "id = ?", postID)
		if err != nil || n == 0 {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

// deleteRevision DELETE /api/posts/{id}/revisions/{rev}
func deleteRevision(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "文章不存在")
			return
		}
		revID, err := strconv.ParseInt(r.PathValue("rev"), 10, 64)
		if err != nil {
			notFound(w, "版本不存在")
			return
		}
		rev, err := deps.Revisions.ByID(revID)
		if err != nil || rev.PostID != postID {
			notFound(w, "版本不存在")
			return
		}
		if _, err := deps.Revisions.Delete(revID); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
