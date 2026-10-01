package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// capRoute 构造一条受 RequireCap 保护的路由，返回实际状态码与错误文案。
func capRoute(m *Middleware, c Capability) func(token string) (int, string) {
	return func(token string) (int, string) {
		h := m.RequireCap(c, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		req := httptest.NewRequest("GET", "/x", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var body struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body.Error
	}
}

const secret = "test-secret"

// signLegacy 签发一个「旧版本」token：不含 role 声明，
// 模拟 RBAC 上线前签发、至今仍未过期的凭据。
func signLegacy(t *testing.T) string {
	t.Helper()
	claims := Claims{
		Sub:      1,
		Username: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        randomID(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

// TestLegacyTokenGetsRoleFromDB 覆盖线上 403 的根因：
// RBAC 上线前签发、不含 role 声明的 token，被 RoleOf 降级成
// subscriber，导致所有 RequireCap 端点 403。会话校验应把库里的角色补上。
func TestLegacyTokenGetsRoleFromDB(t *testing.T) {
	legacy := signLegacy(t)

	m := NewMiddleware(secret).WithSessionCheck(func(jti string, uid int64) (string, bool) {
		return "admin", true
	})

	if code, msg := capRoute(m, CapManageUsers)(legacy); code != http.StatusOK {
		t.Fatalf("旧 token 应按库中角色放行，得到 %d %q", code, msg)
	}
}

// TestRoleDowngradeTakesEffectImmediately 降权后旧 token 不应继续提权。
func TestRoleDowngradeTakesEffectImmediately(t *testing.T) {
	tok, _, err := Sign(secret, time.Hour, 1, "admin", "admin")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	role := "admin"
	m := NewMiddleware(secret).WithSessionCheck(func(string, int64) (string, bool) {
		return role, true
	})
	call := capRoute(m, CapManageUsers)

	if code, _ := call(tok); code != http.StatusOK {
		t.Fatalf("初始应为 200，得到 %d", code)
	}
	role = "subscriber"
	code, msg := call(tok)
	if code != http.StatusForbidden {
		t.Fatalf("降权后应为 403，得到 %d", code)
	}
	if !strings.Contains(msg, "subscriber") || !strings.Contains(msg, string(CapManageUsers)) {
		t.Errorf("403 文案应含角色与权限点，实际 %q", msg)
	}
}

// TestDeactivatedUserRejected active=false 时会话校验失败。
func TestDeactivatedUserRejected(t *testing.T) {
	tok, _, err := Sign(secret, time.Hour, 1, "admin", "admin")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	m := NewMiddleware(secret).WithSessionCheck(func(string, int64) (string, bool) {
		return "", false // SessionAlive 在 active=0 时返回 ok=false
	})
	if code, _ := capRoute(m, CapManageUsers)(tok); code != http.StatusUnauthorized {
		t.Fatalf("停用后应为 401，得到 %d", code)
	}
}

// TestRolelessTokenWithoutSessionCheck 无会话校验且 token 无角色时
// 应按凭据失效处理（401），而不是含糊地报 403。
func TestRolelessTokenWithoutSessionCheck(t *testing.T) {
	legacy := signLegacy(t)
	m := NewMiddleware(secret) // 未注入 sessionCheck
	code, _ := capRoute(m, CapManageUsers)(legacy)
	if code != http.StatusUnauthorized {
		t.Fatalf("期望 401，实际 %d", code)
	}
}
