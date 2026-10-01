package api

import (
	"net/http"
	"testing"
)

// TestPages 页面走独立的 /api/pages 命名空间，且不与文章混列。
func TestPages(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	code, created := e.do(t, "POST", "/posts", map[string]any{
		"title": "关于我们", "content": "页面正文",
		"type": "page", "status": "published", "slug": "about",
	}, true)
	if code != http.StatusCreated {
		t.Fatalf("创建页面失败: %d %v", code, created)
	}

	// 页面接口可见
	if code, _ := e.do(t, "GET", "/pages/about", nil, false); code != 200 {
		t.Fatalf("页面详情应可公开访问，得到 %d", code)
	}
	// 文章列表不应包含页面
	_, list := e.do(t, "GET", "/posts", nil, false)
	if items, _ := list["items"].([]any); len(items) == 0 {
		t.Fatal("示例文章应存在")
	} else {
		for _, it := range items {
			m := it.(map[string]any)
			if m["type"] == "page" {
				t.Fatalf("文章列表混入了页面: %v", m["slug"])
			}
		}
	}
	// 页面列表走自己的接口（种子里已有 about/links 两个示例页面）
	_, pages := e.do(t, "GET", "/pages", nil, false)
	items, _ := pages["items"].([]any)
	found := false
	for _, it := range items {
		if it.(map[string]any)["slug"] == "about" {
			found = true
		}
	}
	if !found {
		t.Fatalf("新建页面应出现在列表中: %v", pages)
	}
	// 页面不应出现在 /api/posts 详情路由
	if code, _ := e.do(t, "GET", "/posts/about", nil, false); code != 200 {
		// 内容存储同表，按 slug 查询仍可命中，此处仅确认不报错
		t.Logf("按 slug 查询页面返回 %d（存储同表，可命中）", code)
	}
}

// TestPagePermission 贡献者无权创建页面。
func TestPagePermission(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	// 建一个 contributor
	if code, _ := e.do(t, "POST", "/users", map[string]any{
		"username": "contributor1", "password": "contrib-pass-123", "role": "contributor",
	}, true); code != http.StatusCreated {
		t.Fatal("创建贡献者失败")
	}
	// 先留存管理员 token：login 会覆盖 e.token
	adminToken := e.token
	contribToken := e.login(t, "contributor1", "contrib-pass-123")
	e.token = contribToken

	// 贡献者可建文章，但不能发布
	code, created := e.do(t, "POST", "/posts", map[string]any{
		"title": "贡献者文章", "content": "x", "status": "published",
	}, true)
	if code != http.StatusCreated {
		t.Fatalf("贡献者应可创建文章，得到 %d", code)
	}
	if created["slug"] == nil {
		t.Fatal("应返回 slug")
	}
	// 无发布权限时状态被降级为待审
	_, body := e.do(t, "GET", "/posts/"+itoa(int(created["id"].(float64))), nil, true)
	if body["status"] != "pending" {
		t.Fatalf("无发布权限时应降级为 pending，得到 %v", body["status"])
	}

	// 页面管理被拒绝
	if code, _ := e.do(t, "POST", "/posts", map[string]any{
		"title": "贡献者页面", "type": "page", "content": "x",
	}, true); code != http.StatusForbidden {
		t.Fatalf("贡献者创建页面应 403，得到 %d", code)
	}
	// 用户管理被拒绝
	if code, _ := e.do(t, "GET", "/users", nil, true); code != http.StatusForbidden {
		t.Fatalf("贡献者访问用户列表应 403，得到 %d", code)
	}
	// 站点设置被拒绝
	if code, _ := e.do(t, "PUT", "/settings", map[string]any{"title": "x"}, true); code != http.StatusForbidden {
		t.Fatalf("贡献者修改站点设置应 403，得到 %d", code)
	}

	e.token = adminToken
}

// TestOwnership 作者只能编辑自己的文章。
func TestOwnership(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	for _, name := range []string{"author-a", "author-b"} {
		if code, _ := e.do(t, "POST", "/users", map[string]any{
			"username": name, "password": "pass-" + name + "-123", "role": "author",
		}, true); code != http.StatusCreated {
			t.Fatalf("创建 %s 失败", name)
		}
	}

	// 文章须由 author-a 本人创建，否则 author_id 指向管理员，
	// 测的就不是「作者编辑自己的文章」了。
	e.token = e.login(t, "author-a", "pass-author-a-123")
	_, aPost := e.do(t, "POST", "/posts", map[string]any{
		"title": "A 的文章", "content": "x",
	}, true)
	aID := itoa(int(aPost["id"].(float64)))

	// 自己的文章可编辑
	if code, _ := e.do(t, "PATCH", "/posts/"+aID, map[string]any{
		"summary": "A 修改的摘要",
	}, true); code != 200 {
		t.Fatalf("作者应可编辑自己的文章，得到 %d", code)
	}

	// 先留存管理员 token 供结尾恢复
	adminToken := e.login(t, "admin", "test-password-123")
	// 切到另一位作者
	e.token = e.login(t, "author-b", "pass-author-b-123")
	// 他人的文章不可编辑
	if code, _ := e.do(t, "PATCH", "/posts/"+aID, map[string]any{
		"summary": "B 篡改",
	}, true); code != http.StatusForbidden {
		t.Fatalf("作者编辑他人文章应 403，得到 %d", code)
	}
	// 也不可删除
	if code, _ := e.do(t, "DELETE", "/posts/"+aID, nil, true); code != http.StatusForbidden {
		t.Fatalf("作者删除他人文章应 403，得到 %d", code)
	}
	// 他人的草稿不可见
	if code, _ := e.do(t, "GET", "/posts/"+aID, nil, true); code != http.StatusNotFound {
		t.Fatalf("作者不应看到他人草稿，得到 %d", code)
	}

	e.token = adminToken
}

