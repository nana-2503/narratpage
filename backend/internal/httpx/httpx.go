package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// MaxBodyBytes JSON 请求体上限（与 Node 版一致）
const MaxBodyBytes = 256 << 10

// WriteJSON 输出 JSON 响应
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("content-type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError 输出 {"error": msg}
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

// ErrBadJSON 畸形 JSON
var ErrBadJSON = errors.New("请求体不是合法的 JSON")

// DecodeJSON 读取并解码 JSON 请求体。
// 超限返回的错误信息含 "http: request body too large"。
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			WriteError(w, http.StatusRequestEntityTooLarge, "请求体过大")
			return err
		}
		WriteError(w, http.StatusBadRequest, ErrBadJSON.Error())
		return ErrBadJSON
	}
	return nil
}

// QueryInt 解析正整数查询参数，非法/缺失时回退默认值
func QueryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// ReadAll 小工具：读取 body 到内存（上传校验用）
func ReadAll(r io.Reader, limit int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, limit))
}
