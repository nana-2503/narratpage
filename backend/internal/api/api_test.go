package api

import (
	"net/http"
	"strings"
	"testing"
)

// TestHealthAnd404 基础可达性与统一 404。
func TestHealthAnd404(t *testing.T) {
	e := newTestEnv(t)
	e.login(t, "admin", "test-password-123")

	if code, body := e.do(t, "GET", "/health", nil, false); code != 200 ||
		body["status"] != "ok" {
		t.Fatalf("健康检查异常: %d %v", code, body)
	}
	if code, _ := e.do(t, "GET", "/nonexistent", nil, false); code != http.StatusNotFound {
		t.Fatalf("未知接口应返回 404，得到 %d", code)
	}
}

// TestAuth 登录、错误密码、me 接口与密码修改。
func TestAuth(t *testing.T) {
	e := newTestEnv(t)

	if code, _ := e.do(t, "POST", "/auth/login", map[string]any{
		"username": "admin", "password": "wrong-password",
	}, false); code != http.StatusUnauthorized {
		t.Fatalf("错误密码应返回 401，得到 %d", code)
	}
	if code, _ := e.do(t, "POST", "/auth/login", map[string]any{
		"username": "", "password": "",
	}, false); code != http.StatusBadRequest {
		t.Fatalf("空凭据应返回 400，得到 %d", code)
	}

	e.token = e.login(t, "admin", "test-password-123")
	code, body := e.do(t, "GET", "/auth/me", nil, true)
	if code != 200 || body["username"] != "admin" {
		t.Fatalf("me 接口异常: %d %v", code, body)
	}
	// 登录响应应带角色，供前端做权限分支
	_, loginBody := e.do(t, "POST", "/auth/login", map[string]any{
		"username": "admin", "password": "test-password-123",
	}, false)
	if u, ok := loginBody["user"].(map[string]any); !ok || u["role"] != "admin" {
		t.Fatalf("登录应返回带角色的 user 对象: %v", loginBody)
	}

	// 无 token 访问受保护接口
	if code, _ := e.do(t, "GET", "/auth/me", nil, false); code != http.StatusUnauthorized {
		t.Fatalf("无 token 应返回 401，得到 %d", code)
	}

	// 改密
	if code, _ := e.do(t, "POST", "/auth/password", map[string]any{
		"oldPassword": "test-password-123", "newPassword": "new-password-456",
	}, true); code != 200 {
		t.Fatalf("改密失败")
	}
	if code, _ := e.do(t, "POST", "/auth/login", map[string]any{
		"username": "admin", "password": "test-password-123",
	}, false); code != http.StatusUnauthorized {
		t.Fatalf("旧密码应失效，得到 %d", code)
	}
	e.token = e.login(t, "admin", "new-password-456")

	// 密码过短
	if code, _ := e.do(t, "POST", "/auth/password", map[string]any{
		"oldPassword": "new-password-456", "newPassword": "short",
	}, true); code != http.StatusBadRequest {
		t.Fatalf("过短密码应返回 400，得到 %d", code)
	}
}

// TestSessionLogout 登出后 token 立即失效。
//
// 这是新增的会话校验能力：JWT 本身无状态，靠 sessions 表
// 记录活跃会话才能实现「登出即失效」。
func TestSessionLogout(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	if code, _ := e.do(t, "GET", "/auth/me", nil, true); code != 200 {
		t.Fatalf("登录后应可用，得到 %d", code)
	}
	if code, _ := e.do(t, "POST", "/auth/logout", nil, true); code != 200 {
		t.Fatalf("登出失败")
	}
	if code, _ := e.do(t, "GET", "/auth/me", nil, true); code != http.StatusUnauthorized {
		t.Fatalf("登出后 token 应失效，得到 %d", code)
	}
	// 重新登录应拿到新会话
	e.token = e.login(t, "admin", "test-password-123")
	if code, _ := e.do(t, "GET", "/auth/me", nil, true); code != 200 {
		t.Fatalf("重新登录后应可用，得到 %d", code)
	}
}

