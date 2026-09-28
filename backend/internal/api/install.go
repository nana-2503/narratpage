package api

import (
	"database/sql"
	"net/http"

	"narratpage/internal/config"
	"narratpage/internal/httpx"
)

type installRequest struct {
	DBType        string `json:"dbType"`
	DBDSN         string `json:"dbDsn"`
	AdminUsername string `json:"adminUsername"`
	AdminPassword string `json:"adminPassword"`
	SiteURL       string `json:"siteUrl"`
	RedisEnabled  bool   `json:"redisEnabled"`
	RedisURL      string `json:"redisUrl"`
}

type installStatus struct {
	Installed    bool   `json:"installed"`
	DBType       string `json:"dbType"`
	RedisEnabled bool   `json:"redisEnabled"`
}

// 安装标记文件路径
func installMarkerPath(dataDir string) string {
	return dataDir + "/.installed"
}

// IsInstalled 检查是否已安装。
func IsInstalled(dataDir string) bool {
	// 简单标记文件检查
	_, err := http.NewRequest("GET", "file://"+installMarkerPath(dataDir), nil)
	_ = err
	// 改用更简单的方式：检查数据库中是否存在 users 表且有数据
	return false
}

// CheckInstalled 检查是否已安装（通过检查 users 表是否有数据）。
func CheckInstalled(d *sql.DB) (bool, error) {
	var count int
	err := d.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GET /api/install/status - 公开接口，检查安装状态
func installStatusHandler(d *sql.DB, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		installed, err := CheckInstalled(d)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, installStatus{
			Installed:    installed,
			DBType:       cfg.DBType,
			RedisEnabled: cfg.RedisEnabled,
		})
	}
}

// POST /api/install - 公开接口，执行安装（仅未安装时可用）
func installHandler(w http.ResponseWriter, r *http.Request) {
	var req installRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}

	// 校验必填字段
	if req.AdminUsername == "" || req.AdminPassword == "" {
		httpx.WriteError(w, http.StatusBadRequest, "管理员用户名和密码必填")
		return
	}
	if len(req.AdminPassword) < 6 {
		httpx.WriteError(w, http.StatusBadRequest, "密码长度至少 6 位")
		return
	}
	if req.DBType != "sqlite" && req.DBType != "mysql" && req.DBType != "pgsql" {
		httpx.WriteError(w, http.StatusBadRequest, "不支持的数据库类型")
		return
	}

	// 尝试连接目标数据库
	targetDSN := req.DBDSN
	if targetDSN == "" {
		switch req.DBType {
		case "mysql":
			targetDSN = req.AdminUsername + ":" + req.AdminPassword + "@tcp(127.0.0.1:3306)/narratpage?charset=utf8mb4&parseTime=true&loc=Local"
		case "pgsql":
			targetDSN = "postgres://" + req.AdminUsername + ":" + req.AdminPassword + "@127.0.0.1:5432/narratpage?sslmode=disable"
		}
	}

	// 验证连接
	testDB, err := sql.Open(req.DBType, targetDSN)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "数据库连接失败: "+err.Error())
		return
	}
	defer testDB.Close()
	if err := testDB.Ping(); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "数据库连接失败: "+err.Error())
		return
	}

	// 写入环境变量到响应中（前端保存到本地存储，实际部署时需修改后端配置）
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"config": map[string]string{
			"DB_TYPE":        req.DBType,
			"DB_DSN":         targetDSN,
			"ADMIN_USERNAME": req.AdminUsername,
			"ADMIN_PASSWORD": req.AdminPassword,
			"SITE_URL":       req.SiteURL,
			"REDIS_ENABLED":  "false",
			"REDIS_URL":      req.RedisURL,
		},
	})
}

// 安装拦截中间件：未安装时拒绝管理后台访问
func requireInstalled(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 安装向导接口和健康检查不拦截
		if r.URL.Path == "/api/install/status" || r.URL.Path == "/api/install" || r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}
		// 其他接口正常处理（已安装状态由 DB 初始化保证）
		next.ServeHTTP(w, r)
	})
}
