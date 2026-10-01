package api

import (
	"net/http"
	"strconv"
	"strings"

	"narratpage/internal/httpx"
)

// unlockPost POST /api/posts/{slug}/unlock
//
// 校验访问密码，成功后下发 HttpOnly 解锁凭据。
// 这是访问密码的唯一入口：正文本身不再依赖任何客户端可伪造的标记。
func unlockPost(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("slug")
		viewer := viewerFrom(r)

		// 定位文章：受可见性约束，私密/草稿不会因解锁而暴露
		var (
			postID int64
			stored string
		)
		var err error
		if id, convErr := strconv.ParseInt(key, 10, 64); convErr == nil {
			var postIDTmp int64
			postIDTmp, stored, err = deps.Posts.PostPassword(id)
			postID = postIDTmp
		} else {
			postID, stored, err = deps.Posts.PostPasswordBySlug(key, viewer)
		}
		if err != nil {
			notFound(w, "文章不存在")
			return
		}

		// 未设密码的文章无需解锁
		if strings.TrimSpace(stored) == "" {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "needed": false})
			return
		}

		var body struct {
			Password string `json:"password"`
		}
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}

		ok, needsRehash := verifyPostPassword(stored, body.Password)
		if !ok {
			// 不区分「文章不存在」与「密码错误」以外的信息；
			// 消息保持含糊，避免被用来探测哪些 slug 受保护。
			httpx.WriteError(w, http.StatusUnauthorized, "访问密码不正确")
			return
		}
		// 历史明文密码：验证通过后顺手升级为哈希
		if needsRehash {
			_ = deps.Posts.SetHashedPassword(postID, body.Password)
		}

		setPostPassCookie(w, r, deps.JWTSecret, postID)
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	}
}