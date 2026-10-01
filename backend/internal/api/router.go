package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"narratpage/internal/auth"
	"narratpage/internal/config"
	"narratpage/internal/httpx"
	"narratpage/internal/models"
	"narratpage/internal/ratelimit"
	"narratpage/internal/repo"
	"narratpage/internal/seed"
)

// registerRoutes 注册全部 HTTP 路由。
func registerRoutes(mux *http.ServeMux, cfg config.Config, deps Deps) {
	users := deps.Users
	mw := auth.NewMiddleware(cfg.JWTSecret)

	// 会话校验：token 需在 sessions 表中登记，支持「登出所有设备」
	if users != nil {
		mw = mw.WithSessionCheck(users.SessionAlive)
	}

	trustProxy := cfg.TrustProxy

	// ---------- 健康检查 ----------
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	// ---------- 安装 ----------
	mux.HandleFunc("GET /api/install/status", installStatusHandler(deps))
	mux.Handle("POST /api/install", mw.RequireCap(auth.CapManageOptions,
		installHandler(deps, cfg)))
	mux.Handle("POST /api/install/apply", mw.RequireCap(auth.CapManageOptions,
		installApplyHandler(deps, cfg)))

	// ---------- 认证 ----------
	loginLimiter := ratelimit.New(
		60_000_000_000, 10, "登录尝试过于频繁，请 1 分钟后再试",
		deps.Redis.RDB(), "login",
	).TrustProxy(trustProxy)
	mux.HandleFunc("POST /api/auth/login", loginLimiter.Wrap(authHandlers(deps, cfg, loginLimiter).login))

	mux.Handle("GET /api/auth/me", mw.Require(http.HandlerFunc(authHandlers(deps, cfg, nil).me)))
	mux.Handle("POST /api/auth/password", mw.Require(http.HandlerFunc(authHandlers(deps, cfg, nil).changePassword)))
	mux.Handle("POST /api/auth/logout", mw.Require(http.HandlerFunc(authHandlers(deps, cfg, nil).logout)))
	mux.Handle("GET /api/auth/sessions", mw.Require(http.HandlerFunc(authHandlers(deps, cfg, nil).sessions)))
	mux.Handle("DELETE /api/auth/sessions/others", mw.Require(http.HandlerFunc(authHandlers(deps, cfg, nil).revokeOthers)))

	// ---------- 公开内容 ----------
	//
	// 路由布局需注意：Go 1.22+ 的 ServeMux 在注册时若发现两个模式
	// 可同时匹配同一路径且无更具体者，会直接 panic。
	// 因此 /api/posts 下不设 /id/{id} 这类字面量首段（它会与
	// {id}/revisions 冲突），而是让 {slug} 同时接受数字 ID 与 slug。
	mux.Handle("GET /api/posts", mw.Optional(listPosts(deps)))
	mux.Handle("POST /api/posts", mw.Require(mw.RequireCap(auth.CapCreatePost, createPost(deps))))
	mux.Handle("POST /api/posts/batch", mw.Require(batchPosts(deps)))
	mux.Handle("DELETE /api/posts/trash/purge", mw.Require(purgeTrash(deps)))
	mux.Handle("GET /api/posts/{id}/revisions", mw.Require(listRevisions(deps)))
	mux.Handle("POST /api/posts/{id}/revisions", mw.Require(saveRevision(deps)))
	mux.Handle("POST /api/posts/{id}/revisions/{rev}/restore", mw.Require(restoreRevision(deps)))
	mux.Handle("DELETE /api/posts/{id}/revisions/{rev}", mw.Require(deleteRevision(deps)))
	mux.Handle("GET /api/posts/{id}/meta", mw.Optional(getPostMeta(deps)))
	mux.Handle("PUT /api/posts/{id}/meta", mw.Require(setPostMeta(deps)))
	mux.Handle("DELETE /api/posts/{id}/meta/{key}", mw.Require(deletePostMeta(deps)))
	mux.Handle("POST /api/posts/{id}/tags", mw.Require(addPostTag(deps)))
	mux.Handle("DELETE /api/posts/{id}/tags/{tagId}", mw.Require(removePostTag(deps)))
	mux.Handle("POST /api/posts/{id}/trash", mw.Require(trashPost(deps)))
	mux.Handle("POST /api/posts/{id}/restore", mw.Require(restorePost(deps)))
	mux.Handle("GET /api/posts/{slug}/neighbors", mw.Optional(getNeighbors(deps)))
	mux.Handle("PUT /api/posts/{id}", mw.Require(updatePost(deps)))
	mux.Handle("PATCH /api/posts/{id}", mw.Require(updatePost(deps)))
	mux.Handle("DELETE /api/posts/{id}", mw.Require(mw.RequireCap(auth.CapDeletePost, deletePost(deps))))

	// 单段路径读取：{slug} 同时接受 slug 与数字 ID
	mux.Handle("GET /api/posts/{slug}", mw.Optional(getPost(deps)))

	// ---------- 页面 ----------
	mux.Handle("GET /api/pages", mw.Optional(listPages(deps)))
	mux.Handle("GET /api/pages/{slug}", mw.Optional(getPage(deps)))

	// ---------- 分类 ----------
	mux.HandleFunc("GET /api/categories", listCategories(deps))
	mux.Handle("POST /api/categories", mw.Require(mw.RequireCap(auth.CapManageTaxonomies, createCategory(deps))))
	mux.Handle("PUT /api/categories/{id}", mw.Require(mw.RequireCap(auth.CapManageTaxonomies, updateCategory(deps))))
	mux.Handle("DELETE /api/categories/{id}", mw.Require(mw.RequireCap(auth.CapManageTaxonomies, deleteCategory(deps))))
	mux.Handle("POST /api/categories/reorder", mw.Require(mw.RequireCap(auth.CapManageTaxonomies, reorderCategories(deps))))

	// ---------- 标签 ----------
	mux.HandleFunc("GET /api/tags", listTags(deps))
	mux.Handle("POST /api/tags", mw.Require(mw.RequireCap(auth.CapManageTaxonomies, createTag(deps))))
	mux.Handle("PUT /api/tags/{id}", mw.Require(mw.RequireCap(auth.CapManageTaxonomies, updateTag(deps))))
	mux.Handle("DELETE /api/tags/{id}", mw.Require(mw.RequireCap(auth.CapManageTaxonomies, deleteTag(deps))))
	mux.Handle("POST /api/tags/merge", mw.Require(mw.RequireCap(auth.CapManageTaxonomies, mergeTags(deps))))

	// ---------- 评论 ----------
	mux.Handle("GET /api/comments", mw.Optional(listComments(deps)))
	mux.HandleFunc("POST /api/comments", createComment(deps))
	mux.Handle("PUT /api/comments/{id}", mw.Require(mw.RequireCap(auth.CapManageComments, moderateComment(deps))))
	mux.Handle("DELETE /api/comments/{id}", mw.Require(mw.RequireCap(auth.CapManageComments, deleteComment(deps))))
	mux.Handle("POST /api/comments/batch", mw.Require(mw.RequireCap(auth.CapManageComments, batchComments(deps))))
	mux.Handle("GET /api/comments/counts", mw.Require(mw.RequireCap(auth.CapManageComments, commentCounts(deps))))

	// ---------- 媒体 ----------
	mux.Handle("GET /api/uploads/", http.StripPrefix("/api/uploads/",
		http.FileServer(http.Dir(deps.UploadDir))))
	mux.Handle("POST /api/uploads", mw.Require(mw.RequireCap(auth.CapManageMedia, uploadImage(deps))))
	mux.Handle("GET /api/media", mw.Require(mw.RequireCap(auth.CapManageMedia, listMedia(deps))))
	mux.Handle("PUT /api/media/{id}", mw.Require(mw.RequireCap(auth.CapManageMedia, updateMedia(deps))))
	mux.Handle("DELETE /api/media/{id}", mw.Require(mw.RequireCap(auth.CapManageMedia, deleteMedia(deps))))

	// ---------- 用户 ----------
	mux.Handle("GET /api/users/me", mw.Require(currentUser(deps)))
	mux.Handle("PUT /api/users/me", mw.Require(updateCurrentUser(deps)))
	mux.Handle("GET /api/users", mw.Require(mw.RequireCap(auth.CapManageUsers, listUsers(deps))))
	mux.Handle("POST /api/users", mw.Require(mw.RequireCap(auth.CapManageUsers, createUser(deps))))
	mux.Handle("PUT /api/users/{id}", mw.Require(mw.RequireCap(auth.CapManageUsers, updateUser(deps))))
	mux.Handle("DELETE /api/users/{id}", mw.Require(mw.RequireCap(auth.CapManageUsers, deleteUser(deps))))

	// ---------- 站点设置 ----------
	mux.Handle("GET /api/site", siteOptions(deps))
	mux.Handle("GET /api/settings", mw.Require(mw.RequireCap(auth.CapManageOptions, siteSettings(deps))))
	mux.Handle("PUT /api/settings", mw.Require(mw.RequireCap(auth.CapManageOptions, updateSiteSettings(deps))))

	// ---------- 归档 / 聚合 ----------
	mux.HandleFunc("GET /api/archive", archiveHandler(deps))
	mux.HandleFunc("GET /api/search", searchHandler(deps))
	mux.HandleFunc("GET /api/sitemap.xml", sitemapHandler(deps))
	mux.HandleFunc("GET /api/robots.txt", robotsHandler(deps, cfg))
	mux.HandleFunc("GET /api/feed.xml", feedHandler(deps, cfg))
	mux.HandleFunc("GET /api/feed/atom.xml", atomHandler(deps, cfg))
	mux.HandleFunc("GET /api/stats", statsHandler(deps))

	// ---------- 重定向 ----------
	mux.Handle("GET /api/redirects", mw.Require(mw.RequireCap(auth.CapManageRedirects, listRedirects(deps))))
	mux.Handle("POST /api/redirects", mw.Require(mw.RequireCap(auth.CapManageRedirects, createRedirect(deps))))
	mux.Handle("DELETE /api/redirects/{id}", mw.Require(mw.RequireCap(auth.CapManageRedirects, deleteRedirect(deps))))

	// ---------- API 统一 404 ----------
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, http.StatusNotFound, "接口不存在")
	})

	// ---------- 其它：RSS 兼容路径、静态资源、SPA 回退 ----------
	mux.HandleFunc("GET /api/rss.xml", feedHandler(deps, cfg)) // 兼容旧订阅地址
	registerStatic(mux, cfg, deps)
}

