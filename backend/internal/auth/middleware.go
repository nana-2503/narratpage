package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"crypto/rand"
	"encoding/hex"
)

// randomID 生成 16 字节随机十六进制串，用作 token jti。
func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 失败属系统级异常，无可安全降级的路径
		panic("auth: 无法读取加密随机源: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// ---------- 角色与权限 ----------

// Role 角色。
type Role string

const (
	RoleAdmin       Role = "admin"       // 全部权限，含用户与站点设置
	RoleEditor      Role = "editor"      // 可发布/编辑/删除他人文章
	RoleAuthor      Role = "author"      // 可编辑自己的文章
	RoleContributor Role = "contributor" // 只能写草稿，不能发布
	RoleSubscriber  Role = "subscriber"  // 只能评论
	RoleAnonymous   Role = "anonymous"   // 未登录
)

// Capability 具体权限点。
type Capability string

const (
	CapCreatePost       Capability = "post.create"
	CapEditAnyPost      Capability = "post.edit_any"
	CapEditOwnPost      Capability = "post.edit_own"
	CapDeletePost       Capability = "post.delete"
	CapPublishPost      Capability = "post.publish"
	CapManagePages      Capability = "page.manage"
	CapManageMedia      Capability = "media.manage"
	CapManageComments   Capability = "comment.manage"
	CapManageTaxonomies Capability = "taxonomy.manage"
	CapManageOptions    Capability = "options.manage"
	CapManageUsers      Capability = "user.manage"
	CapManageRedirects  Capability = "redirect.manage"
)

// roleCaps 角色到权限的映射。新增角色时只需在此登记一处。
var roleCaps = map[Role]map[Capability]bool{
	RoleAdmin: {
		CapCreatePost: true, CapEditAnyPost: true, CapEditOwnPost: true,
		CapDeletePost: true, CapPublishPost: true, CapManagePages: true,
		CapManageMedia: true, CapManageComments: true, CapManageTaxonomies: true,
		CapManageOptions: true, CapManageUsers: true, CapManageRedirects: true,
	},
	RoleEditor: {
		CapCreatePost: true, CapEditAnyPost: true, CapEditOwnPost: true,
		CapDeletePost: true, CapPublishPost: true, CapManagePages: true,
		CapManageMedia: true, CapManageComments: true, CapManageTaxonomies: true,
	},
	RoleAuthor: {
		CapCreatePost: true, CapEditOwnPost: true, CapPublishPost: true,
		CapManageMedia: true,
	},
	RoleContributor: {
		CapCreatePost: true, CapEditOwnPost: true,
	},
	RoleSubscriber: {},
	RoleAnonymous:  {},
}

// Can 判断角色是否具备某权限。
func (r Role) Can(c Capability) bool {
	return roleCaps[r][c]
}

// Valid 报告是否为已知角色。
func (r Role) Valid() bool {
	_, ok := roleCaps[r]
	return ok
}

// RoleOf 从字符串解析角色，未知值降级为最低权限。
func RoleOf(s string) Role {
	r := Role(s)
	if !r.Valid() {
		return RoleSubscriber
	}
	return r
}

// ---------- 上下文 ----------

type ctxKey int

const (
	userKey ctxKey = iota
)

// UserFromContext 取出中间件解析出的用户（无则为 nil）。
func UserFromContext(ctx context.Context) *Claims {
	if v, ok := ctx.Value(userKey).(*Claims); ok {
		return v
	}
	return nil
}

// RoleFromContext 返回当前请求的角色，未登录为 anonymous。
func RoleFromContext(ctx context.Context) Role {
	if u := UserFromContext(ctx); u != nil {
		return RoleOf(u.Role)
	}
	return RoleAnonymous
}

// Can 从上下文判断当前身份是否具备某权限。
func Can(ctx context.Context, c Capability) bool {
	return RoleFromContext(ctx).Can(c)
}

func writeErr(w http.ResponseWriter, msg string) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// 从 Authorization: Bearer <token> 提取 token。
func extractToken(r *http.Request) (string, bool) {
	h := r.Header.Get("authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	return token, token != ""
}

// Middleware 认证与授权中间件集合。
type Middleware struct {
	secret string
	// sessionCheck 可选：校验 token 是否仍在有效会话中，并返回该用户
	// 当前角色，用于支持「登出所有设备」与「降权立即生效」。
	// 为 nil 时跳过（纯无状态模式），此时角色只能取 token 里的声明。
	sessionCheck func(jti string, userID int64) (role string, ok bool)
}

func NewMiddleware(secret string) *Middleware {
	return &Middleware{secret: secret}
}

// WithSessionCheck 注入会话校验。
func (m *Middleware) WithSessionCheck(fn func(jti string, userID int64) (string, bool)) *Middleware {
	m.sessionCheck = fn
	return m
}

func (m *Middleware) resolve(r *http.Request) (*Claims, bool) {
	token, ok := extractToken(r)
	if !ok {
		return nil, false
	}
	claims, err := Parse(m.secret, token)
	if err != nil {
		return nil, false
	}
	if m.sessionCheck != nil && claims.ID != "" {
		role, alive := m.sessionCheck(claims.ID, claims.Sub)
		if !alive {
			return nil, false
		}
		// 角色以数据库为准，不信任 token 里的快照：
		// 降权、停用要在下一个请求就生效，否则旧 token 在有效期内仍能提权。
		claims.Role = role
	} else if claims.Role == "" {
		// 已认证但没有角色：无法判定权限（如 RBAC 上线前签发的旧 token）。
		// 按凭据失效处理并要求重新登录，比报 403 更贴近真实原因。
		return nil, false
	}
	return claims, true
}

// Optional token 无效按游客处理，继续放行。
func (m *Middleware) Optional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if claims, ok := m.resolve(r); ok {
			r = r.WithContext(context.WithValue(r.Context(), userKey, claims))
		}
		next.ServeHTTP(w, r)
	})
}

// Require 无 token 或 token 无效时 401。
func (m *Middleware) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := m.resolve(r)
		if !ok {
			if _, has := extractToken(r); has {
				writeErr(w, "登录已过期，请重新登录")
			} else {
				writeErr(w, "未认证")
			}
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, claims)))
	})
}

// RequireCap 要求具备指定权限，否则 403。
//
// 自身完成身份解析，不依赖调用方先挂 Require：否则漏挂时上下文里没有
// 用户，Can 恒为 false，表现为「谁都 403」这种极难定位的故障。
func (m *Middleware) RequireCap(c Capability, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFromContext(r.Context()) == nil {
			claims, ok := m.resolve(r)
			if !ok {
				writeErr(w, "登录已过期，请重新登录")
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), userKey, claims))
		}
		if !Can(r.Context(), c) {
			// 带上角色与所需权限点：笼统的「没有权限」无法定位问题，
			// 而这里最常见的成因就是角色不符或 token 已失效。
			writeErrStatus(w, http.StatusForbidden,
				fmt.Sprintf("当前角色「%s」缺少 %s 权限", RoleFromContext(r.Context()), c))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeErrStatus(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
