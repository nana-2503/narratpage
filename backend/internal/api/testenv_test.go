package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"narratpage/internal/config"
	"narratpage/internal/db"
	"narratpage/internal/repo"
	"narratpage/internal/seed"
)

// testEnv 是集成测试的共享环境：真实 HTTP handler + 真实数据库。
// 只用 SQLite（无需外部依赖），方言相关行为由 internal/db 的
// 跨库测试单独覆盖。
type testEnv struct {
	base   string
	token  string
	db     *sql.DB
	server *httptest.Server
	dir    string
	cfg    config.Config
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	// 限流器是包级单例，重置以隔离各测试的配额
	resetCommentLimiter()

	dir := t.TempDir()
	cfg := config.Config{
		DataDir:       dir,
		DBType:        "sqlite",
		JWTSecret:     "test-secret",
		JWTExpiry:     time.Hour,
		AdminUsername: "admin",
		AdminPassword: "test-password-123",
		SiteURL:       "http://localhost:8080",
		Port:          "0",
	}
	database, err := db.Open(cfg)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	if err := seed.Run(database, cfg); err != nil {
		t.Fatalf("写入种子数据失败: %v", err)
	}
	options := repo.NewOptions(database, cfg.DBType)
	if err := options.EnsureDefaults(); err != nil {
		t.Fatal(err)
	}
	return newEnvFor(t, cfg, database, options, dir+"/uploads")
}

// newEnvFor 组装 HTTP 测试服务。抽出以便 SQLite 与外部数据库共用。
func newEnvFor(t *testing.T, cfg config.Config, database *sql.DB,
	options *repo.Options, uploadDir string) *testEnv {
	t.Helper()
	if err := ensureDir(uploadDir); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(NewRouter(cfg, Deps{
		DB:        database,
		DBType:    cfg.DBType,
		UploadDir: uploadDir,
		SiteURL:   cfg.SiteURL,
		JWTSecret: cfg.JWTSecret,
		JWTExpiry: cfg.JWTExpiry,
		Posts:     repo.New(database, cfg.DBType),
		Options:   options,
		Users:     repo.NewUsers(database, cfg.DBType),
		Taxonomy:  repo.NewTaxonomy(database, cfg.DBType),
		Media:     repo.NewMedia(database, cfg.DBType),
		PostMeta:  repo.NewPostMeta(database, cfg.DBType),
		Revisions: repo.NewRevisions(database, cfg.DBType),
		Redirects: repo.NewRedirects(database, cfg.DBType),
		Comments:  repo.NewComments(database, cfg.DBType),
	}))

	env := &testEnv{
		base:   srv.URL + "/api",
		db:     database,
		server: srv,
		dir:    uploadDir,
		cfg:    cfg,
	}
	t.Cleanup(func() {
		srv.Close()
		_ = database.Close()
	})
	return env
}

// login 以指定身份登录并保存 token。
func (e *testEnv) login(t *testing.T, username, password string) string {
	t.Helper()
	code, body := e.do(t, "POST", "/auth/login", map[string]any{
		"username": username, "password": password,
	}, false)
	if code != http.StatusOK {
		t.Fatalf("登录失败 (%d): %v", code, body)
	}
	tok, _ := body["token"].(string)
	if tok == "" {
		t.Fatalf("登录未返回 token: %v", body)
	}
	e.token = tok
	return tok
}

// do 发送 JSON 请求。
func (e *testEnv) do(t *testing.T, method, path string, body any, auth bool) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("序列化失败: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.base+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	if auth {
		if e.token == "" {
			t.Fatal("未登录：先调用 login")
		}
		req.Header.Set("authorization", "Bearer "+e.token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := map[string]any{}
	if len(raw) > 0 && strings.HasPrefix(res.Header.Get("content-type"), "application/json") {
		_ = json.Unmarshal(raw, &out)
	}
	return res.StatusCode, out
}

// url 拼接请求地址。
//
// 约定：以 /api/ 开头的路径视为绝对地址（上传接口返回的 url 即如此），
// 直接挂在 server 上；其余路径视为相对 /api 的简写。
func (e *testEnv) url(path string) string {
	if strings.HasPrefix(path, "/api/") {
		return e.server.URL + path
	}
	return e.base + path
}

// doRaw 发送非 JSON 请求。
func (e *testEnv) doRaw(t *testing.T, method, path string, body io.Reader,
	contentType string, auth bool) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, e.url(path), body)
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("content-type", contentType)
	}
	if auth {
		req.Header.Set("authorization", "Bearer "+e.token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

// upload 上传一个文件并返回响应体。
func (e *testEnv) upload(t *testing.T, filename, contentType string, data []byte, auth bool) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	res := e.doRaw(t, "POST", "/uploads", &buf, w.FormDataContentType(), auth)
	out := map[string]any{}
	_ = json.NewDecoder(res.Body).Decode(&out)
	return res.StatusCode, out
}

// 1x1 透明 PNG
var png1x1 = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

func ensureDir(path string) error { return os.MkdirAll(path, 0o755) }