// viewerFrom 从请求上下文构造可见性视角。
func viewerFrom(r *http.Request) repo.Viewer {
	claims := auth.UserFromContext(r.Context())
	if claims == nil {
		return repo.PublicViewer()
	}
	role := auth.RoleOf(claims.Role)
	return repo.Viewer{
		UserID:  claims.Sub,
		Role:    string(role),
		IsAdmin: role == auth.RoleAdmin || role == auth.RoleEditor,
		// 作者需看到自己的草稿，否则新建后立刻「消失」；
		// 贡献者同理（其内容会被降级为待审）。
		CanSeeDrafts: role != auth.RoleSubscriber,
	}
}

// canEditPost 判断当前用户能否编辑某文章。
func canEditPost(r *http.Request, post *models.Post) bool {
	claims := auth.UserFromContext(r.Context())
	if claims == nil {
		return false
	}
	if auth.Can(r.Context(), auth.CapEditAnyPost) {
		return true
	}
	// 作者只能编辑自己的文章。
	// 作者身份为 null（历史数据或导入内容）时按不可编辑处理，
	// 避免出现「无主文章人人可改」。
	return auth.Can(r.Context(), auth.CapEditOwnPost) &&
		post.AuthorID != nil && *post.AuthorID == claims.Sub
}

// canContribute 权限不足时的统一提示。
func forbidden(w http.ResponseWriter, msg string) {
	httpx.WriteError(w, http.StatusForbidden, msg)
}

