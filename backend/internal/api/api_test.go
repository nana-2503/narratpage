package api_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"narratpage/internal/api"
	"narratpage/internal/config"
	"narratpage/internal/db"
	"narratpage/internal/seed"
)

var (
	base       string // API 根地址
	adminToken string

	sharedDB   *sql.DB
	sharedDir  string
	sharedCfg  config.Config
	testServer *httptest.Server
)

// 1x1 透明 PNG
var png1x1 = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "blog-test-")
	if err != nil {
		panic(err)
	}
	sharedDir = tmp
	sharedCfg = config.Config{
		Port:          "0",
		DataDir:       tmp,
		JWTSecret:     "test-secret",
		JWTExpiry:     7 * 24 * time.Hour,
		AdminUsername: "admin",
		AdminPassword: "test-password-123",
		SiteURL:       "http://localhost:8080",
		DBType:        "sqlite",
	}
	sharedDB, err = db.Open(sharedCfg)
	if err != nil {
		panic(err)
	}
	if err := seed.Run(sharedDB, sharedCfg); err != nil {
		panic(err)
	}
	uploads := filepath.Join(tmp, "uploads")
	os.MkdirAll(uploads, 0o755)
	srv := httptest.NewServer(api.NewRouter(sharedCfg, api.Deps{DB: sharedDB, UploadDir: uploads}))
	base = srv.URL + "/api"
	testServer = srv

	code := m.Run()
	srv.Close()
	sharedDB.Close()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// ---------- 请求辅助 ----------

// doJSON 发送 JSON 请求；auth=true 时带管理员 token，字符串 body 原样发送
func doJSON(t *testing.T, method, path string, body any, auth bool) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		if s, ok := body.(string); ok {
			reader = strings.NewReader(s)
		} else {
			b, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			reader = bytes.NewReader(b)
		}
	}
	req, err := http.NewRequest(method, base+path, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("content-type", "application/json")
	}
	if auth {
		req.Header.Set("authorization", "Bearer "+adminToken)
	}
	res, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := map[string]any{}
	if len(raw) > 0 && strings.HasPrefix(res.Header.Get("content-type"), "application/json") {
		_ = json.Unmarshal(raw, &out)
	}
	return res.StatusCode, out
}

// doRaw 发送非 JSON 请求，返回状态码与响应体
func doRaw(t *testing.T, method, path string, body io.Reader, auth bool) (int, []byte, http.Header) {
	t.Helper()
	req, err := http.NewRequest(method, base+path, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if auth {
		req.Header.Set("authorization", "Bearer "+adminToken)
	}
	res, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return res.StatusCode, raw, res.Header
}

// doUpload multipart 上传
func doUpload(t *testing.T, filename string, content []byte, mime string, auth bool) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("form file: %v", err)
	}
	fw.Write(content)
	if err := mw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	req, err := http.NewRequest("POST", base+"/uploads", &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("content-type", mw.FormDataContentType())
	_ = mime
	if auth {
		req.Header.Set("authorization", "Bearer "+adminToken)
	}
	res, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return res.StatusCode, out
}

func firstError(m map[string]any) string {
	if e, ok := m["error"].(string); ok {
		return e
	}
	return ""
}

// ---------- 基础 ----------

func TestHealthAnd404(t *testing.T) {
	if code, body := doJSON(t, "GET", "/health", nil, false); code != 200 || body["status"] != "ok" {
		t.Fatalf("health: %d %v", code, body)
	}
	if code, _ := doJSON(t, "GET", "/no-such-route", nil, false); code != 404 {
		t.Fatalf("404: %d", code)
	}
	// 畸形 JSON → 400
	code, _ := doJSON(t, "POST", "/auth/login", "{broken json", false)
	if code != 400 {
		t.Fatalf("malformed json: %d", code)
	}
}