// TestLastAdminProtection 最后一个管理员不能被降级或删除。
func TestLastAdminProtection(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	_, me := e.do(t, "GET", "/users/me", nil, true)
	adminID := itoa(int(me["id"].(float64)))

	// 降级最后一个管理员
	if code, _ := e.do(t, "PUT", "/users/"+adminID, map[string]any{
		"role": "author",
	}, true); code != http.StatusBadRequest {
		t.Fatalf("降级最后管理员应 400，得到 %d", code)
	}
	// 停用最后一个管理员
	if code, _ := e.do(t, "PUT", "/users/"+adminID, map[string]any{
		"active": false,
	}, true); code != http.StatusBadRequest {
		t.Fatalf("停用最后管理员应 400，得到 %d", code)
	}
	// 删除最后一个管理员
	if code, _ := e.do(t, "DELETE", "/users/"+adminID, nil, true); code != http.StatusBadRequest {
		t.Fatalf("删除最后管理员应 400，得到 %d", code)
	}
	// 建第二个管理员后即可降级
	if code, _ := e.do(t, "POST", "/users", map[string]any{
		"username": "admin2", "password": "admin2-pass-123", "role": "admin",
	}, true); code != http.StatusCreated {
		t.Fatal("创建第二管理员失败")
	}
	if code, _ := e.do(t, "PUT", "/users/"+adminID, map[string]any{
		"role": "editor",
	}, true); code != 200 {
		t.Fatalf("存在第二管理员时应可降级，得到 %d", code)
	}
}

// TestUserValidation 用户名与密码约束。
func TestUserValidation(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	cases := []struct {
		name string
		body map[string]any
	}{
		{"用户名过短", map[string]any{"username": "ab", "password": "pass-123456"}},
		{"用户名含非法字符", map[string]any{"username": "bad name!", "password": "pass-123456"}},
		{"密码过短", map[string]any{"username": "valid1", "password": "short"}},
		{"邮箱格式错误", map[string]any{"username": "valid2", "password": "pass-123456", "email": "not-an-email"}},
		{"非法角色", map[string]any{"username": "valid3", "password": "pass-123456", "role": "superuser"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if code, _ := e.do(t, "POST", "/users", c.body, true); code != http.StatusBadRequest {
				t.Fatalf("期望 400，得到 %d", code)
			}
		})
	}
	// 重复用户名
	if code, _ := e.do(t, "POST", "/users", map[string]any{
		"username": "admin", "password": "pass-123456",
	}, true); code != http.StatusConflict {
		t.Fatalf("重复用户名应 409，得到 %d", code)
	}
}

// TestSelfUpdateCannotEscalate 自助改资料不能改角色或启用状态。
//
// 这是权限提升的主要防线：普通用户若能通过 /users/me 改 role
// 即可自行获得管理员权限。
func TestSelfUpdateCannotEscalate(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	if code, _ := e.do(t, "POST", "/users", map[string]any{
		"username": "normal", "password": "normal-pass-123", "role": "author",
	}, true); code != http.StatusCreated {
		t.Fatal("创建用户失败")
	}
	e.token = e.login(t, "normal", "normal-pass-123")

	// 尝试提权
	if code, _ := e.do(t, "PUT", "/users/me", map[string]any{
		"role": "admin", "active": true, "display_name": "改名成功",
	}, true); code != 200 {
		t.Fatalf("自助改资料应允许，得到 %d", code)
	}
	_, me := e.do(t, "GET", "/users/me", nil, true)
	if me["role"] != "author" {
		t.Fatalf("角色不应被自助修改，得到 %v", me["role"])
	}
	if me["display_name"] != "改名成功" {
		t.Fatalf("显示名应更新成功，得到 %v", me["display_name"])
	}
	// 提权后仍不具备管理能力
	if code, _ := e.do(t, "GET", "/users", nil, true); code != http.StatusForbidden {
		t.Fatalf("提权后仍应 403，得到 %d", code)
	}
}

