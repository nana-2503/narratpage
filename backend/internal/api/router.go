package api

import (
	"log"
	"net/http"
	"strings"

	"narratpage/internal/auth"
	"narratpage/internal/config"
	"narratpage/internal/httpx"
	"narratpage/internal/ratelimit"
)

// NewRouter 组装全部路由与中间件（API 契约与 Node 版逐条一致）
func NewRouter(cfg config.Config, deps Deps) http.Handler {
	db := deps.DB
	mux := http.NewServeMux()
	mw := auth.NewMiddleware(cfg.JWTSecret)
	authH := newAuthHandlers(db, cfg, mw)

	// 健康检查
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	// 登录限流：同一 IP 每分钟最多 10 次（仅 POST /api/auth/login）
	loginLimiter := ratelimit.New(60_000_000_000, 10, "登录尝试过于频繁，请 1 分钟后再试")
	mux.HandleFunc("POST /api/auth/login", loginLimiter.Wrap(authH.login))
	mux.Handle("GET /api/auth/me", mw.Require(http.HandlerFunc(authH.me)))
	mux.Handle("POST /api/auth/password", mw.Require(http.HandlerFunc(authH.changePassword)))

	// 公开 RSS
	mux.HandleFunc("GET /api/rss.xml", rssFeed(db, cfg.SiteURL))

	// 图片：静态下载（匿名）+ 上传（管理员）
	mux.Handle("GET /api/uploads/", http.StripPrefix("/api/uploads/",
		http.FileServer(http.Dir(deps.UploadDir))))
	mux.Handle("POST /api/uploads", mw.Require(uploadImage(deps.UploadDir)))

	// 文章（列表/详情对游客开放，写操作需登录）
	mux.Handle("GET /api/posts", mw.Optional(listPosts(db)))
	mux.Handle("GET /api/posts/id/{id}", mw.Require(getPostByID(db)))
	// neighbors 用 /neighbors/{slug} 形式：若放在 {slug}/neighbors 会与
	// /id/{id} 的字面段产生无法消歧的冲突（/posts/id/neighbors 双重匹配）
	mux.Handle("GET /api/posts/neighbors/{slug}", mw.Optional(getNeighbors(db)))
	mux.Handle("GET /api/posts/{slug}", mw.Optional(getPostBySlug(db)))
	mux.Handle("POST /api/posts", mw.Require(createPost(db)))
	mux.Handle("PUT /api/posts/{id}", mw.Require(updatePost(db)))
	mux.Handle("DELETE /api/posts/{id}", mw.Require(deletePost(db)))

	// 分类
	mux.HandleFunc("GET /api/categories", listCategories(db))
	mux.Handle("POST /api/categories", mw.Require(createCategory(db)))
	mux.Handle("PUT /api/categories/{id}", mw.Require(updateCategory(db)))
	mux.Handle("DELETE /api/categories/{id}", mw.Require(deleteCategory(db)))

	// 评论
	mux.Handle("GET /api/comments", mw.Optional(listComments(db)))
	mux.HandleFunc("POST /api/comments", createComment(db))
	mux.Handle("PUT /api/comments/{id}", mw.Require(moderateComment(db)))
	mux.Handle("DELETE /api/comments/{id}", mw.Require(deleteComment(db)))

	// API 统一 404
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteError(w, http.StatusNotFound, "接口不存在")
	})

	// 前端静态产物 + SPA history 回退（一体化部署）
	if dist := cfg.FrontendDist; dist != "" && hasIndex(dist) {
		static := spaHandler(dist)
		mux.Handle("/", static)
		log.Printf("[blog] 前端产物已托管: %s", dist)
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			httpx.WriteError(w, http.StatusNotFound, "接口不存在")
		})
		log.Print("[blog] 未检测到前端产物，仅提供 API（可通过 FRONTEND_DIST 指定）")
	}

	return requestLogger(mux)
}

// spaHandler 静态文件优先，非 /api 的 GET 缺失路径回退 index.html
func spaHandler(dist string) http.Handler {
	fileServer := http.FileServer(http.Dir(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			httpx.WriteError(w, http.StatusNotFound, "接口不存在")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			httpx.WriteError(w, http.StatusNotFound, "接口不存在")
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if f, err := http.Dir(dist).Open(path); err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		http.ServeFile(w, r, dist+"/index.html")
	})
}

func hasIndex(dist string) bool {
	f, err := http.Dir(dist).Open("index.html")
	if err != nil {
		return false
	}
	f.Close()
	return true
}