// buildPostQuery 从请求参数构造列表查询。
func buildPostQuery(r *http.Request, defaultType models.ContentType) repo.PostQuery {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	if pageSize <= 0 {
		pageSize = 10
	}
	year, _ := strconv.Atoi(q.Get("year"))
	month, _ := strconv.Atoi(q.Get("month"))
	author, _ := strconv.ParseInt(q.Get("author"), 10, 64)

	typ := defaultType
	if v := q.Get("type"); v != "" {
		typ = models.ContentType(v)
	}

	return repo.PostQuery{
		Type:     typ,
		Status:   q.Get("status"),
		Category: q.Get("category"),
		Tag:      q.Get("tag"),
		Author:   author,
		Search:   strings.TrimSpace(q.Get("q")),
		Year:     year,
		Month:    month,
		Sticky:   q.Get("sticky") == "1" || q.Get("sticky") == "true",
		Page:     page,
		PageSize: pageSize,
		Order:    q.Get("order"),
	}
}

// ensureInstalled 保证有默认设置与种子数据（首次启动时调用）。
// cmd/server 已自行完成同样的编排，此处保留给其它入口复用。
func ensureInstalled(database *sql.DB, cfg config.Config, options *repo.Options) error {
	installed, err := CheckInstalled(database)
	if err != nil {
		return err
	}
	if !installed {
		if err := seed.Run(database, cfg); err != nil {
			return err
		}
	}
	return options.EnsureDefaults()
}