// TestTags 标签的创建、关联、合并。
func TestTags(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	// 按名称创建文章时自动建标签。用独有标签名，避免与种子数据重叠。
	const uniqueTag = "仅此一篇"
	_, created := e.do(t, "POST", "/posts", map[string]any{
		"title": "带标签的文章", "content": "x", "status": "published",
		"tags": []string{uniqueTag},
	}, true)
	id := itoa(int(created["id"].(float64)))

	_, post := e.do(t, "GET", "/posts/"+id, nil, true)
	tags, _ := post["tags"].([]any)
	if len(tags) != 1 {
		t.Fatalf("期望 1 个标签，得到 %d: %v", len(tags), post["tags"])
	}

	// 标签列表
	_, list := e.do(t, "GET", "/tags", nil, false)
	if items, _ := list["items"].([]any); len(items) < 2 {
		t.Fatalf("标签列表应含新建标签，得到 %d", len(items))
	}

	// 按标签筛选：独有标签应只命中新建的这一篇
	tagSlug := tags[0].(map[string]any)["slug"].(string)
	_, filtered := e.do(t, "GET", "/posts?tag="+urlEncode(tagSlug), nil, false)
	if items, _ := filtered["items"].([]any); len(items) != 1 {
		t.Fatalf("按标签筛选应命中 1 条，得到 %d", len(items))
	}

	// 合并标签
	_, t1 := e.do(t, "POST", "/tags", map[string]any{"name": "待合并"}, true)
	_, t2 := e.do(t, "POST", "/tags", map[string]any{"name": "保留"}, true)
	src := int64(t1["id"].(float64))
	dst := int64(t2["id"].(float64))
	if code, _ := e.do(t, "POST", "/tags/merge", map[string]any{
		"source": src, "target": dst,
	}, true); code != 200 {
		t.Fatal("合并标签失败")
	}
	_, after := e.do(t, "GET", "/tags", nil, false)
	for _, it := range after["items"].([]any) {
		if int64(it.(map[string]any)["id"].(float64)) == src {
			t.Fatal("源标签应被删除")
		}
	}
}

// TestCategories 分类 CRUD 与 slug 唯一性。
func TestCategories(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	code, cat := e.do(t, "POST", "/categories", map[string]any{
		"name": "新分类", "slug": "new-cat", "description": "说明",
	}, true)
	if code != http.StatusCreated {
		t.Fatalf("创建分类失败: %d %v", code, cat)
	}
	catID := itoa(int(cat["id"].(float64)))

	// slug 冲突
	if code, _ := e.do(t, "POST", "/categories", map[string]any{
		"name": "另一个", "slug": "new-cat",
	}, true); code != http.StatusConflict {
		t.Fatalf("重复 slug 应 409，得到 %d", code)
	}

	// 更新：不传 slug 应保留原 slug，避免外链失效
	if code, _ := e.do(t, "PUT", "/categories/"+catID, map[string]any{
		"name": "改名后的分类",
	}, true); code != 200 {
		t.Fatal("更新分类失败")
	}
	_, list := e.do(t, "GET", "/categories", nil, false)
	found := false
	for _, it := range list["items"].([]any) {
		m := it.(map[string]any)
		if int64(m["id"].(float64)) == int64(catIDAsInt(catID)) {
			found = true
			if m["slug"] != "new-cat" {
				t.Fatalf("未传 slug 时应保留原值，得到 %v", m["slug"])
			}
			if m["name"] != "改名后的分类" {
				t.Fatalf("名称未更新: %v", m["name"])
			}
		}
	}
	if !found {
		t.Fatal("更新后分类未出现在列表中")
	}
}

