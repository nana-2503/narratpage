package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"narratpage/internal/auth"
	"narratpage/internal/config"
	"narratpage/internal/httpx"
)

// authHandlers 认证相关接口
type authHandlers struct {
	db     *sql.DB
	secret string
	expiry time.Duration
	mw     *auth.Middleware
}

func newAuthHandlers(db *sql.DB, cfg config.Config, mw *auth.Middleware) *authHandlers {
	return &authHandlers{db: db, secret: cfg.JWTSecret, expiry: cfg.JWTExpiry, mw: mw}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// POST /api/auth/login
func (h *authHandlers) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}
	if req.Username == "" || req.Password == "" {
		httpx.WriteError(w, http.StatusBadRequest, "用户名和密码必填")
		return
	}
	var id int
	var hash, username string
	err := h.db.QueryRow(`SELECT id, username, password_hash FROM users WHERE username = ?`, req.Username).
		Scan(&id, &username, &hash)
	if err == sql.ErrNoRows || !auth.CheckPassword(req.Password, hash) {
		httpx.WriteError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	token, err := auth.Sign(h.secret, h.expiry, id, username)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"token": token, "username": username})
}

// GET /api/auth/me
func (h *authHandlers) me(w http.ResponseWriter, r *http.Request) {
	claims := auth.UserFromContext(r.Context())
	if claims == nil {
		httpx.WriteError(w, http.StatusUnauthorized, "未认证")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"username": claims.Username})
}

type passwordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

const (
	minPassword = 8
	maxPassword = 72 // bcrypt 仅使用前 72 字节
)

// POST /api/auth/password
func (h *authHandlers) changePassword(w http.ResponseWriter, r *http.Request) {
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
		httpx.WriteError(w, http.StatusBadRequest, "原密码和新密码必填")
		return
	}
	if len(req.NewPassword) < minPassword || len(req.NewPassword) > maxPassword {
		httpx.WriteError(w, http.StatusBadRequest, "新密码长度需在 "+itoa(minPassword)+"-"+itoa(maxPassword)+" 位之间")
		return
	}
	var hash string
	err := h.db.QueryRow(`SELECT password_hash FROM users WHERE id = ?`, claims.Sub).Scan(&hash)
	if err == sql.ErrNoRows {
		httpx.WriteError(w, http.StatusNotFound, "用户不存在")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	if !auth.CheckPassword(req.OldPassword, hash) {
		httpx.WriteError(w, http.StatusUnauthorized, "原密码错误")
		return
	}
	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	if _, err := h.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, newHash, claims.Sub); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// itoa 小工具（避免到处 import strconv）
func itoa(n int) string {
	return strconv.Itoa(n)
}
