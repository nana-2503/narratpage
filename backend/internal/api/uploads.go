package api

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"narratpage/internal/auth"
	"narratpage/internal/httpx"
	"narratpage/internal/models"
)

const maxUploadBytes = 10 * 1024 * 1024 // 10MB

// extByMime 允许的类型（以嗅探内容为准，不信任客户端声明的 MIME）
var extByMime = map[string]string{
	"image/png":       ".png",
	"image/jpeg":      ".jpg",
	"image/webp":      ".webp",
	"image/gif":       ".gif",
	"image/svg+xml":   ".svg",
	"application/pdf": ".pdf",
}

var mimeByExt = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".webp": "image/webp",
	".gif":  "image/gif",
	".svg":  "image/svg+xml",
	".pdf":  "application/pdf",
}

// uploadImage POST /api/uploads
//
// 安全要点：
//   - 类型判定读文件头 magic bytes，不信任客户端声明；
//   - 文件名随机生成，杜绝路径穿越与覆盖；
//   - SVG 属可执行内容，单独处理（见 sanitizeSVG）。
func uploadImage(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := auth.UserFromContext(r.Context())
		if claims == nil {
			httpx.WriteError(w, http.StatusUnauthorized, "未认证")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			if strings.Contains(err.Error(), "request body too large") {
				httpx.WriteError(w, http.StatusRequestEntityTooLarge, "文件大小不能超过 10MB")
				return
			}
			badRequest(w, "上传失败")
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			badRequest(w, "未收到文件")
			return
		}
		defer file.Close()

		head := make([]byte, 512)
		n, _ := io.ReadFull(file, head)
		head = head[:n]
		sniffed := http.DetectContentType(head)
		if ext, ok := extByMime[sniffed]; ok {
			storeUpload(w, deps, file, head, ext, claims.Sub)
			return
		}
		// DetectContentType 对 SVG 返回 text/xml 或 text/plain，需按内容判定
		if looksLikeSVG(head) {
			if !sanitizeSVG(head) {
				badRequest(w, "SVG 含不允许的脚本内容")
				return
			}
			storeUpload(w, deps, file, head, ".svg", claims.Sub)
			return
		}
		badRequest(w, "仅支持 PNG / JPEG / WebP / GIF / SVG / PDF")
	}
}

// storeUpload 落盘并登记媒体记录。
func storeUpload(w http.ResponseWriter, deps Deps, file multipartFile, head []byte,
	ext string, userID int64) {
	name := randHex(12) + ext
	dst := filepath.Join(deps.UploadDir, name)

	// 按月分目录存放，避免单目录文件过多
	dir := filepath.Join(deps.UploadDir, time.Now().Format("2006/01"))
	_ = os.MkdirAll(dir, 0o755)
	dst = filepath.Join(dir, name)
	relName := time.Now().Format("2006/01") + "/" + name

	out, err := os.Create(dst)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
		return
	}
	defer out.Close()

	if _, err := out.Write(head); err == nil {
		if _, err := io.Copy(out, file); err != nil {
			_ = os.Remove(dst)
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
	}

	info, err := out.Stat()
	size := int64(0)
	if err == nil {
		size = info.Size()
	}

	url := "/api/uploads/" + relName
	mime := mimeByExt[ext]
	item := &models.Media{
		Filename: name, URL: url, Mime: mime, Size: size,
		UploadedBy: &userID,
	}
	if id, err := deps.Media.Create(item); err == nil {
		item.ID = id
	}

	httpx.WriteJSON(w, http.StatusCreated, map[string]any{
		"id": item.ID, "url": url, "filename": name,
		"mime": mime, "size": size,
	})
}

// multipartFile 是上传文件的最小接口，便于测试替换。
type multipartFile interface {
	io.Reader
	io.Seeker
}

// looksLikeSVG 通过内容特征判断 SVG。
func looksLikeSVG(head []byte) bool {
	s := strings.ToLower(string(head))
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "<svg") ||
		(strings.HasPrefix(s, "<?xml") && strings.Contains(s, "<svg"))
}

// sanitizeSVG 检查 SVG 是否含脚本或外部引用。
// SVG 可内嵌 <script>，直接对外提供等同于开放 XSS，故做白名单式检查。
func sanitizeSVG(content []byte) bool {
	s := strings.ToLower(string(content))
	dangerous := []string{
		"<script", "javascript:", "onload=", "onerror=", "onclick=",
		"<foreignobject", "<iframe", "<embed", "<object",
		"data:text/html", "<!entity", "<!doctype",
	}
	for _, d := range dangerous {
		if strings.Contains(s, d) {
			return false
		}
	}
	return true
}

// listMedia GET /api/media
func listMedia(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		pageSize, _ := strconv.Atoi(q.Get("pageSize"))
		mime := q.Get("mime")
		switch mime {
		case "image":
			mime = "image/"
		case "pdf":
			mime = "application/pdf"
		}
		result, err := deps.Media.List(page, pageSize, mime, q.Get("q"))
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		count, size := deps.Media.Stats()
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"items": result.Items, "total": result.Total,
			"page": result.Page, "pageSize": result.PageSize,
			"totalPages": result.TotalPages,
			"count":      count, "totalSize": size,
		})
	}
}

// updateMedia PUT /api/media/{id}
func updateMedia(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "文件不存在")
			return
		}
		var body struct {
			Title   *string `json:"title"`
			AltText *string `json:"alt_text"`
			Caption *string `json:"caption"`
		}
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		m, err := deps.Media.ByID(id)
		if err != nil {
			notFound(w, "文件不存在")
			return
		}
		if body.Title != nil {
			if len(*body.Title) > 200 {
				badRequest(w, "标题最长 200 字符")
				return
			}
			m.Title = *body.Title
		}
		if body.AltText != nil {
			if len(*body.AltText) > 255 {
				badRequest(w, "替代文本最长 255 字符")
				return
			}
			m.AltText = *body.AltText
		}
		if body.Caption != nil {
			m.Caption = *body.Caption
		}
		if err := deps.Media.Update(id, m.Title, m.AltText, m.Caption); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		updated, _ := deps.Media.ByID(id)
		httpx.WriteJSON(w, http.StatusOK, updated)
	}
}

// deleteMedia DELETE /api/media/{id}
func deleteMedia(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "文件不存在")
			return
		}
		m, err := deps.Media.ByID(id)
		if err != nil {
			notFound(w, "文件不存在")
			return
		}
		// 被文章引用时保留文件，仅删记录：避免历史文章出现坏图
		used := deps.Media.PostsUsing(m.URL)
		if _, err := deps.Media.Delete(id); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true, "referenced": used,
		})
	}
}

// randHex 生成随机十六进制文件名。
func randHex(n int) string {
	b := make([]byte, n)
	if _, err := randRead(b); err != nil {
		panic("api: 无法读取加密随机源: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// hashFile 计算文件内容哈希（用于去重与校验）。
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
