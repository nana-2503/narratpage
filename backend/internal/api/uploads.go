package api

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"narratpage/internal/auth"
	"narratpage/internal/httpx"
)

const maxUploadBytes = 5 * 1024 * 1024 // 5MB

// extByMime 允许的图片类型（以嗅探内容为准，不信任客户端声明）
var extByMime = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// POST /api/uploads — 上传图片（仅管理员）。
// 类型判定用 http.DetectContentType（读文件头 magic bytes），比信任
// 声明的 MIME 更安全；文件名随机生成，防穿越与覆盖。
func uploadImage(uploadDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if auth.UserFromContext(r.Context()) == nil {
			httpx.WriteError(w, http.StatusUnauthorized, "未认证")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			if strings.Contains(err.Error(), "request body too large") {
				httpx.WriteError(w, http.StatusRequestEntityTooLarge, "图片大小不能超过 5MB")
				return
			}
			httpx.WriteError(w, http.StatusBadRequest, "上传失败")
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "未收到文件")
			return
		}
		defer file.Close()

		head := make([]byte, 512)
		n, _ := io.ReadFull(file, head)
		sniffed := http.DetectContentType(head[:n])
		ext, ok := extByMime[sniffed]
		if !ok {
			httpx.WriteError(w, http.StatusBadRequest, "仅支持 PNG / JPEG / WebP / GIF 图片")
			return
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}

		name := randHex(8) + ext
		dst := filepath.Join(uploadDir, name)
		out, err := os.Create(dst)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		defer out.Close()
		if _, err := io.Copy(out, file); err != nil {
			os.Remove(dst)
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusCreated, map[string]string{"url": "/api/uploads/" + name})
	}
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand 失败属系统级异常
	}
	return hex.EncodeToString(b)
}
