package seed

import (
	"database/sql"
	"log"

	"narratpage/internal/auth"
	"narratpage/internal/config"
	"narratpage/internal/db"
)

type seedCategory struct {
	name string
	slug string
}

type seedPost struct {
	title        string
	slug         string
	summary      string
	content      string
	categorySlug string
}

var seedCategories = []seedCategory{
	{"技术", "tech"},
	{"生活", "life"},
	{"随笔", "notes"},
}

var seedPosts = []seedPost{
	{
		title:   "为什么选择 SQLite 作为博客数据库",
		slug:    "why-sqlite-for-blog",
		summary: "单文件、零运维、读多写少的场景下，SQLite 往往是博客系统最务实的后端存储选择。",
		content: `## 单文件的魅力

 SQLite 把整个数据库放进一个文件，备份就是拷贝文件，迁移就是挂载卷。对个人博客这种读多写少的场景，它没有连接池调优，没有慢查询监控，运维成本趋近于零。

 ## 写放大不是问题

 博客的写入集中在作者本人：一天几篇文章、几十条评论。WAL 模式下读写不互斥，这个量级绰绰有余。

 ## 什么时候该换

 当出现多实例部署、写入并发超过百 QPS、或需要全文检索排名时，再迁移到 PostgreSQL 也不迟——届时数据导出只是一条 ` + "`.dump`" + ` 的事。`,
		categorySlug: "tech",
	},
	{
		title:   "Markdown 写作流水线",
		slug:    "markdown-writing-pipeline",
		summary: "从草稿到发布，一套基于纯文本的写作流程：本地编辑、版本管理、一键发布。",
		content: `## 纯文本优先

 文章用 Markdown 书写，Git 做版本管理，发布只是推送到仓库。不依赖任何专有格式，十年后依然打得开。

 ## 结构即大纲

 先列标题再填内容。H2 是章节，H3 是论点，写作前大纲先成立，文章就不会散。`,
		categorySlug: "notes",
	},
	{
		title:   "重新开始写博客",
		slug:    "restart-blogging",
		summary: "清理掉收藏夹里的教程，关掉永远在配置的编辑器，先把第一篇发出去。",
		content: `## 完成比完美重要

 搭博客的真正风险不是选错技术栈，而是把全部时间花在配置环境上。先让最小版本跑起来，剩下的在路上迭代。

 ## 写给未来的自己

 保持记录的习惯。三年后回看，这些文字就是时间存在的证据。`,
		categorySlug: "life",
	},
}

type seedComment struct {
	postSlug string
	author   string
	content  string
	status   string
}

var seedComments = []seedComment{
	{"why-sqlite-for-blog", "读者甲", "写得很好，已经准备把博客迁到 SQLite 了。", "approved"},
	{"markdown-writing-pipeline", "路过乙", "大纲先行这点很认同。", "approved"},
	{"restart-blogging", "游客丙", "占位待审核的一条评论。", "pending"},
}

// Run 幂等种子：仅在对应表为空时写入
func Run(database *sql.DB, cfg config.Config) error {
	now := db.NowISO()

	var userCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		return err
	}
	if userCount == 0 {
		hash, err := auth.HashPassword(cfg.AdminPassword)
		if err != nil {
			return err
		}
		_, err = database.Exec(
			`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
			cfg.AdminUsername, hash, now,
		)
		if err != nil {
			return err
		}
		log.Printf("[seed] 默认管理员已创建: %s", cfg.AdminUsername)
		if cfg.AdminPassword == "admin123" {
			log.Printf("[seed] 警告: 正在使用默认密码 admin123，请尽快通过 ADMIN_PASSWORD 环境变量修改")
		}
	}

	var catCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM categories`).Scan(&catCount); err != nil {
		return err
	}
	if catCount == 0 {
		for _, c := range seedCategories {
			if _, err := database.Exec(
				`INSERT INTO categories (name, slug, created_at) VALUES (?, ?, ?)`,
				c.name, c.slug, now,
			); err != nil {
				return err
			}
		}
	}

	var postCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM posts`).Scan(&postCount); err != nil {
		return err
	}
	if postCount == 0 {
		for _, p := range seedPosts {
			if _, err := database.Exec(
				`INSERT INTO posts (title, slug, summary, content, category_id, status, published_at, created_at, updated_at)
				 VALUES (?, ?, ?, ?, (SELECT id FROM categories WHERE slug = ?), 'published', ?, ?, ?)`,
				p.title, p.slug, p.summary, p.content, p.categorySlug, now, now, now,
			); err != nil {
				return err
			}
		}
		log.Printf("[seed] 示例文章已写入")
	}

	var commentCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM comments`).Scan(&commentCount); err != nil {
		return err
	}
	if commentCount == 0 {
		for _, c := range seedComments {
			if _, err := database.Exec(
				`INSERT INTO comments (post_id, author, content, status, created_at)
				 VALUES ((SELECT id FROM posts WHERE slug = ?), ?, ?, ?, ?)`,
				c.postSlug, c.author, c.content, c.status, now,
			); err != nil {
				return err
			}
		}
	}
	return nil
}