// TestPostsCRUD 文章全流程：创建 → 读取 → 更新 → 回收站 → 永久删除。
func TestPostsCRUD(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	// 游客看不到草稿
	code, created := e.do(t, "POST", "/posts", map[string]any{
		"title": "测试文章", "content": "正文内容", "status": "draft",
	}, true)
	if code != http.StatusCreated {
		t.Fatalf("创建失败: %d %v", code, created)
	}
	slug, _ := created["slug"].(string)
	if slug == "" {
		t.Fatalf("创建应返回 slug: %v", created)
	}

	if code, _ := e.do(t, "GET", "/posts/"+slug, nil, false); code != http.StatusNotFound {
		t.Fatalf("游客访问草稿应返回 404，得到 %d", code)
	}
	if code, _ := e.do(t, "GET", "/posts/"+slug, nil, true); code != 200 {
		t.Fatalf("管理员访问草稿应成功，得到 %d", code)
	}

	// 发布
	if code, _ := e.do(t, "PUT", "/posts/"+itoa(int(created["id"].(float64))), map[string]any{
		"status": "published",
	}, true); code != 200 {
		t.Fatalf("发布失败")
	}
	if code, _ := e.do(t, "GET", "/posts/"+slug, nil, false); code != 200 {
		t.Fatalf("发布后游客应可访问，得到 %d", code)
	}

	// 更新内容
	id := itoa(int(created["id"].(float64)))
	if code, _ := e.do(t, "PATCH", "/posts/"+id, map[string]any{
		"summary": "新的摘要",
	}, true); code != 200 {
		t.Fatalf("部分更新失败")
	}
	_, body := e.do(t, "GET", "/posts/"+id, nil, true)
	if body["summary"] != "新的摘要" {
		t.Fatalf("摘要未更新: %v", body["summary"])
	}

	// 回收站
	if code, _ := e.do(t, "POST", "/posts/"+id+"/trash", nil, true); code != 200 {
		t.Fatalf("移入回收站失败")
	}
	if code, _ := e.do(t, "GET", "/posts/"+slug, nil, true); code != http.StatusNotFound {
		t.Fatalf("回收站内容不应出现在列表查询中，得到 %d", code)
	}
	// 回收站列表应能查到
	_, trash := e.do(t, "GET", "/posts?status=trash", nil, true)
	if items, ok := trash["items"].([]any); !ok || len(items) != 1 {
		t.Fatalf("回收站应包含该文章: %v", trash)
	}
	// 恢复
	if code, _ := e.do(t, "POST", "/posts/"+id+"/restore", nil, true); code != 200 {
		t.Fatalf("恢复失败")
	}
	if code, _ := e.do(t, "GET", "/posts/"+id, nil, true); code != 200 {
		t.Fatalf("恢复后应可访问，得到 %d", code)
	}
	// 永久删除
	if code, _ := e.do(t, "DELETE", "/posts/"+id, nil, true); code != 200 {
		t.Fatalf("删除失败")
	}
	if code, _ := e.do(t, "GET", "/posts/"+id, nil, true); code != http.StatusNotFound {
		t.Fatalf("删除后应返回 404，得到 %d", code)
	}
}

// TestPostsValidation 输入校验边界。
func TestPostsValidation(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	cases := []struct {
		name string
		body map[string]any
		want int
	}{
		{"空标题", map[string]any{"title": "  "}, http.StatusBadRequest},
		{"标题超长", map[string]any{"title": strings.Repeat("字", 201)}, http.StatusBadRequest},
		{"摘要超长", map[string]any{"title": "ok", "summary": strings.Repeat("字", 501)}, http.StatusBadRequest},
		{"非法状态", map[string]any{"title": "ok", "status": "unknown"}, http.StatusBadRequest},
		{"非法类型", map[string]any{"title": "ok", "type": "weird"}, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if code, _ := e.do(t, "POST", "/posts", c.body, true); code != c.want {
				t.Fatalf("期望 %d，得到 %d", c.want, code)
			}
		})
	}
}

