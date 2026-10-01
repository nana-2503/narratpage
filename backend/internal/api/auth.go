package api

import (
	"net/http"
	"strings"
	"time"

	"narratpage/internal/auth"
	"narratpage/internal/httpx"
	"narratpage/internal/ratelimit"
)

// authAPI 认证相关 handler。
type authAPI struct {
	deps    Deps
	limiter *ratelimit.Limiter
}

func authHandlers(deps Deps, _ any, limiter *ratelimit.Limiter) *authAPI {
	return &authAPI{deps: deps, limiter: limiter}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// POST /api/auth/login
func (a *authAPI) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		badRequest(w, "用户名和密码必填")
		return
	}

	user, hash, err := a.deps.Users.ByUsername(req.Username)
	// 用户不存在与密码错误返回同一提示，避免用户名枚举
	if err != nil || !auth.CheckPassword(req.Password, hash) {
		httpx.WriteError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if !user.Active {
		httpx.WriteError(w, http.StatusForbidden, "账号已被停用")
		return
	}

	token, jti, err := auth.Sign(a.jwtSecret(), a.jwtExpiry(), user.ID, user.Username, user.Role)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}

	// 登记会话，供「登出所有设备」与设备列表使用
	a.deps.Users.RecordSession(jti, user.ID, ratelimit.ClientIP(r, a.trustProxy()), r.UserAgent())
	a.deps.Users.TouchLogin(user.ID)

	// 登录成功后清空该 IP 的失败计数，避免正常用户被累积计数影响
	if a.limiter != nil {
		a.limiter.Reset(r)
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user": map[string]any{
			"id": user.ID, "username": user.Username,
			"display_name": user.DisplayName, "role": user.Role,
		},
	})
}

// GET /api/auth/me
func (a *authAPI) me(w http.ResponseWriter, r *http.Request) {
	claims := auth.UserFromContext(r.Context())
	if claims == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "未认证")
		return
	}
	user, err := a.deps.Users.ByID(claims.Sub)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "用户不存在")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, user)
}

const (
	minPassword = 8
	maxPassword = 72 // bcrypt 仅使用前 72 字节
)

type passwordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

// POST /api/auth/password
func (a *authAPI) changePassword(w http.ResponseWriter, r *http.Request) {
	claims := auth.UserFromContext(r.Context())
	if claims == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "未认证")
		return
	}
	var req passwordRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}
	if req.OldPassword == "" || req.NewPassword == "" {
		badRequest(w, "原密码和新密码必填")
		return
	}
	if len(req.NewPassword) < minPassword || len(req.NewPassword) > maxPassword {
		badRequest(w, "新密码长度需在 8-72 位之间")
		return
	}
	if req.OldPassword == req.NewPassword {
		badRequest(w, "新密码不能与原密码相同")
		return
	}

	user, hash, err := a.deps.Users.ByUsername(claims.Username)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "用户不存在")
		return
	}
	if !auth.CheckPassword(req.OldPassword, hash) {
		httpx.WriteError(w, http.StatusUnauthorized, "原密码错误")
		return
	}
	if err := a.deps.Users.UpdatePassword(user.ID, req.NewPassword); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	// 改密后使其它会话失效
	_, _ = a.deps.Users.RevokeOtherSessions(user.ID, claims.ID)

	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// POST /api/auth/logout
func (a *authAPI) logout(w http.ResponseWriter, r *http.Request) {
	claims := auth.UserFromContext(r.Context())
	if claims != nil && claims.ID != "" {
		_ = a.deps.Users.RevokeSession(claims.ID)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// GET /api/auth/sessions
func (a *authAPI) sessions(w http.ResponseWriter, r *http.Request) {
	claims := auth.UserFromContext(r.Context())
	items, err := a.deps.Users.ListSessions(claims.Sub)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	// 标记当前会话
	out := make([]map[string]any, 0, len(items))
	for _, s := range items {
		out = append(out, map[string]any{
			"id": s.ID, "ip": s.IP, "user_agent": s.UserAgent,
			"created_at": s.CreatedAt, "last_seen_at": s.LastSeenAt,
			"current": s.TokenID == claims.ID,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

// DELETE /api/auth/sessions/others
func (a *authAPI) revokeOthers(w http.ResponseWriter, r *http.Request) {
	claims := auth.UserFromContext(r.Context())
	n, err := a.deps.Users.RevokeOtherSessions(claims.Sub, claims.ID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "revoked": n})
}

// jwtSecret/jwtExpiry 从 deps 读取运行配置。
// 认证需要这两项，故把它们放进 Deps 而非逐个 handler 传参。
func (a *authAPI) jwtSecret() string        { return a.deps.JWTSecret }
func (a *authAPI) jwtExpiry() time.Duration { return a.deps.JWTExpiry }
func (a *authAPI) trustProxy() bool         { return a.deps.TrustProxy }