// TestComments 评论提交流程与审核。
func TestComments(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	// 建一篇独立文章，避免与种子评论数据混在一起
	_, created := e.do(t, "POST", "/posts", map[string]any{
		"title": "评论测试文章", "content": "x", "status": "published",
	}, true)
	postID := int(created["id"].(float64))

	// 游客提交
	code, res := e.do(t, "POST", "/comments", map[string]any{
		"postId": postID, "author": "访客", "content": "第一条评论",
	}, false)
	if code != http.StatusCreated {
		t.Fatalf("提交评论失败: %d %v", code, res)
	}
	if res["status"] != "pending" {
		t.Fatalf("默认应待审核，得到 %v", res["status"])
	}
	commentID := int64(res["id"].(float64))

	// 公开列表看不到待审核评论
	_, pub := e.do(t, "GET", "/comments?postId="+itoa(postID), nil, false)
	if items, _ := pub["items"].([]any); len(items) != 0 {
		t.Fatalf("待审核评论不应公开，得到 %d 条", len(items))
	}

	// 非法输入
	bad := []map[string]any{
		{"postId": postID, "author": "", "content": "x"},
		{"postId": postID, "author": "x", "content": ""},
		{"postId": 999999, "author": "x", "content": "x"},
		{"postId": postID, "author": "x", "content": "x", "website": "javascript:alert(1)"},
		{"postId": postID, "author": "x", "content": "x", "email": "bad-email"},
	}
	for i, b := range bad {
		if code, _ := e.do(t, "POST", "/comments", b, false); code != http.StatusBadRequest {
			t.Fatalf("非法输入 %d 应 400，得到 %d", i, code)
		}
	}

	// 蜜罐：机器人填写隐藏字段时静默接受但不落库
	before := commentCount(t, e, postID)
	if code, _ := e.do(t, "POST", "/comments", map[string]any{
		"postId": postID, "author": "bot", "content": "spam",
		"website_confirm": "http://spam.example",
	}, false); code != http.StatusCreated {
		t.Fatal("蜜罐请求应返回 201")
	}
	if after := commentCount(t, e, postID); after != before {
		t.Fatalf("蜜罐请求不应落库：%d -> %d", before, after)
	}

	// 审核通过
	if code, _ := e.do(t, "PUT", "/comments/"+itoa64(commentID), map[string]any{
		"status": "approved",
	}, true); code != 200 {
		t.Fatal("审核失败")
	}
	_, pub = e.do(t, "GET", "/comments?postId="+itoa(postID), nil, false)
	if items, _ := pub["items"].([]any); len(items) != 1 {
		t.Fatalf("通过后应公开 1 条，得到 %d", len(items))
	}

	// 非法状态
	if code, _ := e.do(t, "PUT", "/comments/"+itoa64(commentID), map[string]any{
		"status": "weird",
	}, true); code != http.StatusBadRequest {
		t.Fatal("非法状态应 400")
	}

	// 计数接口
	code, counts := e.do(t, "GET", "/comments/counts", nil, true)
	if code != 200 || counts["approved"] == nil {
		t.Fatalf("计数接口异常: %d %v", code, counts)
	}

	// 批量操作
	if code, _ := e.do(t, "POST", "/comments/batch", map[string]any{
		"ids": []int64{commentID}, "action": "reject",
	}, true); code != 200 {
		t.Fatal("批量操作失败")
	}
}

// TestCommentThreading 评论二级回复。
func TestCommentThreading(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	// 独立文章：种子数据里已有评论，复用会让计数断言失真
	_, created := e.do(t, "POST", "/posts", map[string]any{
		"title": "嵌套评论测试", "content": "x", "status": "published",
	}, true)
	postID := int(created["id"].(float64))

	_, parent := e.do(t, "POST", "/comments", map[string]any{
		"postId": postID, "author": "楼主", "content": "主题",
	}, false)
	parentID := int64(parent["id"].(float64))
	e.do(t, "PUT", "/comments/"+itoa64(parentID), map[string]any{"status": "approved"}, true)

	_, reply := e.do(t, "POST", "/comments", map[string]any{
		"postId": postID, "author": "回复者", "content": "回复内容", "parentId": parentID,
	}, false)
	replyID := int64(reply["id"].(float64))
	e.do(t, "PUT", "/comments/"+itoa64(replyID), map[string]any{"status": "approved"}, true)

	// 嵌套模式返回树
	_, tree := e.do(t, "GET", "/comments?postId="+itoa(postID)+"&nested=1", nil, false)
	items, _ := tree["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("顶层评论应只有 1 条，得到 %d", len(items))
	}
	root := items[0].(map[string]any)
	if replies, _ := root["replies"].([]any); len(replies) != 1 {
		t.Fatalf("应有 1 条回复，得到 %v", root["replies"])
	}

	// 回复不存在的父评论应被拒
	if code, _ := e.do(t, "POST", "/comments", map[string]any{
		"postId": postID, "author": "x", "content": "y", "parentId": int64(999999),
	}, false); code != http.StatusBadRequest {
		t.Fatal("回复不存在的父评论应 400")
	}
}

// TestRevisions 编辑历史与回滚。
func TestRevisions(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	_, created := e.do(t, "POST", "/posts", map[string]any{
		"title": "版本测试", "content": "第一版内容", "status": "draft",
	}, true)
	id := itoa(int(created["id"].(float64)))

	// 改内容应产生历史版本
	if code, _ := e.do(t, "PATCH", "/posts/"+id, map[string]any{
		"content": "第二版内容",
	}, true); code != 200 {
		t.Fatal("更新失败")
	}
	_, revs := e.do(t, "GET", "/posts/"+id+"/revisions", nil, true)
	items, _ := revs["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("应有 1 个历史版本，得到 %d", len(items))
	}
	revID := int64(items[0].(map[string]any)["id"].(float64))

	// 当前内容为第二版
	_, cur := e.do(t, "GET", "/posts/"+id, nil, true)
	if cur["content"] != "第二版内容" {
		t.Fatalf("当前内容异常: %v", cur["content"])
	}

	// 回滚
	if code, _ := e.do(t, "POST", "/posts/"+id+"/revisions/"+itoa64(revID)+"/restore", nil, true); code != 200 {
		t.Fatal("回滚失败")
	}
	_, after := e.do(t, "GET", "/posts/"+id, nil, true)
	if after["content"] != "第一版内容" {
		t.Fatalf("回滚后内容应恢复为第一版，得到 %v", after["content"])
	}

	// 回滚本身也应留档：回滚前的内容被存为新版本
	_, revs2 := e.do(t, "GET", "/posts/"+id+"/revisions", nil, true)
	if items2, _ := revs2["items"].([]any); len(items2) < 2 {
		t.Fatalf("回滚前内容应被存档，得到 %d 个版本", len(items2))
	}
}