// TestListStatusAllSentinel 后台「全部」标签页以 status=all 请求列表。
//
// 回归测试：早期实现把 all 当作精确状态匹配（p.status = 'all'），
// 导致文章/页面列表恒为空。all 必须归一化为「不按状态过滤」。
func TestListStatusAllSentinel(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	if code, _ := e.do(t, "POST", "/posts", map[string]any{
		"title": "all-草稿", "content": "x", "status": "draft",
	}, true); code != http.StatusCreated {
		t.Fatalf("创建草稿失败: %d", code)
	}
	if code, _ := e.do(t, "POST", "/posts", map[string]any{
		"title": "all-已发布", "content": "x", "status": "published",
	}, true); code != http.StatusCreated {
		t.Fatalf("创建已发布失败: %d", code)
	}

	for _, reqURL := range []string{"/posts?status=all", "/posts?status=All"} {
		_, body := e.do(t, "GET", reqURL, nil, true)
		items, _ := body["items"].([]any)
		if total, _ := body["total"].(float64); total < 2 || len(items) < 2 {
			t.Fatalf("%s 应返回全部未删除内容，得到 total=%v items=%d", reqURL, body["total"], len(items))
		}
	}

	// 空 status 与 all 等价
	_, plain := e.do(t, "GET", "/posts", nil, true)
	_, all := e.do(t, "GET", "/posts?status=all", nil, true)
	if plain["total"] != all["total"] {
		t.Fatalf("status=all 应与缺省一致: %v vs %v", plain["total"], all["total"])
	}
}

// TestPostSearchLikeEscaping 搜索中的 LIKE 通配符按字面量处理。
//
// 回归测试：早期实现在 MySQL 上写 ESCAPE '\' 会语法错误，
// 且未转义时用户输入的 % 会匹配全部内容。
func TestPostSearchLikeEscaping(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	for _, title := range []string{"100% 完成度", "under_score 命名", "普通标题"} {
		if code, _ := e.do(t, "POST", "/posts", map[string]any{
			"title": title, "content": "x", "status": "published",
		}, true); code != http.StatusCreated {
			t.Fatalf("创建 %q 失败", title)
		}
	}

	// 搜索 "100%" 应只命中一条
	_, body := e.do(t, "GET", "/posts?q="+urlEncode("100%"), nil, false)
	items, _ := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("转义搜索期望 1 条，得到 %d 条: %v", len(items), body)
	}

	// 搜索 "_" 转义后只命中含下划线的一条
	_, body = e.do(t, "GET", "/posts?q="+urlEncode("_"), nil, false)
	items, _ = body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("转义下划线搜索期望 1 条，得到 %d 条", len(items))
	}
}

// TestUnauthorizedWrite 未认证写操作一律 401。
func TestUnauthorizedWrite(t *testing.T) {
	e := newTestEnv(t)
	// 明确不设置 token
	e.token = ""
	writes := []struct{ method, path string }{
		{"POST", "/posts"},
		{"PUT", "/posts/1"},
		{"DELETE", "/posts/1"},
		{"POST", "/categories"},
		{"PUT", "/tags/1"},
		{"DELETE", "/comments/1"},
		{"PUT", "/settings"},
	}
	for _, w := range writes {
		if code, _ := e.do(t, w.method, w.path, map[string]any{}, false); code == http.StatusOK {
			t.Fatalf("%s %s 未认证时不应成功", w.method, w.path)
		}
	}
	// 显式校验关键接口返回 401
	if code, _ := e.do(t, "POST", "/posts", map[string]any{"title": "x"}, false); code != http.StatusUnauthorized {
		t.Fatalf("未认证建文章应 401，得到 %d", code)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func urlEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' {
			b.WriteByte(c)
			continue
		}
		const hexDigits = "0123456789ABCDEF"
		b.WriteByte('%')
		b.WriteByte(hexDigits[c>>4])
		b.WriteByte(hexDigits[c&0x0F])
	}
	return b.String()
}
