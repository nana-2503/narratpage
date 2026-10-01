// Package api 提供 HTTP 路由与 handler。
//
// 分层约定：handler 只做「解析请求 → 调用 repo → 写响应」，
// 业务规则与可见性判定一律下沉到 repo 层。
package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"narratpage/internal/config"
	"narratpage/internal/httpx"
	"narratpage/internal/redis"
	"narratpage/internal/repo"
)

// Deps 处理器依赖。
type Deps struct {
	DB        *sql.DB
	DBType    string
	UploadDir string
	SiteURL   string
	JWTSecret string
	JWTExpiry time.Duration
	// TrustProxy 声明后端位于可信反向代理之后，
	// 影响限流取 IP 的方式（见 ratelimit.ClientIP）。
	TrustProxy bool
	Redis      *redis.Client

	// 仓储
	Posts     *repo.Repo
	Options   *repo.Options
	Users     *repo.Users
	Taxonomy  *repo.Taxonomy
	Media     *repo.Media
	PostMeta  *repo.PostMeta
	Revisions *repo.Revisions
	Redirects *repo.Redirects
	Comments  *repo.Comments
}

// NewRouter 组装全部路由与中间件。
func NewRouter(cfg config.Config, deps Deps) http.Handler {
	mux := http.NewServeMux()
	registerRoutes(mux, cfg, deps)
	return requestLogger(cors(mux))
}

// cors 处理跨域。默认同源部署无需此中间件，但前后端分离部署
// （前端挂 CDN、后端独立域名）时必须放行 OPTIONS 预检。
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// errIsNotFound 判断错误是否为「资源不存在」。
func errIsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, repo.ErrNotFound)
}

// notFound 输出 404。
func notFound(w http.ResponseWriter, msg string) {
	httpx.WriteError(w, http.StatusNotFound, msg)
}

// badRequest 输出 400。
func badRequest(w http.ResponseWriter, msg string) {
	httpx.WriteError(w, http.StatusBadRequest, msg)
}

// normalizePath 归一化路径：确保以 / 开头、去掉尾部斜杠。
func normalizePath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	return p
}