func TestAuth(t *testing.T) {
	if code, _ := doJSON(t, "GET", "/auth/me", nil, false); code != 401 {
		t.Fatalf("no token: %d", code)
	}
	if code, _ := doJSON(t, "GET", "/auth/me", nil, false); code != 401 {
		t.Fatalf("no token 2: %d", code)
	}
	// 错误 token
	req, _ := http.NewRequest("GET", base+"/auth/me", nil)
	req.Header.Set("authorization", "Bearer not-a-valid-jwt")
	res, _ := (&http.Client{}).Do(req)
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("bad token: %d", res.StatusCode)
	}
	// 错误密码 / 缺字段
	if code, _ := doJSON(t, "POST", "/auth/login", map[string]string{"username": "admin", "password": "wrong"}, false); code != 401 {
		t.Fatalf("wrong password: %d", code)
	}
	if code, _ := doJSON(t, "POST", "/auth/login", map[string]string{"username": "admin"}, false); code != 400 {
		t.Fatalf("missing field: %d", code)
	}
	// 正确登录
	code, body := doJSON(t, "POST", "/auth/login", map[string]string{"username": "admin", "password": "test-password-123"}, false)
	if code != 200 || body["token"] == nil || body["username"] != "admin" {
		t.Fatalf("login: %d %v", code, body)
	}
	adminToken, _ = body["token"].(string)
	// 携带有效 token
	if code, me := doJSON(t, "GET", "/auth/me", nil, true); code != 200 || me["username"] != "admin" {
		t.Fatalf("me: %d %v", code, me)
	}
	// 未登录写文章 401
	if code, _ := doJSON(t, "POST", "/posts", map[string]string{"title": "x"}, false); code != 401 {
		t.Fatalf("create without auth: %d", code)
	}
}

// ---------- 文章 ----------

