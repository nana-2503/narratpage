package api

import (
	"net/http"
	"os"
	"testing"
	"time"

	"narratpage/internal/config"
	"narratpage/internal/db"
	"narratpage/internal/repo"
	"narratpage/internal/seed"
)

// newCrossDBEnv 在外部数据库上构建完整环境并跑通核心流程。
//
// 这是本项目「多数据库支持」的核心保障：SQLite 通过不代表
// MySQL / PostgreSQL 通过，方言差异（自增、RETURNING、布尔、
// 索引幂等、占位符）必须在这两个真实引擎上验证。
func newCrossDBEnv(t *testing.T, dbType, dsn string) *testEnv {
	t.Helper()

	// 每次用独立库名，避免与其它测试或开发数据冲突
	schema := "nptest_" + randomSuffix(t)
	adminDSN, cleanupDSN := withDatabase(t, dbType, dsn, schema)
	t.Cleanup(cleanupDSN)

	dir := t.TempDir()
	cfg := config.Config{
		DataDir:       dir,
		DBType:        dbType,
		DBDSN:         adminDSN,
		JWTSecret:     "test-secret",
		JWTExpiry:     time.Hour,
		AdminUsername: "admin",
		AdminPassword: "test-password-123",
		SiteURL:       "http://localhost:8080",
		Port:          "0",
	}

	database, err := db.Open(cfg)
	if err != nil {
		t.Fatalf("[%s] 打开数据库失败: %v", dbType, err)
	}
	if err := seed.Run(database, cfg); err != nil {
		t.Fatalf("[%s] 种子数据失败: %v", dbType, err)
	}
	options := repo.NewOptions(database, cfg.DBType)
	if err := options.EnsureDefaults(); err != nil {
		t.Fatalf("[%s] 写入默认设置失败: %v", dbType, err)
	}

	uploadDir := dir + "/uploads"
	if err := ensureDir(uploadDir); err != nil {
		t.Fatal(err)
	}

	env := newEnvFor(t, cfg, database, options, uploadDir)
	resetCommentLimiter()
	return env
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	return itoa(int(time.Now().UnixNano() % 1_000_000))
}

// withDatabase 在目标库上创建独立 schema，并返回该 schema 的 DSN 与清理函数。
func withDatabase(t *testing.T, dbType, dsn, schema string) (string, func()) {
	t.Helper()
	if dbType == "mysql" {
		if _, err := db.Exec2(dsn, dbType,
			"CREATE DATABASE IF NOT EXISTS "+schema+" CHARACTER SET utf8mb4"); err != nil {
			t.Skipf("跳过：无法创建测试库 %v", err)
		}
		return replaceMySQLDB(dsn, schema), func() { dropMySQL(t, dsn, schema) }
	}
	// PostgreSQL 用独立 database（而非 schema）隔离：
	// search_path 在不同驱动/连接池下行为不一致，
	// 独立库能让 DSN 与开发环境形态保持一致。
	if _, err := db.Exec2(dsn, dbType, "CREATE DATABASE "+schema); err != nil {
		t.Skipf("跳过：无法创建测试库 %v", err)
	}
	return replacePGDB(dsn, schema), func() { dropPG(t, dsn, schema) }
}

func dropMySQL(t *testing.T, dsn, schema string) {
	_, _ = db.Exec2(dsn, "mysql", "DROP DATABASE IF EXISTS "+schema)
}

func dropPG(t *testing.T, dsn, schema string) {
	// Exec2 不做占位符改写，故此处直接拼接；
	// schema 由 randomSuffix 生成，仅含字母数字，无注入风险。
	// 先断开会话再删库，否则仍有连接的库无法删除。
	_, _ = db.Exec2(dsn, "pgsql",
		"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '"+schema+"'")
	_, _ = db.Exec2(dsn, "pgsql", "DROP DATABASE IF EXISTS "+schema)
}

// ---------- MySQL ----------

// TestCrossDBMySQL 在真实 MySQL 8 上跑通核心业务流程。
//
// 覆盖的关键方言点：
//   - 自增主键（AUTO_INCREMENT 而非 AUTOINCREMENT）
//   - INSERT 取 ID 走 LastInsertId 而非 RETURNING
//   - 布尔列以 0/1 存储
//   - 索引创建的幂等（MySQL 无 CREATE INDEX IF NOT EXISTS）
//   - 占位符保持 ?
//   - LIKE ... ESCAPE '!'
func TestCrossDBMySQL(t *testing.T) {
	dsn := os.Getenv("NP_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("跳过：未设置 NP_TEST_MYSQL_DSN")
	}
	e := newCrossDBEnv(t, "mysql", dsn)
	runCoreFlow(t, e, "mysql")
}

// ---------- PostgreSQL ----------