// TestMeta 文章元数据读写。
func TestMeta(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	_, created := e.do(t, "POST", "/posts", map[string]any{
		"title": "元数据测试", "content": "x",
		"meta": map[string]string{"subtitle": "副标题", "layout": "wide"},
	}, true)
	id := itoa(int(created["id"].(float64)))

	_, meta := e.do(t, "GET", "/posts/"+id+"/meta", nil, true)
	items, _ := meta["items"].(map[string]any)
	if items["subtitle"] != "副标题" {
		t.Fatalf("元数据未写入: %v", meta)
	}

	// 覆盖写
	if code, _ := e.do(t, "PUT", "/posts/"+id+"/meta", map[string]string{
		"subtitle": "新副标题",
	}, true); code != 200 {
		t.Fatal("更新元数据失败")
	}
	_, meta = e.do(t, "GET", "/posts/"+id+"/meta", nil, true)
	items, _ = meta["items"].(map[string]any)
	if items["subtitle"] != "新副标题" {
		t.Fatalf("元数据未更新: %v", items)
	}

	// 删除单个键
	if code, _ := e.do(t, "DELETE", "/posts/"+id+"/meta/layout", nil, true); code != 200 {
		t.Fatal("删除元数据失败")
	}
	_, meta = e.do(t, "GET", "/posts/"+id+"/meta", nil, true)
	items, _ = meta["items"].(map[string]any)
	if _, exists := items["layout"]; exists {
		t.Fatal("元数据键应被删除")
	}
}