func TestPosts(t *testing.T) {
	// 游客列表只含已发布
	code, list := doJSON(t, "GET", "/posts?pageSize=50", nil, false)
	if code != 200 {
		t.Fatalf("list: %d", code)
	}
	items, _ := list["items"].([]any)
	if len(items) < 3 {
		t.Fatalf("seed posts: %v", list)
	}
	for _, it := range items {
		if m, _ := it.(map[string]any); m["status"] != "published" {
			t.Fatalf("guest sees non-published: %v", m)
		}
	}
	// 管理员 status=all 可见草稿
	_, adminList := doJSON(t, "GET", "/posts?status=all&pageSize=50", nil, true)
	if toInt(adminList["total"]) < toInt(list["total"]) {
		t.Fatalf("admin total < guest total")
	}

	// 创建：缺标题 400 / 分类不存在 400 / 成功且 slug 自动生成
	if code, _ := doJSON(t, "POST", "/posts", map[string]any{"content": "正文"}, true); code != 400 {
		t.Fatalf("create no title: %d", code)
	}
	if code, body := doJSON(t, "POST", "/posts", map[string]any{"title": "测试文章", "content": "正文", "category_id": 99999}, true); code != 400 || !strings.Contains(firstError(body), "分类") {
		t.Fatalf("bad category: %d %v", code, body)
	}
	code, created := doJSON(t, "POST", "/posts", map[string]any{"title": "Hello World 测试", "content": "正文"}, true)
	if code != 201 || created["slug"] != "hello-world-测试" {
		t.Fatalf("create: %d %v", code, created)
	}
	// 重复 slug 自动追加后缀
	_, dup := doJSON(t, "POST", "/posts", map[string]any{"title": "Hello World 测试", "content": "正文2"}, true)
	if dup["slug"] == "hello-world-测试" {
		t.Fatalf("slug not uniquified: %v", dup)
	}

	// 搜索转义：q=% 不匹配全部
	_, all := doJSON(t, "GET", "/posts?pageSize=50", nil, false)
	code, percent := doJSON(t, "GET", "/posts?q=%25&pageSize=50", nil, false)
	if code != 200 || toInt(percent["total"]) != 0 || toInt(all["total"]) == 0 {
		t.Fatalf("like escape: %d %v", code, percent)
	}

	// 草稿对外不可见
	_, draft := doJSON(t, "POST", "/posts", map[string]any{"title": "Only Draft"}, true)
	draftSlug := draft["slug"].(string)
	if code, _ := doJSON(t, "GET", "/posts/"+draftSlug, nil, false); code != 404 {
		t.Fatalf("guest draft: %d", code)
	}
	if code, body := doJSON(t, "GET", "/posts/"+draftSlug, nil, true); code != 200 || body["title"] != "Only Draft" {
		t.Fatalf("owner draft: %d %v", code, body)
	}

	// /posts/id/:id 读正文；游客 401
	_, byID := doJSON(t, "POST", "/posts", map[string]any{"title": "ById Draft", "content": "秘密内容"}, true)
	id := toInt(byID["id"])
	code, detail := doJSON(t, "GET", fmt.Sprintf("/posts/id/%d", id), nil, true)
	if code != 200 || detail["content"] != "秘密内容" {
		t.Fatalf("by id: %d %v", code, detail)
	}
	if code, _ := doJSON(t, "GET", fmt.Sprintf("/posts/id/%d", id), nil, false); code != 401 {
		t.Fatalf("by id guest: %d", code)
	}

	// 阅读数自增
	_, c1 := doJSON(t, "POST", "/posts", map[string]any{"title": "View Counter", "content": "x", "status": "published"}, true)
	slug := c1["slug"].(string)
	_, v1 := doJSON(t, "GET", "/posts/"+slug, nil, false)
	_, v2 := doJSON(t, "GET", "/posts/"+slug, nil, false)
	if toInt(v2["views"]) != toInt(v1["views"])+1 {
		t.Fatalf("views: %v %v", v1["views"], v2["views"])
	}

	// 部分更新：仅改摘要；发布时间语义
	_, upd := doJSON(t, "POST", "/posts", map[string]any{"title": "To Update", "content": "v1"}, true)
	uid := toInt(upd["id"])
	if code, _ := doJSON(t, "PUT", fmt.Sprintf("/posts/%d", uid), map[string]any{"summary": "新摘要"}, true); code != 200 {
		t.Fatalf("update: %d", code)
	}
	_, d2 := doJSON(t, "GET", fmt.Sprintf("/posts/id/%d", uid), nil, true)
	if d2["summary"] != "新摘要" || d2["title"] != "To Update" || d2["published_at"] != nil {
		t.Fatalf("partial update: %v", d2)
	}
	if code, _ := doJSON(t, "PUT", fmt.Sprintf("/posts/%d", uid), map[string]any{"status": "published"}, true); code != 200 {
		t.Fatalf("publish: %d", code)
	}
	_, d3 := doJSON(t, "GET", fmt.Sprintf("/posts/id/%d", uid), nil, true)
	if d3["published_at"] == nil || d3["published_at"] == "" {
		t.Fatalf("published_at not set: %v", d3)
	}
	if code, _ := doJSON(t, "PUT", "/posts/999999", map[string]any{"title": "x"}, true); code != 404 {
		t.Fatalf("update missing: %d", code)
	}

	// 删除幂等 404
	code, _ = doJSON(t, "DELETE", fmt.Sprintf("/posts/%d", uid), nil, true)
	if code != 200 {
		t.Fatalf("delete: %d", code)
	}
	if code, _ := doJSON(t, "DELETE", fmt.Sprintf("/posts/%d", uid), nil, true); code != 404 {
		t.Fatalf("delete twice: %d", code)
	}
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

// ---------- 分类 ----------

func TestCategories(t *testing.T) {
	code, created := doJSON(t, "POST", "/categories", map[string]string{"name": "测试分类"}, true)
	if code != 201 || created["slug"] != "测试分类" {
		t.Fatalf("create: %d %v", code, created)
	}
	id := toInt(created["id"])
	if code, _ := doJSON(t, "POST", "/categories", map[string]string{"name": "测试分类"}, true); code != 409 {
		t.Fatalf("dup slug: %d", code)
	}
	// 改名保留 slug
	if code, _ := doJSON(t, "PUT", fmt.Sprintf("/categories/%d", id), map[string]string{"name": "分类新名"}, true); code != 200 {
		t.Fatalf("rename: %d", code)
	}
	if code, _ := doJSON(t, "POST", "/categories", map[string]string{"name": "测试分类"}, true); code != 409 {
		t.Fatalf("slug still taken: %d", code)
	}
	// 显式换 slug 后释放
	if code, _ := doJSON(t, "PUT", fmt.Sprintf("/categories/%d", id), map[string]string{"name": "分类新名", "slug": "cat-renamed"}, true); code != 200 {
		t.Fatalf("reslug: %d", code)
	}
	if code, _ := doJSON(t, "POST", "/categories", map[string]string{"name": "测试分类"}, true); code != 201 {
		t.Fatalf("recreate: %d", code)
	}
	// 删除幂等
	if code, _ := doJSON(t, "DELETE", fmt.Sprintf("/categories/%d", id), nil, true); code != 200 {
		t.Fatalf("delete: %d", code)
	}
	if code, _ := doJSON(t, "DELETE", fmt.Sprintf("/categories/%d", id), nil, true); code != 404 {
		t.Fatalf("delete twice: %d", code)
	}
	// 游客写操作 401
	if code, _ := doJSON(t, "POST", "/categories", map[string]string{"name": "x"}, false); code != 401 {
		t.Fatalf("guest create: %d", code)
	}

	// 删除分类后文章变未分类
	_, cat := doJSON(t, "POST", "/categories", map[string]string{"name": "临时分类"}, true)
	cid := toInt(cat["id"])
	_, post := doJSON(t, "POST", "/posts", map[string]any{"title": "Cat Post", "category_id": cid}, true)
	pid := toInt(post["id"])
	doJSON(t, "DELETE", fmt.Sprintf("/categories/%d", cid), nil, true)
	_, pd := doJSON(t, "GET", fmt.Sprintf("/posts/id/%d", pid), nil, true)
	if pd["category_id"] != nil || pd["category_name"] != nil {
		t.Fatalf("post not uncategorized: %v", pd)
	}
}

// ---------- 评论 ----------

func TestComments(t *testing.T) {
	_, post := doJSON(t, "POST", "/posts", map[string]any{"title": "Comment Target"}, true)
	postID := toInt(post["id"])

	code, created := doJSON(t, "POST", "/comments", map[string]any{"postId": postID, "author": "读者", "content": "待审核的评论"}, false)
	if code != 201 || created["status"] != "pending" {
		t.Fatalf("create: %d %v", code, created)
	}
	commentID := toInt(created["id"])

	// 游客默认只看已通过
	_, list := doJSON(t, "GET", fmt.Sprintf("/comments?postId=%d", postID), nil, false)
	items, _ := list["items"].([]any)
	if len(items) != 0 {
		t.Fatalf("guest sees pending: %v", list)
	}

	// 校验
	if code, _ := doJSON(t, "POST", "/comments", map[string]any{"postId": postID, "author": "", "content": "x"}, false); code != 400 {
		t.Fatalf("empty author: %d", code)
	}
	if code, _ := doJSON(t, "POST", "/comments", map[string]any{"postId": postID, "author": strings.Repeat("x", 31), "content": "x"}, false); code != 400 {
		t.Fatalf("long author: %d", code)
	}
	if code, _ := doJSON(t, "POST", "/comments", map[string]any{"postId": 999999, "author": "a", "content": "b"}, false); code != 400 {
		t.Fatalf("missing post: %d", code)
	}

	// 管理员按状态筛选，联表返回文章标题
	_, pending := doJSON(t, "GET", fmt.Sprintf("/comments?postId=%d&status=pending", postID), nil, true)
	pitems, _ := pending["items"].([]any)
	if len(pitems) != 1 {
		t.Fatalf("pending list: %v", pending)
	}
	if m, _ := pitems[0].(map[string]any); m["post_title"] != "Comment Target" {
		t.Fatalf("post_title: %v", m)
	}

	// 审核通过后游客可见
	if code, _ := doJSON(t, "PUT", fmt.Sprintf("/comments/%d", commentID), map[string]string{"status": "approved"}, true); code != 200 {
		t.Fatalf("moderate: %d", code)
	}
	_, after := doJSON(t, "GET", fmt.Sprintf("/comments?postId=%d", postID), nil, false)
	aitems, _ := after["items"].([]any)
	if len(aitems) != 1 {
		t.Fatalf("approved visible: %v", after)
	}

	// 游客不能审核/删除
	if code, _ := doJSON(t, "PUT", fmt.Sprintf("/comments/%d", commentID), map[string]string{"status": "rejected"}, false); code != 401 {
		t.Fatalf("guest moderate: %d", code)
	}
	if code, _ := doJSON(t, "DELETE", fmt.Sprintf("/comments/%d", commentID), nil, false); code != 401 {
		t.Fatalf("guest delete: %d", code)
	}
	// 非法状态 400
	if code, _ := doJSON(t, "PUT", fmt.Sprintf("/comments/%d", commentID), map[string]string{"status": "deleted"}, true); code != 400 {
		t.Fatalf("bad status: %d", code)
	}
	// 删除
	if code, _ := doJSON(t, "DELETE", fmt.Sprintf("/comments/%d", commentID), nil, true); code != 200 {
		t.Fatalf("delete: %d", code)
	}
}

// ---------- RSS ----------

func TestRSS(t *testing.T) {
	code, raw, header := doRaw(t, "GET", "/rss.xml", nil, false)
	if code != 200 || !strings.Contains(header.Get("content-type"), "rss+xml") {
		t.Fatalf("rss: %d %s", code, header.Get("content-type"))
	}
	xml := string(raw)
	for _, want := range []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<rss version="2.0">`,
		"<channel>",
		"叙页博客系统",
		"<lastBuildDate>",
		"为什么选择 SQLite 作为博客数据库",
	} {
		if !strings.Contains(xml, want) {
			t.Fatalf("rss missing %q", want)
		}
	}
	// 标题中的 XML 特殊字符必须转义
	doJSON(t, "POST", "/posts", map[string]any{"title": `XML <b>转义</b> "引号" & 符号`, "status": "published"}, true)
	code, raw2, _ := doRaw(t, "GET", "/rss.xml", nil, false)
	xml2 := string(raw2)
	if !strings.Contains(xml2, "XML &lt;b&gt;转义&lt;/b&gt;") {
		t.Fatalf("rss escape: %s", xml2)
	}
	if strings.Contains(xml2, "<b>转义</b>") {
		t.Fatalf("rss unescaped: %s", xml2)
	}
}

// ---------- 上下篇 ----------

func TestNeighbors(t *testing.T) {
	var slugs []string
	for _, title := range []string{"邻居文章甲", "邻居文章乙", "邻居文章丙"} {
		_, created := doJSON(t, "POST", "/posts", map[string]any{"title": title, "status": "published"}, true)
		slugs = append(slugs, created["slug"].(string))
	}
	_, mid := doJSON(t, "GET", "/posts/neighbors/"+slugs[1], nil, true)
	prev, _ := mid["prev"].(map[string]any)
	next, _ := mid["next"].(map[string]any)
	if prev["slug"] != slugs[2] || next["slug"] != slugs[0] {
		t.Fatalf("neighbors: %v %v", prev, next)
	}
	// 游客可访问；404 场景
	if code, _ := doJSON(t, "GET", "/posts/neighbors/"+slugs[1], nil, false); code != 200 {
		t.Fatalf("guest neighbors: %d", code)
	}
	if code, _ := doJSON(t, "GET", "/posts/neighbors/no-such-post", nil, false); code != 404 {
		t.Fatalf("missing: %d", code)
	}
	_, draft := doJSON(t, "POST", "/posts", map[string]any{"title": "草稿邻居"}, true)
	if code, _ := doJSON(t, "GET", "/posts/neighbors/"+draft["slug"].(string), nil, false); code != 404 {
		t.Fatalf("draft neighbors: %d", code)
	}
	// neighbors 与列表排序互为一致（同秒发布 id 兜底）
	_, list := doJSON(t, "GET", "/posts?pageSize=50", nil, false)
	order := []string{}
	for _, it := range mustItems(t, list) {
		order = append(order, it["slug"].(string))
	}
	for _, s := range []string{"why-sqlite-for-blog", "markdown-writing-pipeline", "restart-blogging"} {
		idx := indexOf(order, s)
		if idx < 0 {
			t.Fatalf("%s not in list", s)
		}
		_, nb := doJSON(t, "GET", "/posts/neighbors/"+s, nil, false)
		wantPrev := any(nil)
		wantNext := any(nil)
		if idx > 0 {
			wantPrev = order[idx-1]
		}
		if idx < len(order)-1 {
			wantNext = order[idx+1]
		}
		if slugOrNil(nb["prev"]) != wantPrev || slugOrNil(nb["next"]) != wantNext {
			t.Fatalf("consistency %s: prev=%v next=%v want %v %v", s, nb["prev"], nb["next"], wantPrev, wantNext)
		}
	}
}

func mustItems(t *testing.T, m map[string]any) []map[string]any {
	t.Helper()
	raw, _ := m["items"].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, it := range raw {
		out = append(out, it.(map[string]any))
	}
	return out
}

func slugOrNil(v any) any {
	if m, ok := v.(map[string]any); ok {
		return m["slug"]
	}
	return nil
}

func indexOf(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}

// ---------- 修改密码 ----------

func TestChangePassword(t *testing.T) {
	if code, _ := doJSON(t, "POST", "/auth/password", map[string]string{"oldPassword": "x", "newPassword": "y"}, false); code != 401 {
		t.Fatalf("no auth: %d", code)
	}
	if code, _ := doJSON(t, "POST", "/auth/password", map[string]string{"oldPassword": "wrong-old", "newPassword": "newpass123"}, true); code != 401 {
		t.Fatalf("wrong old: %d", code)
	}
	if code, _ := doJSON(t, "POST", "/auth/password", map[string]string{"oldPassword": "test-password-123", "newPassword": "short"}, true); code != 400 {
		t.Fatalf("short: %d", code)
	}
	if code, _ := doJSON(t, "POST", "/auth/password", map[string]string{"oldPassword": "test-password-123", "newPassword": "brand-new-pass-456"}, true); code != 200 {
		t.Fatalf("change: %d", code)
	}
	if code, body := doJSON(t, "POST", "/auth/login", map[string]string{"username": "admin", "password": "brand-new-pass-456"}, false); code != 200 || body["token"] == nil {
		t.Fatalf("login new: %d %v", code, body)
	}
	if code, _ := doJSON(t, "POST", "/auth/login", map[string]string{"username": "admin", "password": "test-password-123"}, false); code != 401 {
		t.Fatalf("old password still works: %d", code)
	}
}

// ---------- 上传 ----------

func TestUpload(t *testing.T) {
	// 游客 401
	if code, _ := doUpload(t, "dot.png", png1x1, "image/png", false); code != 401 {
		t.Fatalf("guest upload: %d", code)
	}
	// 管理员上传
	code, created := doUpload(t, "dot.png", png1x1, "image/png", true)
	if code != 201 {
		t.Fatalf("upload: %d", code)
	}
	url, _ := created["url"].(string)
	if !strings.HasPrefix(url, "/api/uploads/") || !strings.HasSuffix(url, ".png") {
		t.Fatalf("url: %s", url)
	}
	if len(filepath.Base(url)) != 16+len(".png") {
		t.Fatalf("random name: %s", url)
	}
	// 匿名可读且字节一致（url 已含 /api 前缀，直接对服务器根地址请求）
	imgRes, err := http.Get(testServer.URL + url)
	if err != nil {
		t.Fatalf("download request: %v", err)
	}
	raw, _ := io.ReadAll(imgRes.Body)
	imgRes.Body.Close()
	if imgRes.StatusCode != 200 || !bytes.Equal(raw, png1x1) {
		t.Fatalf("download: %d %d bytes", imgRes.StatusCode, len(raw))
	}
	// 声明 image/png 但内容不是图片 → 400
	if code, body := doUpload(t, "fake.png", []byte("<html>not an image</html>"), "image/png", true); code != 400 || !strings.Contains(firstError(body), "图片") {
		t.Fatalf("fake content: %d %v", code, body)
	}
	// 超过 5MB → 413
	if code, _ := doUpload(t, "big.png", bytes.Repeat([]byte{1}, 6*1024*1024), "image/png", true); code != 413 {
		t.Fatalf("too large: %d", code)
	}
}

// ---------- 限流 ----------

func TestLoginRateLimit(t *testing.T) {
	// 独立实例，避免与其他用例共享计数器
	srv := httptest.NewServer(api.NewRouter(sharedCfg, api.Deps{DB: sharedDB, UploadDir: filepath.Join(sharedDir, "uploads")}))
	defer srv.Close()
	old := base
	base = srv.URL + "/api"
	defer func() { base = old }()

	var last int
	for i := 0; i < 11; i++ {
		last, _ = doJSON(t, "POST", "/auth/login", map[string]string{"username": "admin", "password": "wrong"}, false)
	}
	if last != 429 {
		t.Fatalf("rate limit: %d", last)
	}
}
