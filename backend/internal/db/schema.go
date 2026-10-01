package db

import (
	"narratpage/internal/dialect"
)

// SchemaFor 返回当前数据库的完整建表结构。
//
// 设计取舍：
//   - 时间统一以 UTC ISO-8601 文本存储，三库一致且可字典序排序；
//   - 有界文本用 Varchar（MySQL 的 TEXT 不允许 DEFAULT），无界用 TEXT；
//   - 外键一律声明，删分类时文章落为未分类（SET NULL），删文章时评论级联。
func SchemaFor(dbType string) []string {
	d := dialect.Dialect(dbType)

	users := dialect.Table{
		Name: "users",
		Columns: []dialect.Column{
			dialect.ID(d, "id"),
			{Name: "username", Type: d.Varchar(64), NotNull: true, Unique: true},
			{Name: "email", Type: d.Varchar(255), NotNull: true, Default: "''"},
			{Name: "display_name", Type: d.Varchar(64), NotNull: true, Default: "''"},
			{Name: "password_hash", Type: d.Varchar(255), NotNull: true},
			// admin | editor | author | contributor | subscriber
			dialect.EnumCol(d, "role", "admin", "admin", "editor", "author", "contributor", "subscriber"),
			{Name: "bio", Type: d.Text()},
			{Name: "avatar_url", Type: d.Varchar(512), NotNull: true, Default: "''"},
			dialect.BoolCol(d, "active"),
			dialect.TSCol(d, "created_at"),
			dialect.TSCol(d, "last_login_at"),
		},
		Indexes: []dialect.Index{
			{Name: "idx_users_role", Columns: []string{"role"}},
		},
	}

	categories := dialect.Table{
		Name: "categories",
		Columns: []dialect.Column{
			dialect.ID(d, "id"),
			{Name: "name", Type: d.Varchar(64), NotNull: true},
			{Name: "slug", Type: d.Varchar(128), NotNull: true, Unique: true},
			{Name: "description", Type: d.Text()},
			// 0 = 文章，1 = 页面（页面也归入分类体系以便统一导航）
			dialect.IntCol(d, "is_page"),
			dialect.IntCol(d, "position"),
			dialect.TSCol(d, "created_at"),
		},
	}

	tags := dialect.Table{
		Name: "tags",
		Columns: []dialect.Column{
			dialect.ID(d, "id"),
			{Name: "name", Type: d.Varchar(64), NotNull: true},
			{Name: "slug", Type: d.Varchar(128), NotNull: true, Unique: true},
			{Name: "description", Type: d.Text()},
			dialect.TSCol(d, "created_at"),
		},
	}

	posts := dialect.Table{
		Name: "posts",
		Columns: []dialect.Column{
			dialect.ID(d, "id"),
			{Name: "title", Type: d.Varchar(200), NotNull: true},
			{Name: "slug", Type: d.Varchar(200), NotNull: true, Unique: true},
			{Name: "summary", Type: d.Varchar(500), NotNull: true, Default: "''"},
			dialect.TextCol(d, "content"),
			{Name: "cover_url", Type: d.Varchar(512), NotNull: true, Default: "''"},
			dialect.FKCol(d, "category_id", "categories", "id", "SET NULL"),
			dialect.EnumCol(d, "status", "draft", "draft", "published", "pending", "private", "trash"),
			// 文章 / 页面 / 附件
			dialect.EnumCol(d, "type", "post", "post", "page", "attachment"),
			// 置顶：列表排序时优先
			dialect.BoolCol(d, "sticky"),
			// 访问密码（留空表示不保护）
			{Name: "password", Type: d.Varchar(128), NotNull: true, Default: "''"},
			// 菜单顺序：页面与文章列表按此升序
			dialect.IntCol(d, "menu_order"),
			dialect.FKCol(d, "author_id", "users", "id", "SET NULL"),
			dialect.IntCol(d, "views"),
			{Name: "template", Type: d.Varchar(64), NotNull: true, Default: "''"},
			dialect.TSCol(d, "created_at"),
			dialect.TSCol(d, "updated_at"),
			dialect.NullableTSCol(d, "published_at"),
			// 定时发布：晚于当前时间则公开列表不可见
			dialect.NullableTSCol(d, "scheduled_at"),
			dialect.NullableTSCol(d, "deleted_at"),
		},
		Indexes: []dialect.Index{
			{Name: "idx_posts_status", Columns: []string{"status", "published_at"}},
			{Name: "idx_posts_category", Columns: []string{"category_id"}},
			{Name: "idx_posts_type", Columns: []string{"type", "status"}},
			{Name: "idx_posts_author", Columns: []string{"author_id"}},
			{Name: "idx_posts_deleted", Columns: []string{"deleted_at"}},
		},
	}

	postTags := dialect.Table{
		Name: "post_tags",
		Columns: []dialect.Column{
			dialect.ID(d, "id"),
			dialect.FKCol(d, "post_id", "posts", "id", "CASCADE"),
			dialect.FKCol(d, "tag_id", "tags", "id", "CASCADE"),
		},
		Indexes: []dialect.Index{
			{Name: "idx_post_tags_post", Columns: []string{"post_id"}},
			{Name: "idx_post_tags_tag", Columns: []string{"tag_id"}},
			{Name: "uq_post_tags", Columns: []string{"post_id", "tag_id"}, Unique: true},
		},
	}

	comments := dialect.Table{
		Name: "comments",
		Columns: []dialect.Column{
			dialect.ID(d, "id"),
			dialect.FKCol(d, "post_id", "posts", "id", "CASCADE"),
			// 父评论：支持二级嵌套
			dialect.FKCol(d, "parent_id", "comments", "id", "CASCADE"),
			dialect.FKCol(d, "author_id", "users", "id", "SET NULL"),
			{Name: "author", Type: d.Varchar(64), NotNull: true},
			{Name: "email", Type: d.Varchar(255), NotNull: true, Default: "''"},
			{Name: "website", Type: d.Varchar(255), NotNull: true, Default: "''"},
			dialect.TextCol(d, "content"),
			{Name: "ip_hash", Type: d.Varchar(64), NotNull: true, Default: "''"},
			dialect.EnumCol(d, "status", "pending", "pending", "approved", "rejected", "spam"),
			dialect.BoolCol(d, "is_pingback"),
			dialect.TSCol(d, "created_at"),
		},
		Indexes: []dialect.Index{
			{Name: "idx_comments_post", Columns: []string{"post_id", "status"}},
			{Name: "idx_comments_parent", Columns: []string{"parent_id"}},
		},
	}

	media := dialect.Table{
		Name: "media",
		Columns: []dialect.Column{
			dialect.ID(d, "id"),
			{Name: "filename", Type: d.Varchar(255), NotNull: true},
			{Name: "url", Type: d.Varchar(512), NotNull: true},
			{Name: "mime", Type: d.Varchar(128), NotNull: true, Default: "''"},
			dialect.IntCol(d, "size"),
			{Name: "title", Type: d.Varchar(200), NotNull: true, Default: "''"},
			{Name: "alt_text", Type: d.Varchar(255), NotNull: true, Default: "''"},
			{Name: "caption", Type: d.Text()},
			// 宽高用于前端预留比例、生成 srcset
			dialect.IntCol(d, "width"),
			dialect.IntCol(d, "height"),
			dialect.FKCol(d, "uploaded_by", "users", "id", "SET NULL"),
			dialect.TSCol(d, "created_at"),
		},
		Indexes: []dialect.Index{
			{Name: "idx_media_created", Columns: []string{"created_at"}},
		},
	}

	postMeta := dialect.Table{
		Name: "postmeta",
		Columns: []dialect.Column{
			dialect.ID(d, "id"),
			dialect.FKCol(d, "post_id", "posts", "id", "CASCADE"),
			{Name: "meta_key", Type: d.Varchar(128), NotNull: true},
			dialect.TextCol(d, "meta_value"),
		},
		Indexes: []dialect.Index{
			{Name: "idx_postmeta_key", Columns: []string{"meta_key"}},
			{Name: "uq_postmeta", Columns: []string{"post_id", "meta_key"}, Unique: true},
		},
	}

	// options 承载站点级设置（键值对），替代散落的环境变量
	options := dialect.Table{
		Name: "options",
		Columns: []dialect.Column{
			dialect.ID(d, "id"),
			{Name: "option_key", Type: d.Varchar(128), NotNull: true, Unique: true},
			dialect.TextCol(d, "option_value"),
			dialect.TSCol(d, "updated_at"),
		},
	}

	// sessions 用于「登出所有设备」与在线设备列表；JWT 本身无状态
	sessions := dialect.Table{
		Name: "sessions",
		Columns: []dialect.Column{
			dialect.ID(d, "id"),
			{Name: "token_id", Type: d.Varchar(64), NotNull: true, Unique: true},
			dialect.FKCol(d, "user_id", "users", "id", "CASCADE"),
			{Name: "ip", Type: d.Varchar(64), NotNull: true, Default: "''"},
			{Name: "user_agent", Type: d.Varchar(512), NotNull: true, Default: "''"},
			dialect.TSCol(d, "created_at"),
			dialect.TSCol(d, "last_seen_at"),
		},
		Indexes: []dialect.Index{
			{Name: "idx_sessions_user", Columns: []string{"user_id"}},
		},
	}

	// revisions 保存文章历史版本，误编辑可回滚
	revisions := dialect.Table{
		Name: "revisions",
		Columns: []dialect.Column{
			dialect.ID(d, "id"),
			dialect.FKCol(d, "post_id", "posts", "id", "CASCADE"),
			dialect.FKCol(d, "author_id", "users", "id", "SET NULL"),
			{Name: "title", Type: d.Varchar(200), NotNull: true, Default: "''"},
			dialect.TextCol(d, "content"),
			dialect.TextCol(d, "excerpt"),
			dialect.TSCol(d, "created_at"),
		},
		Indexes: []dialect.Index{
			{Name: "idx_revisions_post", Columns: []string{"post_id", "created_at"}},
		},
	}

	// redirects 管理旧链接跳转，保住外链与 SEO
	redirects := dialect.Table{
		Name: "redirects",
		Columns: []dialect.Column{
			dialect.ID(d, "id"),
			{Name: "from_path", Type: d.Varchar(255), NotNull: true, Unique: true},
			{Name: "to_path", Type: d.Varchar(255), NotNull: true},
			dialect.IntCol(d, "hits"),
			dialect.BoolCol(d, "enabled"),
			dialect.TSCol(d, "created_at"),
		},
	}

	// 站点地图条目：支持 post/page/分类/标签/作者等任意归档
	sitemap := dialect.Table{
		Name: "sitemap",
		Columns: []dialect.Column{
			dialect.ID(d, "id"),
			{Name: "url", Type: d.Varchar(512), NotNull: true, Unique: true},
			{Name: "kind", Type: d.Varchar(32), NotNull: true},
			{Name: "object_id", Type: d.BigInt(), NotNull: true, Default: "0"},
			{Name: "lastmod", Type: d.Varchar(32), NotNull: true, Default: "''"},
			{Name: "priority", Type: d.Varchar(8), NotNull: true, Default: "'0.5'"},
			{Name: "changefreq", Type: d.Varchar(16), NotNull: true, Default: "'weekly'"},
		},
	}

	schema := dialect.Schema{Tables: []dialect.Table{
		users, categories, tags, posts, postTags, comments,
		media, postMeta, options, sessions, revisions, redirects, sitemap,
	}}
	return schema.Statements(d)
}
