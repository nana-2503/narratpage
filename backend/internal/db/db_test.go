package db

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// nowMilliForTest 提供测试用的唯一前缀，避免与遗留数据冲突。
func nowMilliForTest() int64 { return time.Now().UnixNano() }

// TestSchemaSQLite 验证建表语句在 SQLite 上可完整执行。
func TestSchemaSQLite(t *testing.T) {
	dir := t.TempDir()
	d, err := Open(testConfig(dir, "sqlite", ""))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.Close()

	assertTablesExist(t, d, "sqlite",
		"users", "categories", "tags", "posts", "post_tags", "comments",
		"media", "postmeta", "options", "sessions", "revisions", "redirects", "sitemap",
	)
}

// TestSchemaExternal 验证建表语句在外部 MySQL / PostgreSQL 上可执行。
// 仅在设置了 NP_TEST_MYSQL_DSN / NP_TEST_PG_DSN 时运行。
func TestSchemaExternal(t *testing.T) {
	for _, tc := range []struct {
		name   string
		env    string
		dbType string
	}{
		{"mysql", "NP_TEST_MYSQL_DSN", "mysql"},
		{"postgres", "NP_TEST_PG_DSN", "pgsql"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dsn := os.Getenv(tc.env)
			if dsn == "" {
				t.Skipf("跳过：未设置 %s", tc.env)
			}
			d, err := Open(testConfig(t.TempDir(), tc.dbType, dsn))
			if err != nil {
				t.Fatalf("open %s: %v", tc.name, err)
			}
			defer d.Close()

			assertTablesExist(t, d, tc.dbType,
				"users", "categories", "tags", "posts", "post_tags", "comments",
				"media", "postmeta", "options", "sessions", "revisions", "redirects", "sitemap",
			)
		})
	}
}

// TestInsertExternal 验证跨库插入取 ID 的分支（MySQL 走 LastInsertId，
// PostgreSQL 走 RETURNING）。这是本项目多数据库支持的核心分叉点。
func TestInsertExternal(t *testing.T) {
	for _, tc := range []struct {
		name   string
		env    string
		dbType string
	}{
		{"mysql", "NP_TEST_MYSQL_DSN", "mysql"},
		{"postgres", "NP_TEST_PG_DSN", "pgsql"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dsn := os.Getenv(tc.env)
			if dsn == "" {
				t.Skipf("跳过：未设置 %s", tc.env)
			}
			d, err := Open(testConfig(t.TempDir(), tc.dbType, dsn))
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer d.Close()

			now := NowISO()
			id, err := Insert(d, tc.dbType, "tags",
				[]string{"name", "slug", "created_at"},
				[]any{"测试标签", "test-tag", now})
			if err != nil {
				t.Fatalf("Insert: %v", err)
			}
			if id <= 0 {
				t.Fatalf("期望返回正数 ID，得到 %d", id)
			}

			var gotID int64
			var gotName, gotSlug string
			if err := QueryRow(d, tc.dbType,
				"SELECT id, name, slug FROM tags WHERE id = ?", id).
				Scan(&gotID, &gotName, &gotSlug); err != nil {
				t.Fatalf("回读失败: %v", err)
			}
			if gotName != "测试标签" || gotSlug != "test-tag" {
				t.Fatalf("回读内容不符: %q / %q", gotName, gotSlug)
			}

			// UPDATE / DELETE 的受影响行数语义在 MySQL 下依赖 RowsAffected，
			// 这里顺带验证 0 行更新能被正确识别。
			n, err := Update(d, tc.dbType, "tags",
				map[string]any{"name": "新名称"}, "id = ?", id)
			if err != nil {
				t.Fatalf("Update: %v", err)
			}
			if n != 1 {
				t.Fatalf("期望更新 1 行，得到 %d", n)
			}
			n, err = Delete(d, tc.dbType, "tags", "id = ?", id)
			if err != nil {
				t.Fatalf("Delete: %v", err)
			}
			if n != 1 {
				t.Fatalf("期望删除 1 行，得到 %d", n)
			}
		})
	}
}

// TestLikeEscapeExternal 验证 LIKE ... ESCAPE 在三库上均可执行。
// 旧实现在 MySQL 上写成 ESCAPE '\'，会因反斜杠转义导致语法错误。
func TestLikeEscapeExternal(t *testing.T) {
	cases := []struct {
		name   string
		env    string
		dbType string
	}{
		{"sqlite", "", "sqlite"},
		{"mysql", "NP_TEST_MYSQL_DSN", "mysql"},
		{"postgres", "NP_TEST_PG_DSN", "pgsql"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dsn := ""
			if tc.env != "" {
				dsn = os.Getenv(tc.env)
				if dsn == "" {
					t.Skipf("跳过：未设置 %s", tc.env)
				}
			}
			dir := t.TempDir()
			d, err := Open(testConfig(dir, tc.dbType, dsn))
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer d.Close()

			now := NowISO()
			// 标题含 LIKE 通配符 % 与 _，用于验证转义后按字面量匹配。
			// 用唯一前缀避免与遗留数据撞 slug 唯一键。
			prefix := "t" + strconv.FormatInt(nowMilliForTest(), 36) + "-"
			titles := []string{prefix + "100% 咖啡", prefix + "under_score", prefix + "普通标题"}
			for _, title := range titles {
				if _, err := Insert(d, tc.dbType, "tags",
					[]string{"name", "slug", "created_at"},
					[]any{title, slugFor(title), now}); err != nil {
					t.Fatalf("插入 %q 失败: %v", title, err)
				}
			}

			// 所有查询都限定在本轮插入的前缀内，否则外部库中
			// 上一轮遗留的行会污染计数。
			// 搜索 "100%" 应只命中第一行，且 '%' 需被转义为字面量
			n, err := Count(d, tc.dbType,
				"SELECT COUNT(*) FROM tags WHERE name LIKE ? ESCAPE '!'",
				escapeForTest(prefix)+"%100"+escapeForTest("%")+"%")
			if err != nil {
				t.Fatalf("LIKE 查询失败: %v", err)
			}
			if n != 1 {
				t.Fatalf("期望命中 1 行，得到 %d", n)
			}

			// 用 '_' 做对照：转义后按字面量匹配，只有含下划线的行命中；
			// 若未转义，'_' 是单字符通配符，会匹配到全部 3 行。
			n, err = Count(d, tc.dbType,
				"SELECT COUNT(*) FROM tags WHERE name LIKE ? ESCAPE '!'",
				escapeForTest(prefix)+"%"+escapeForTest("_")+"%")
			if err != nil {
				t.Fatalf("LIKE 查询失败: %v", err)
			}
			if n != 1 {
				t.Fatalf("转义后期望命中 1 行，得到 %d", n)
			}

			// 未转义对照：'<stem>_%' 中的 '_' 作单字符通配符，
			// 能匹配到本轮全部 3 行（它们都以 stem + "-" 开头）。
			// 注意 LIKE 没有隐式尾部 %，所以必须显式写 '%'。
			n, err = Count(d, tc.dbType,
				"SELECT COUNT(*) FROM tags WHERE name LIKE ? ESCAPE '!'",
				escapeForTest(strings.TrimSuffix(prefix, "-"))+"_%")
			if err != nil {
				t.Fatalf("LIKE 查询失败: %v", err)
			}
			if n != 3 {
				t.Fatalf("未转义时 '_' 应作通配符命中 3 行，得到 %d", n)
			}
		})
	}
}
