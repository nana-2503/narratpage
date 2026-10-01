package api

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"narratpage/internal/config"
	"narratpage/internal/httpx"
)

// registerStatic 注册前端静态产物托管、SPA 回退与重定向。
func registerStatic(mux *http.ServeMux, cfg config.Config, deps Deps) {
	dist := cfg.FrontendDist
	if dist == "" {
		dist = resolveFrontendDist()
	}

	// 重定向规则优先于静态资源：/old-slug 这类旧链接需要先被接管
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// API 路径不参与 SPA 回退
		if strings.HasPrefix(r.URL.Path, "/api/") {
			httpx.WriteError(w, http.StatusNotFound, "接口不存在")
			return
		}
		// 仅 GET/HEAD 可重定向
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			if to, ok := deps.Redirects.Find(normalizePath(r.URL.Path)); ok {
				// 永久跳转，保住外链权重
				http.Redirect(w, r, to, http.StatusMovedPermanently)
				return
			}
		}

		if dist == "" || !hasIndex(dist) {
			httpx.WriteError(w, http.StatusNotFound, "资源不存在")
			return
		}
		serveSPA(w, r, dist)
	})
}

func hasIndex(dist string) bool {
	f, err := os.Open(filepath.Join(dist, "index.html"))
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// serveSPA 静态文件优先，缺失时回退 index.html 交给前端路由。
func serveSPA(w http.ResponseWriter, r *http.Request, dist string) {
	clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	// 阻断路径穿越：解析后必须仍在 dist 之内
	full := filepath.Join(dist, clean)
	if !strings.HasPrefix(full, filepath.Clean(dist)+string(os.PathSeparator)) && full != filepath.Clean(dist) {
		httpx.WriteError(w, http.StatusNotFound, "资源不存在")
		return
	}
	if info, err := os.Stat(full); err == nil && !info.IsDir() {
		// 带内容指纹的资源可长缓存，index.html 必须每次校验
		if strings.HasPrefix(clean, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.ServeFile(w, r, full)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, filepath.Join(dist, "index.html"))
}

// resolveFrontendDist 未显式指定时按可执行文件目录推断（一体化部署约定）。
func resolveFrontendDist() string {
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "frontend", "dist")
		if hasIndex(candidate) {
			return candidate
		}
	}
	if hasIndex(filepath.Join("frontend", "dist")) {
		return "frontend/dist"
	}
	return ""
}

// CheckInstalled 以 users 表是否有数据作为安装完成标志。
func CheckInstalled(d *sql.DB) (bool, error) {
	var n int
	if err := d.QueryRow("SELECT COUNT(*) FROM users").Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}
