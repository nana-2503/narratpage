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
	"narratpage/internal/seed"
)

// installStatusHandler GET /api/install/status
func installStatusHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		installed, err := CheckInstalled(deps.DB)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"installed":    installed,
			"dbType":       deps.DBType,
			"redisEnabled": deps.Redis != nil,
		})
	}
}

// installHandler POST /api/install
//
// 第一步：校验配置并试连目标数据库，返回可直接写入 .env 的片段。
// 这一步不改动任何持久化状态，因此可以安全地反复尝试。
func installHandler(deps Deps, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req models.InstallRequest
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return
		}

		username := strings.TrimSpace(req.AdminUsername)
		if username == "" {
			badRequest(w, "管理员用户名必填")
			return
		}
		if len(req.AdminPassword) < 8 {
			badRequest(w, "密码长度至少 8 位")
			return
		}
		if len(req.AdminPassword) > 72 {
			badRequest(w, "密码最长 72 位")
			return
		}
		switch req.DBType {
		case "sqlite", "mysql", "pgsql":
		default:
			badRequest(w, "不支持的数据库类型")
			return
		}
		siteTitle := strings.TrimSpace(req.SiteTitle)
		if siteTitle == "" {
			siteTitle = models.DefaultSiteOptions().Title
		}

		// 校验目标库可连接
		dsn := strings.TrimSpace(req.DBDSN)
		if req.DBType != "sqlite" && dsn == "" {
			badRequest(w, "请填写数据库连接串")
			return
		}
		if req.DBType != "sqlite" {
			probe, err := sql.Open(req.DBType, dsn)
			if err != nil {
				badRequest(w, "数据库连接失败")
				return
			}
			defer probe.Close()
			if err := probe.Ping(); err != nil {
				badRequest(w, "数据库连接失败，请检查地址、账号与密码")
				return
			}
		}

		// 用户名唯一性
		if taken, _ := deps.Users.UsernameExists(username, 0); taken {
			httpx.WriteError(w, http.StatusConflict, "用户名已存在")
			return
		}

		siteURL := strings.TrimRight(strings.TrimSpace(req.SiteURL), "/")

		httpx.WriteJSON(w, http.StatusOK, models.InstallResult{
			OK:        true,
			Username:  username,
			SiteTitle: siteTitle,
			DBType:    req.DBType,
			EnvSnippet: map[string]string{
				"DB_TYPE":        req.DBType,
				"DB_DSN":         dsn,
				"ADMIN_USERNAME": username,
				"ADMIN_PASSWORD": req.AdminPassword,
				"SITE_URL":       siteURL,
				"REDIS_ENABLED":  strconv.FormatBool(req.RedisEnabled),
				"REDIS_URL":      req.RedisURL,
			},
			NeedRestart: true,
		})
	}
}

// installApplyHandler POST /api/install/apply
//
// 第二步：在当前数据库上真正创建管理员与站点设置。
//
// 说明：DB_TYPE / DB_DSN 属于进程级配置，无法在运行期切换，
// 因此切换数据库需重启容器；本接口负责的是「把账号与站点设置
// 落到当前已连接的库里」，这也是原实现缺失的关键一环——
// 之前走完向导，密码并不会真正生效。
func installApplyHandler(deps Deps, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		installed, err := CheckInstalled(deps.DB)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		if installed {
			httpx.WriteError(w, http.StatusConflict, "系统已安装")
			return
		}

		var req models.InstallRequest
		if err := httpx.DecodeJSON(w, r, &req); err != nil {
			return
		}
		username := strings.TrimSpace(req.AdminUsername)
		if username == "" || len(req.AdminPassword) < 8 {
			badRequest(w, "请提供用户名与至少 8 位密码")
			return
		}

		// 创建管理员
		admin := &models.User{
			Username:    username,
			Email:       strings.TrimSpace(req.AdminEmail),
			DisplayName: username,
			Role:        string(auth.RoleAdmin),
			Active:      true,
		}
		if _, err := deps.Users.Create(admin, req.AdminPassword); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "创建管理员失败")
			return
		}

		// 写入站点设置
		site := models.DefaultSiteOptions()
		if t := strings.TrimSpace(req.SiteTitle); t != "" {
			site.Title = t
		}
		if u := strings.TrimRight(strings.TrimSpace(req.SiteURL), "/"); u != "" {
			site.URL = u
		}
		if err := deps.Options.SaveSite(site); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "写入站点设置失败")
			return
		}

		// 首次安装写入示例内容，便于立即看到效果
		if err := seed.Run(deps.DB, config.Config{
			AdminUsername: username,
			AdminPassword: req.AdminPassword,
		}); err != nil {
			// 示例数据失败不影响安装结果
			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"ok": true, "username": username, "seeded": false,
			})
			return
		}

		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true, "username": username, "seeded": true,
		})
	}
}
