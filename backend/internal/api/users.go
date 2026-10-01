package api

import (
	"net/http"
	"strconv"
	"strings"

	"narratpage/internal/auth"
	"narratpage/internal/httpx"
	"narratpage/internal/models"
)

// currentUser GET /api/users/me
func currentUser(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := auth.UserFromContext(r.Context())
		user, err := deps.Users.ByID(claims.Sub)
		if err != nil {
			httpx.WriteError(w, http.StatusNotFound, "用户不存在")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, user)
	}
}

// updateCurrentUser PUT /api/users/me
func updateCurrentUser(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := auth.UserFromContext(r.Context())
		user, err := deps.Users.ByID(claims.Sub)
		if err != nil {
			httpx.WriteError(w, http.StatusNotFound, "用户不存在")
			return
		}
		var body struct {
			DisplayName *string `json:"display_name"`
			Email       *string `json:"email"`
			Bio         *string `json:"bio"`
			AvatarURL   *string `json:"avatar_url"`
		}
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		if body.DisplayName != nil {
			trimmed := strings.TrimSpace(*body.DisplayName)
			if len(trimmed) > 64 {
				badRequest(w, "显示名最长 64 字符")
				return
			}
			user.DisplayName = trimmed
		}
		if body.Email != nil {
			email := strings.TrimSpace(*body.Email)
			if email != "" && !strings.Contains(email, "@") {
				badRequest(w, "邮箱格式不正确")
				return
			}
			user.Email = email
		}
		if body.Bio != nil {
			if len(*body.Bio) > 2000 {
				badRequest(w, "简介最长 2000 字符")
				return
			}
			user.Bio = *body.Bio
		}
		if body.AvatarURL != nil {
			avatar := strings.TrimSpace(*body.AvatarURL)
			if len(avatar) > 512 {
				badRequest(w, "头像地址过长")
				return
			}
			user.AvatarURL = avatar
		}
		// 角色与启用状态不在此处修改：自助改资料不应有提权路径。
		if err := deps.Users.UpdateProfile(user.ID, user); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		updated, _ := deps.Users.ByID(user.ID)
		httpx.WriteJSON(w, http.StatusOK, updated)
	}
}

// listUsers GET /api/users
func listUsers(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
		result, err := deps.Users.List(page, pageSize, r.URL.Query().Get("q"))
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, result)
	}
}

type userInput struct {
	Username    *string `json:"username"`
	Email       *string `json:"email"`
	DisplayName *string `json:"display_name"`
	Password    *string `json:"password"`
	Role        *string `json:"role"`
	Bio         *string `json:"bio"`
	AvatarURL   *string `json:"avatar_url"`
	Active      *bool   `json:"active"`
}