// TestCrossDBPostgres 在真实 PostgreSQL 16 上跑通核心业务流程。
//
// 覆盖的关键方言点：
//   - 自增主键用 IDENTITY
//   - INSERT/UPDATE 走 RETURNING
//   - 布尔列为真正的 BOOLEAN
//   - 占位符改写为 $n
func TestCrossDBPostgres(t *testing.T) {
	dsn := os.Getenv("NP_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("跳过：未设置 NP_TEST_PG_DSN")
	}
	e := newCrossDBEnv(t, "pgsql", dsn)
	runCoreFlow(t, e, "pgsql")
}

// runCoreFlow 是一组与方言无关的业务断言，三库共用。
func runCoreFlow(t *testing.T, e *testEnv, label string) {
	// 登录放在最前：后续子测试都依赖 e.token，
	// 用 t.Run 分组时若登录放在某个子测试内，其它子测试就拿不到 token。
	e.login(t, "admin", "test-password-123")

	var postID string
	var postSlug string

	t.Run(label+"/健康检查", func(t *testing.T) {
		if code, body := e.do(t, "GET", "/health", nil, false); code != 200 ||
			body["status"] != "ok" {
			t.Fatalf("健康检查异常: %d %v", code, body)
		}
	})

	t.Run(label+"/创建并读取文章", func(t *testing.T) {
		code, created := e.do(t, "POST", "/posts", map[string]any{
			"title": "跨库文章", "content": "正文", "summary": "摘要",
			"status": "published", "tags": []string{"跨库标签"},
		}, true)
		if code != http.StatusCreated {
			t.Fatalf("创建失败: %d %v", code, created)
		}
		id, _ := created["id"].(float64)
		if id <= 0 {
			t.Fatalf("应返回有效 ID，得到 %v", created["id"])
		}
		postID = itoa(int(id))
		postSlug, _ = created["slug"].(string)
		if postSlug == "" {
			t.Fatal("应返回 slug")
		}

		code, body := e.do(t, "GET", "/posts/"+postSlug, nil, false)
		if code != 200 {
			t.Fatalf("读取失败: %d", code)
		}
		if body["title"] != "跨库文章" || body["content"] != "正文" {
			t.Fatalf("内容不符: %v", body)
		}
		// 标签应已关联
		tags, _ := body["tags"].([]any)
		if len(tags) != 1 {
			t.Fatalf("标签未关联: %v", body["tags"])
		}
	})

	t.Run(label+"/更新文章", func(t *testing.T) {
		if code, _ := e.do(t, "PATCH", "/posts/"+postID, map[string]any{
			"summary": "改过的摘要", "sticky": true,
		}, true); code != 200 {
			t.Fatal("更新失败")
		}
		_, body := e.do(t, "GET", "/posts/"+postID, nil, true)
		if body["summary"] != "改过的摘要" {
			t.Fatalf("摘要未更新: %v", body["summary"])
		}
		// 布尔列应正确回读为 true
		if body["sticky"] != true {
			t.Fatalf("布尔列回读异常，sticky=%v", body["sticky"])
		}
	})

	t.Run(label+"/搜索含通配符", func(t *testing.T) {
		if code, _ := e.do(t, "POST", "/posts", map[string]any{
			"title": "50% off", "content": "x", "status": "published",
		}, true); code != http.StatusCreated {
			t.Fatal("创建失败")
		}
		_, body := e.do(t, "GET", "/posts?q="+urlEncode("50%"), nil, false)
		items, _ := body["items"].([]any)
		if len(items) != 1 {
			t.Fatalf("LIKE 转义异常，期望 1 条得到 %d", len(items))
		}
	})

	t.Run(label+"/分类与标签", func(t *testing.T) {
		code, cat := e.do(t, "POST", "/categories", map[string]any{
			"name": "跨库分类", "slug": "xdb-cat",
		}, true)
		if code != http.StatusCreated {
			t.Fatalf("创建分类失败: %d %v", code, cat)
		}
		if code, _ := e.do(t, "POST", "/categories", map[string]any{
			"name": "重复 slug", "slug": "xdb-cat",
		}, true); code != http.StatusConflict {
			t.Fatalf("重复 slug 应 409，得到 %d", code)
		}

		// 重复执行建表逻辑不应报错（索引幂等）
		if err := db.Migrate(e.db, e.cfg.DBType); err != nil {
			t.Fatalf("重复迁移应幂等: %v", err)
		}

		if code, _ := e.do(t, "POST", "/tags", map[string]any{
			"name": "跨库标签2",
		}, true); code != http.StatusCreated {
			t.Fatal("创建标签失败")
		}
		_, list := e.do(t, "GET", "/tags", nil, false)
		if items, _ := list["items"].([]any); len(items) < 2 {
			t.Fatalf("标签列表异常: %d", len(items))
		}
	})

	t.Run(label+"/评论与审核", func(t *testing.T) {
		code, res := e.do(t, "POST", "/comments", map[string]any{
			"postId": atoiOrZero(postID), "author": "访客", "content": "评论",
		}, false)
		if code != http.StatusCreated {
			t.Fatalf("提交评论失败: %d %v", code, res)
		}
		cid := int64(res["id"].(float64))

		_, pub := e.do(t, "GET", "/comments?postId="+postID, nil, false)
		if items, _ := pub["items"].([]any); len(items) != 0 {
			t.Fatalf("待审核评论不应公开: %d", len(items))
		}
		if code, _ := e.do(t, "PUT", "/comments/"+itoa64(cid), map[string]any{
			"status": "approved",
		}, true); code != 200 {
			t.Fatal("审核失败")
		}
		_, pub = e.do(t, "GET", "/comments?postId="+postID, nil, false)
		if items, _ := pub["items"].([]any); len(items) != 1 {
			t.Fatalf("审核后应公开 1 条: %d", len(items))
		}
	})

	t.Run(label+"/回收站", func(t *testing.T) {
		if code, _ := e.do(t, "POST", "/posts/"+postID+"/trash", nil, true); code != 200 {
			t.Fatal("移入回收站失败")
		}
		_, trash := e.do(t, "GET", "/posts?status=trash", nil, true)
		if items, _ := trash["items"].([]any); len(items) != 1 {
			t.Fatalf("回收站应有 1 条: %d", len(items))
		}
		if code, _ := e.do(t, "POST", "/posts/"+postID+"/restore", nil, true); code != 200 {
			t.Fatal("恢复失败")
		}
		if code, _ := e.do(t, "GET", "/posts/"+postID, nil, true); code != 200 {
			t.Fatal("恢复后不可访问")
		}
	})

	t.Run(label+"/用户与角色", func(t *testing.T) {
		if code, _ := e.do(t, "POST", "/users", map[string]any{
			"username": "xdb-author", "password": "xdb-pass-123", "role": "author",
		}, true); code != http.StatusCreated {
			t.Fatal("创建用户失败")
		}
		adminToken := e.token
		authorToken := e.login(t, "xdb-author", "xdb-pass-123")
		e.token = authorToken

		_, created := e.do(t, "POST", "/posts", map[string]any{
			"title": "作者的文章", "content": "x",
		}, true)
		aid := itoa(int(created["id"].(float64)))
		// 作者可编辑自己的
		if code, _ := e.do(t, "PATCH", "/posts/"+aid, map[string]any{
			"summary": "作者改了",
		}, true); code != 200 {
			t.Fatalf("作者应可编辑自己的文章，得到 %d", code)
		}
		// 不能编辑他人的
		if code, _ := e.do(t, "PATCH", "/posts/"+postID, map[string]any{
			"summary": "篡改",
		}, true); code != http.StatusForbidden {
			t.Fatalf("作者编辑他人文章应 403，得到 %d", code)
		}
		e.token = adminToken
	})

	t.Run(label+"/站点设置", func(t *testing.T) {
		if code, rb := e.do(t, "PUT", "/settings", map[string]any{
			"title": "跨库站点", "posts_per_page": 15,
		}, true); code != 200 {
			t.Fatalf("更新站点设置失败: %d %v", code, rb)
		}
		_, site := e.do(t, "GET", "/site", nil, false)
		if site["title"] != "跨库站点" {
			t.Fatalf("站点设置未生效: %v", site["title"])
		}
		if n, _ := site["posts_per_page"].(float64); int(n) != 15 {
			t.Fatalf("整数设置未生效: %v", site["posts_per_page"])
		}
		// 再写一次验证 upsert 幂等
		if code, _ := e.do(t, "PUT", "/settings", map[string]any{
			"title": "跨库站点2",
		}, true); code != 200 {
			t.Fatal("二次更新失败")
		}
	})

	t.Run(label+"/订阅与统计", func(t *testing.T) {
		for _, path := range []string{"/feed.xml", "/feed/atom.xml", "/sitemap.xml", "/robots.txt"} {
			if res := e.doRaw(t, "GET", path, nil, "", false); res.StatusCode != 200 {
				t.Fatalf("%s 应可用，得到 %d", path, res.StatusCode)
			}
		}
		code, stats := e.do(t, "GET", "/stats", nil, false)
		if code != 200 {
			t.Fatal("统计接口失败")
		}
		if n, _ := stats["posts"].(float64); n < 1 {
			t.Fatalf("统计异常: %v", stats)
		}
	})

	t.Run(label+"/归档", func(t *testing.T) {
		code, archive := e.do(t, "GET", "/archive", nil, false)
		if code != 200 {
			t.Fatal("归档失败")
		}
		if _, ok := archive["authors"].([]any); !ok {
			t.Fatalf("归档缺少作者列表: %v", archive)
		}
	})
}

func atoiOrZero(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}
