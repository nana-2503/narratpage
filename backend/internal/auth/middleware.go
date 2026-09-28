package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

func writeErr(w http.ResponseWriter, msg string) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

type ctxKey int

const userKey ctxKey = iota

// UserFromContext 取出中间件解析出的用户（无则为 nil）
func UserFromContext(ctx context.Context) *Claims {
	if v, ok := ctx.Value(userKey).(*Claims); ok {
		return v
	}
	return nil
}

// 从 Authorization: Bearer <token> 提取 token
func extractToken(r *http.Request) (string, bool) {
	h := r.Header.Get("authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	return token, token != ""
}

// Middleware 认证中间件集合（签名与 Node 版语义一致）
type Middleware struct {
	secret string
}

func NewMiddleware(secret string) *Middleware {
	return &Middleware{secret: secret}
}

// Optional token 无效按游客处理，继续放行
func (m *Middleware) Optional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token, ok := extractToken(r); ok {
			if claims, err := Parse(m.secret, token); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), userKey, claims))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Require 无 token 或 token 无效时 401
func (m *Middleware) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := extractToken(r)
		if !ok {
			writeErr(w, "未认证")
			return
		}
		claims, err := Parse(m.secret, token)
		if err != nil {
			writeErr(w, "登录已过期，请重新登录")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, claims)))
	})
}