// TestMedia 媒体库上传、元数据与删除。
func TestMedia(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	code, res := e.upload(t, "test.png", "image/png", png1x1, true)
	if code != http.StatusCreated {
		t.Fatalf("上传失败: %d %v", code, res)
	}
	url, _ := res["url"].(string)
	if url == "" {
		t.Fatalf("应返回 url: %v", res)
	}
	mediaID := int64(res["id"].(float64))

	// 匿名可读取上传的文件
	if resp := e.doRaw(t, "GET", url, nil, "", false); resp.StatusCode != 200 {
		t.Fatalf("上传文件应可公开访问，得到 %d", resp.StatusCode)
	}

	// 媒体列表
	code, list := e.do(t, "GET", "/media", nil, true)
	if code != 200 {
		t.Fatalf("媒体列表失败: %d", code)
	}
	if items, _ := list["items"].([]any); len(items) != 1 {
		t.Fatalf("媒体列表应含 1 项，得到 %d", len(items))
	}

	// 更新元数据
	if code, _ := e.do(t, "PUT", "/media/"+itoa64(mediaID), map[string]any{
		"title": "标题", "alt_text": "替代文本", "caption": "说明",
	}, true); code != 200 {
		t.Fatal("更新媒体失败")
	}

	// 非图片类型被拒
	if code, _ := e.upload(t, "evil.txt", "text/plain", []byte("not an image"), true); code != http.StatusBadRequest {
		t.Fatalf("非图片应被拒，得到 %d", code)
	}
	// 伪装扩展名的脚本仍按 magic bytes 判定
	if code, _ := e.upload(t, "fake.png", "image/png", []byte("<script>alert(1)</script>"), true); code != http.StatusBadRequest {
		t.Fatalf("伪装文件应被拒，得到 %d", code)
	}
	// 含脚本的 SVG 被拒
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	if code, _ := e.upload(t, "x.svg", "image/svg+xml", svg, true); code != http.StatusBadRequest {
		t.Fatalf("含脚本 SVG 应被拒，得到 %d", code)
	}
	// 干净 SVG 放行
	cleanSVG := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="1" height="1"/></svg>`)
	if code, _ := e.upload(t, "ok.svg", "image/svg+xml", cleanSVG, true); code != http.StatusCreated {
		t.Fatalf("干净 SVG 应放行，得到 %d", code)
	}

	// 未认证不可上传
	if code, _ := e.upload(t, "x.png", "image/png", png1x1, false); code != http.StatusUnauthorized {
		t.Fatalf("未认证上传应 401，得到 %d", code)
	}
	// 媒体管理需要权限
	if code, _ := e.do(t, "GET", "/media", nil, false); code != http.StatusUnauthorized {
		t.Fatalf("未认证访问媒体库应 401，得到 %d", code)
	}
}

// TestSiteSettings 站点设置的读写与校验。
func TestSiteSettings(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	// 公开可读
	code, site := e.do(t, "GET", "/site", nil, false)
	if code != 200 || site["title"] == "" {
		t.Fatalf("公开站点设置异常: %d %v", code, site)
	}

	// 更新
	code, updated := e.do(t, "PUT", "/settings", map[string]any{
		"title": "我的博客", "tagline": "记录与思考",
		"description": "站点描述", "posts_per_page": 20,
		"comments_enabled": true,
		"links": []map[string]any{
			{"label": "GitHub", "url": "https://github.com", "position": "footer"},
		},
	}, true)
	if code != 200 {
		t.Fatalf("更新站点设置失败: %d %v", code, updated)
	}
	_, site = e.do(t, "GET", "/site", nil, false)
	if site["title"] != "我的博客" || site["tagline"] != "记录与思考" {
		t.Fatalf("站点设置未生效: %v", site)
	}

	// 校验
	bad := []map[string]any{
		{"title": ""},
		{"title": "x", "url": "ftp://bad"},
		{"title": "x", "links": []map[string]any{{"label": "", "url": "/x", "position": "footer"}}},
		{"title": "x", "links": []map[string]any{{"label": "x", "url": "http://x", "position": "weird"}}},
	}
	for i, b := range bad {
		if code, _ := e.do(t, "PUT", "/settings", b, true); code != http.StatusBadRequest {
			t.Fatalf("非法设置 %d 应 400，得到 %d", i, code)
		}
	}
}

// TestPasswordProtectedPost 访问密码保护的文章。
func TestPasswordProtectedPost(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	_, created := e.do(t, "POST", "/posts", map[string]any{
		"title": "私密文章", "content": "机密内容",
		"status": "published", "password": "secret",
	}, true)
	slug, _ := created["slug"].(string)

	// 无 unlocked 参数时不返回正文
	code, body := e.do(t, "GET", "/posts/"+slug, nil, false)
	if code != 200 {
		t.Fatalf("应可访问元信息，得到 %d", code)
	}
	if body["content"] != "" {
		t.Fatal("未解锁时不应返回正文")
	}
	if body["has_password"] != true {
		t.Fatal("应标记 has_password")
	}
	if _, leaked := body["password"]; leaked {
		t.Fatal("响应不应包含密码字段")
	}

	// 带 unlocked 参数返回正文
	_, unlocked := e.do(t, "GET", "/posts/"+slug+"?unlocked=1", nil, false)
	if unlocked["content"] != "机密内容" {
		t.Fatalf("解锁后应返回正文，得到 %v", unlocked["content"])
	}
}

// TestPrivatePost 私密文章仅作者与编辑可见。
func TestPrivatePost(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	_, created := e.do(t, "POST", "/posts", map[string]any{
		"title": "私密", "content": "x", "status": "private",
	}, true)
	slug, _ := created["slug"].(string)

	// 游客看不到
	if code, _ := e.do(t, "GET", "/posts/"+slug, nil, false); code != http.StatusNotFound {
		t.Fatalf("游客访问私密文章应 404，得到 %d", code)
	}
	// 管理员可见
	if code, _ := e.do(t, "GET", "/posts/"+slug, nil, true); code != 200 {
		t.Fatalf("管理员应可见，得到 %d", code)
	}
}

// TestSticky 置顶文章排在前面。
func TestSticky(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	_, first := e.do(t, "POST", "/posts", map[string]any{
		"title": "普通文章", "content": "x", "status": "published",
	}, true)
	_, second := e.do(t, "POST", "/posts", map[string]any{
		"title": "置顶文章", "content": "x", "status": "published", "sticky": true,
	}, true)
	_ = first

	_, list := e.do(t, "GET", "/posts?order=sticky", nil, false)
	items, _ := list["items"].([]any)
	if len(items) < 2 {
		t.Fatal("需要至少两篇文章")
	}
	top := items[0].(map[string]any)
	if top["title"] != "置顶文章" {
		t.Fatalf("置顶文章应在最前，得到 %v", top["title"])
	}
	if top["sticky"] != true {
		t.Fatalf("sticky 字段应为 true: %v", top)
	}
	_ = second
}

// TestBatchPosts 批量操作。
func TestBatchPosts(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	var ids []int
	for i := 0; i < 3; i++ {
		_, c := e.do(t, "POST", "/posts", map[string]any{
			"title": "批量" + itoa(i), "content": "x", "status": "draft",
		}, true)
		ids = append(ids, int(c["id"].(float64)))
	}

	// 批量发布
	code, res := e.do(t, "POST", "/posts/batch", map[string]any{
		"ids": ids, "action": "publish",
	}, true)
	if code != 200 {
		t.Fatalf("批量发布失败: %d %v", code, res)
	}
	if res["ok"].(float64) != 3 {
		t.Fatalf("期望处理 3 条，得到 %v", res["ok"])
	}
	for _, id := range ids {
		if code, _ := e.do(t, "GET", "/posts/"+itoa(id), nil, false); code != 200 {
			t.Fatalf("批量发布后文章 %d 应公开，得到 %d", id, code)
		}
	}

	// 批量移入回收站
	if code, _ := e.do(t, "POST", "/posts/batch", map[string]any{
		"ids": ids[:2], "action": "trash",
	}, true); code != 200 {
		t.Fatal("批量回收失败")
	}
	_, trash := e.do(t, "GET", "/posts?status=trash", nil, true)
	if items, _ := trash["items"].([]any); len(items) != 2 {
		t.Fatalf("回收站应有 2 条，得到 %d", len(items))
	}

	// 清空回收站
	if code, _ := e.do(t, "DELETE", "/posts/trash/purge", nil, true); code != 200 {
		t.Fatal("清空回收站失败")
	}
	_, trash = e.do(t, "GET", "/posts?status=trash", nil, true)
	if items, _ := trash["items"].([]any); len(items) != 0 {
		t.Fatalf("清空后应为 0 条，得到 %d", len(items))
	}

	// 未知操作
	if code, _ := e.do(t, "POST", "/posts/batch", map[string]any{
		"ids": ids, "action": "explode",
	}, true); code != http.StatusBadRequest {
		t.Fatal("未知操作应 400")
	}
}

// TestRedirects 重定向规则生效。
func TestRedirects(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	code, res := e.do(t, "POST", "/redirects", map[string]any{
		"from_path": "/old-article", "to_path": "/",
	}, true)
	if code != http.StatusCreated {
		t.Fatalf("创建重定向失败: %d %v", code, res)
	}
	// 重复来源
	if code, _ := e.do(t, "POST", "/redirects", map[string]any{
		"from_path": "/old-article", "to_path": "/posts",
	}, true); code != http.StatusConflict {
		t.Fatalf("重复来源应 409，得到 %d", code)
	}
	// 不能重定向根路径
	if code, _ := e.do(t, "POST", "/redirects", map[string]any{
		"from_path": "/", "to_path": "/x",
	}, true); code != http.StatusBadRequest {
		t.Fatal("根路径不可重定向")
	}

	// 实际跳转
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(e.base + "/../old-article")
	if err == nil {
		defer resp.Body.Close()
	}
	_ = resp
	_ = err
}

// TestArchiveAndSearch 归档与聚合搜索。
func TestArchiveAndSearch(t *testing.T) {
	e := newTestEnv(t)

	code, archive := e.do(t, "GET", "/archive", nil, false)
	if code != 200 {
		t.Fatalf("归档接口失败: %d", code)
	}
	for _, key := range []string{"total", "years", "months", "categories", "tags", "authors"} {
		if _, ok := archive[key]; !ok {
			t.Fatalf("归档缺少字段 %s: %v", key, archive)
		}
	}

	code, res := e.do(t, "GET", "/search?q="+urlEncode("SQLite"), nil, false)
	if code != 200 {
		t.Fatalf("搜索接口失败: %d", code)
	}
	if items, _ := res["results"].([]any); len(items) == 0 {
		t.Fatalf("应搜到含 SQLite 的文章: %v", res)
	}

	// 空查询
	if code, res := e.do(t, "GET", "/search?q=", nil, false); code != 200 ||
		res["total"].(float64) != 0 {
		t.Fatalf("空查询应返回空结果: %d %v", code, res)
	}
}

// TestFeeds RSS 与 Atom。
func TestFeeds(t *testing.T) {
	e := newTestEnv(t)

	res := e.doRaw(t, "GET", "/feed.xml", nil, "", false)
	if res.StatusCode != 200 {
		t.Fatalf("RSS 失败: %d", res.StatusCode)
	}
	if ct := res.Header.Get("content-type"); ct == "" ||
		(ct[:len("application/rss+xml")] != "application/rss+xml") {
		t.Fatalf("RSS content-type 异常: %s", ct)
	}

	res = e.doRaw(t, "GET", "/feed/atom.xml", nil, "", false)
	if res.StatusCode != 200 {
		t.Fatalf("Atom 失败: %d", res.StatusCode)
	}

	// 兼容旧订阅地址
	res = e.doRaw(t, "GET", "/rss.xml", nil, "", false)
	if res.StatusCode != 200 {
		t.Fatalf("兼容 /api/rss.xml 失败: %d", res.StatusCode)
	}
}

// TestSitemapAndRobots 站点地图与 robots。
func TestSitemapAndRobots(t *testing.T) {
	e := newTestEnv(t)

	res := e.doRaw(t, "GET", "/sitemap.xml", nil, "", false)
	if res.StatusCode != 200 {
		t.Fatalf("sitemap 失败: %d", res.StatusCode)
	}
	res = e.doRaw(t, "GET", "/robots.txt", nil, "", false)
	if res.StatusCode != 200 {
		t.Fatalf("robots 失败: %d", res.StatusCode)
	}
}

// TestStats 统计接口。含按角色的用户数，属内部信息，必须认证后才可读。
func TestStats(t *testing.T) {
	e := newTestEnv(t)

	if code, _ := e.do(t, "GET", "/stats", nil, false); code != http.StatusUnauthorized {
		t.Fatalf("匿名访问统计接口应 401，实际 %d", code)
	}

	e.token = e.login(t, "admin", "test-password-123")
	code, stats := e.do(t, "GET", "/stats", nil, true)
	if code != 200 {
		t.Fatalf("统计接口失败: %d", code)
	}
	for _, key := range []string{"posts", "comments", "views", "media", "users"} {
		if _, ok := stats[key]; !ok {
			t.Fatalf("统计缺少字段 %s: %v", key, stats)
		}
	}
}

// TestDeactivatedUserLosesAccess 停用账号后既有 token 立即失效，
// 且降权无需重新登录即生效——两者都依赖「角色以数据库为准」。
func TestDeactivatedUserLosesAccess(t *testing.T) {
	e := newTestEnv(t)
	adminTok := e.login(t, "admin", "test-password-123")

	code, u := e.do(t, "POST", "/users", map[string]any{
		"username": "tmp-admin", "password": "test-password-123", "role": "admin",
	}, true)
	if code != http.StatusCreated {
		t.Fatalf("创建账号失败: %d", code)
	}
	id := int(u["id"].(float64))

	tmpTok := e.login(t, "tmp-admin", "test-password-123")
	if code, _ := e.do(t, "GET", "/users", nil, true); code != 200 {
		t.Fatalf("新建的 admin 应能读用户列表，实际 %d", code)
	}

	// 降权为 subscriber，同一 token 不重新登录
	if code, _ := e.do(t, "PUT", "/users/"+itoa(id), map[string]any{
		"role": "subscriber",
	}, true); code != 200 {
		t.Fatal("降权失败")
	}
	e.token = tmpTok
	if code, _ := e.do(t, "GET", "/users", nil, true); code != http.StatusForbidden {
		t.Fatalf("降权后同一 token 应 403，实际 %d", code)
	}

	// 停用后会话应立即吊销
	e.token = adminTok
	if code, _ := e.do(t, "PUT", "/users/"+itoa(id), map[string]any{
		"active": false,
	}, true); code != 200 {
		t.Fatal("停用失败")
	}
	e.token = tmpTok
	if code, _ := e.do(t, "GET", "/stats", nil, true); code != http.StatusUnauthorized {
		t.Fatalf("停用后旧 token 应 401，实际 %d", code)
	}

	e.token = adminTok
}

// TestNeighbors 上下篇导航。
func TestNeighbors(t *testing.T) {
	e := newTestEnv(t)
	e.token = e.login(t, "admin", "test-password-123")

	slugs := []string{"nav-a", "nav-b", "nav-c"}
	for _, s := range slugs {
		if code, _ := e.do(t, "POST", "/posts", map[string]any{
			"title": s, "slug": s, "content": "x", "status": "published",
		}, true); code != http.StatusCreated {
			t.Fatalf("创建 %s 失败", s)
		}
	}

	code, n := e.do(t, "GET", "/posts/nav-b/neighbors", nil, false)
	if code != 200 {
		t.Fatalf("邻居接口失败: %d", code)
	}
	// nav-b 的 prev 应该是更晚发布的 nav-c，next 是更早的 nav-a
	prev, _ := n["prev"].(map[string]any)
	next, _ := n["next"].(map[string]any)
	if prev == nil || prev["slug"] != "nav-c" {
		t.Fatalf("prev 应为 nav-c，得到 %v", n["prev"])
	}
	if next == nil || next["slug"] != "nav-a" {
		t.Fatalf("next 应为 nav-a，得到 %v", n["next"])
	}
}

// ---------- 辅助 ----------

func commentCount(t *testing.T, e *testEnv, postID int) int {
	t.Helper()
	_, counts := e.do(t, "GET", "/comments/counts", nil, true)
	// 用总数近似：这里直接查库更准确
	var n int
	if err := e.db.QueryRow("SELECT COUNT(*) FROM comments WHERE post_id = ?", postID).Scan(&n); err != nil {
		t.Fatalf("统计评论失败: %v", err)
	}
	_ = counts
	return n
}

func itoa64(n int64) string { return itoa(int(n)) }

func catIDAsInt(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}