// validateUser 校验用户输入。
func (in *userInput) validate(requirePassword bool) string {
	if in.Username != nil {
		u := strings.TrimSpace(*in.Username)
		if len(u) < 3 || len(u) > 64 {
			return "用户名长度需在 3-64 位之间"
		}
		for _, r := range u {
			if !(r == '_' || r == '-' || r == '.' ||
				(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
				return "用户名只能包含字母、数字、点、下划线与连字符"
			}
		}
	}
	if in.Email != nil {
		e := strings.TrimSpace(*in.Email)
		if e != "" && !strings.Contains(e, "@") {
			return "邮箱格式不正确"
		}
	}
	// 注意不能用 RoleOf 判断：它对未知值会降级为最低权限，
	// 那样非法角色会被静默接受。必须直接校验原值。
	if in.Role != nil && !auth.Role(*in.Role).Valid() {
		return "非法角色"
	}
	if in.Password != nil {
		if len(*in.Password) < 8 || len(*in.Password) > 72 {
			return "密码长度需在 8-72 位之间"
		}
	} else if requirePassword {
		return "密码必填"
	}
	if in.Bio != nil && len(*in.Bio) > 2000 {
		return "简介最长 2000 字符"
	}
	return ""
}

// userFromInput 从提交体构造用户对象。
func userFromInput(in *userInput, username string) *models.User {
	u := &models.User{
		Username:    username,
		Email:       strings.TrimSpace(derefStr(in.Email)),
		DisplayName: strings.TrimSpace(derefStr(in.DisplayName)),
		Role:        string(auth.RoleAuthor),
		Bio:         derefStr(in.Bio),
		AvatarURL:   strings.TrimSpace(derefStr(in.AvatarURL)),
		Active:      true,
	}
	if in.Role != nil {
		u.Role = string(auth.RoleOf(*in.Role))
	}
	if in.Active != nil {
		u.Active = *in.Active
	}
	return u
}

// createUser POST /api/users
func createUser(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body userInput
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		if msg := body.validate(true); msg != "" {
			badRequest(w, msg)
			return
		}
		username := strings.TrimSpace(derefStr(body.Username))
		if taken, _ := deps.Users.UsernameExists(username, 0); taken {
			httpx.WriteError(w, http.StatusConflict, "用户名已存在")
			return
		}

		u := userFromInput(&body, username)
		id, err := deps.Users.Create(u, derefStr(body.Password))
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		created, _ := deps.Users.ByID(id)
		httpx.WriteJSON(w, http.StatusCreated, created)
	}
}

// updateUser PUT /api/users/{id}
func updateUser(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "用户不存在")
			return
		}
		current, err := deps.Users.ByID(id)
		if err != nil {
			notFound(w, "用户不存在")
			return
		}
		var body userInput
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		if msg := body.validate(false); msg != "" {
			badRequest(w, msg)
			return
		}

		// 防止把最后一个管理员降级或停用，导致无人可管理站点
		losingAdmin := (body.Role != nil && auth.RoleOf(*body.Role) != auth.RoleAdmin) ||
			(body.Active != nil && !*body.Active)
		if current.Role == string(auth.RoleAdmin) && losingAdmin {
			remaining, _ := deps.Users.CountAdmins(current.ID)
			if remaining == 0 {
				badRequest(w, "至少需要保留一个启用状态的管理员")
				return
			}
		}

		if body.Email != nil {
			current.Email = strings.TrimSpace(*body.Email)
		}
		if body.DisplayName != nil {
			current.DisplayName = strings.TrimSpace(*body.DisplayName)
		}
		if body.Bio != nil {
			current.Bio = *body.Bio
		}
		if body.AvatarURL != nil {
			current.AvatarURL = strings.TrimSpace(*body.AvatarURL)
		}
		if body.Role != nil {
			current.Role = string(auth.RoleOf(*body.Role))
		}
		if body.Active != nil {
			current.Active = *body.Active
		}

		if err := deps.Users.Update(id, current); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		if body.Password != nil {
			if err := deps.Users.UpdatePassword(id, *body.Password); err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
				return
			}
			// 管理员改密后使该用户会话失效，强制重新登录
			_, _ = deps.Users.RevokeAllForUser(id)
		}
		// 停用账号时同时踢下线
		if body.Active != nil && !*body.Active {
			_, _ = deps.Users.RevokeAllForUser(id)
		}

		updated, _ := deps.Users.ByID(id)
		httpx.WriteJSON(w, http.StatusOK, updated)
	}
}

// deleteUser DELETE /api/users/{id}
func deleteUser(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "用户不存在")
			return
		}
		current, err := deps.Users.ByID(id)
		if err != nil {
			notFound(w, "用户不存在")
			return
		}
		if current.Role == string(auth.RoleAdmin) {
			remaining, _ := deps.Users.CountAdmins(current.ID)
			if remaining == 0 {
				badRequest(w, "不能删除最后一个管理员")
				return
			}
		}
		n, err := deps.Users.Delete(id)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		if n == 0 {
			notFound(w, "用户不存在")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
