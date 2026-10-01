package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"narratpage/internal/auth"
	"narratpage/internal/models"
	"narratpage/internal/repo"
)

// ---------- 访问密码 ----------
//
// 早期实现把「?unlocked 查询参数」当作已解锁凭据：任何人在 URL 后
// 追加 ?unlocked 即可读到全文，密码从未被校验；密码本身还明文存库。
// 这里改为 WordPress 的做法：
//
//  1. 密码以 bcrypt 存储；
//  2. 校验通过后签发 HMAC 签名、带有效期的解锁凭据，
//     写入 HttpOnly Cookie；
//  3. 每次取正文都在服务端校验该凭据，客户端无法伪造。

const (
	postPassCookiePrefix = "np_postpass_"
	// 解锁凭据有效期。够读完一篇长文，又不至于长期留在浏览器里。
	postPassTTL = 30 * time.Minute
)

// hashPostPassword 生成密码存储形式。
func hashPostPassword(plain string) (string, error) {
	return auth.HashPassword(strings.TrimSpace(plain))
}

// verifyPostPassword 校验访问密码。
// needsRehash 表示库中仍是历史明文，调用方应择机改写为哈希。
func verifyPostPassword(stored, plain string) (ok bool, needsRehash bool) {
	stored = strings.TrimSpace(stored)
	plain = strings.TrimSpace(plain)
	if stored == "" || plain == "" {
		return false, false
	}
	if strings.HasPrefix(stored, "$2a$") || strings.HasPrefix(stored, "$2b$") ||
		strings.HasPrefix(stored, "$2y$") {
		return auth.CheckPassword(plain, stored), false
	}
	// 历史明文：常量时间比较，避免时序侧信道
	if len(stored) == len(plain) &&
		subtle.ConstantTimeCompare([]byte(stored), []byte(plain)) == 1 {
		return true, true
	}
	return false, false
}

func postPassCookieName(postID int64) string {
	return postPassCookiePrefix + strconv.FormatInt(postID, 10)
}

// signPostPass 签发解锁凭据：<到期秒>.<HMAC 签名>。
func signPostPass(secret string, postID int64, expires time.Time) string {
	payload := fmt.Sprintf("%d|%d", postID, expires.Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return fmt.Sprintf("%d.%s", expires.Unix(),
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil)))
}

// verifyPostPass 校验解锁凭据的签名与有效期。
func verifyPostPass(secret string, postID int64, value string) bool {
	expStr, sig, ok := strings.Cut(value, ".")
	if !ok {
		return false
	}
	expUnix, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > expUnix {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d|%d", postID, expUnix)))
	want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(sig), []byte(want)) == 1
}

// postPassGranted 判断当前请求是否可以读到受保护正文。
// 管理员/编辑与作者本人无需密码（与 WordPress 一致），
// 其余访客必须在有效期内持有一枚服务端签发的解锁凭据。
func postPassGranted(r *http.Request, post *models.Post, v repo.Viewer, secret string) bool {
	if v.IsAdmin || (post.AuthorID != nil && v.UserID == *post.AuthorID) {
		return true
	}
	c, err := r.Cookie(postPassCookieName(post.ID))
	if err != nil {
		return false
	}
	return verifyPostPass(secret, post.ID, c.Value)
}

// setPostPassCookie 下发解锁凭据。
func setPostPassCookie(w http.ResponseWriter, r *http.Request, secret string, postID int64) {
	http.SetCookie(w, &http.Cookie{
		Name:   postPassCookieName(postID),
		Value:  signPostPass(secret, postID, time.Now().Add(postPassTTL)),
		Path:   "/",
		MaxAge: int(postPassTTL.Seconds()),
		// HttpOnly：前端 JS 读不到，避免被 XSS 读走；解锁状态由服务端判定
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
	})
}
