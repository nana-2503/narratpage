package seed

import (
	"database/sql"
	"log"

	"narratpage/internal/auth"
	"narratpage/internal/config"
	"narratpage/internal/db"
	"narratpage/internal/dialect"
	"narratpage/internal/repo"
)

// Run 幂等种子：仅在对应表为空时写入示例内容。
//
// 注意：所有语句都必须经 db 包执行以适配占位符。早期实现直接用
// `?` 拼接，在 PostgreSQL 下会因占位符不匹配而启动失败。
func Run(database *sql.DB, cfg config.Config) error {
	now := db.NowISO()
	dbType := cfg.DBType

	var userCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		return err
	}
	if userCount == 0 {
		hash, err := auth.HashPassword(cfg.AdminPassword)
		if err != nil {
			return err
		}
		_, err = db.Insert(database, dbType, "users",
			[]string{"username", "display_name", "password_hash", "role", "active", "created_at"},
			[]any{cfg.AdminUsername, cfg.AdminUsername, hash, "admin",
				dialect.Dialect(dbType).QuoteBool(true), now})
		if err != nil {
			return err
		}
		log.Printf("[seed] 默认管理员已创建: %s", cfg.AdminUsername)
		if cfg.AdminPassword == "admin123" {
			log.Print("[seed] 警告: 正在使用默认密码 admin123，请尽快修改")
		}
	}

	// 分类
	var catCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM categories`).Scan(&catCount); err != nil {
		return err
	}
	if catCount == 0 {
		for i, c := range seedCategories {
			if _, err := db.Insert(database, dbType, "categories",
				[]string{"name", "slug", "description", "is_page", "position", "created_at"},
				[]any{c.name, c.slug, c.description, 0, i, now}); err != nil {
				return err
			}
		}
		log.Print("[seed] 示例分类已写入")
	}

	// 标签
	var tagCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM tags`).Scan(&tagCount); err != nil {
		return err
	}
	if tagCount == 0 {
		for _, t := range seedTags {
			// slug 统一走 Slugify，保证与后续 ResolveTagIDs 的结果一致
			slug := t.slug
			if slug == "" {
				slug = repo.Slugify(t.name, "tag")
			}
			if _, err := db.Insert(database, dbType, "tags",
				[]string{"name", "slug", "description", "created_at"},
				[]any{t.name, slug, t.description, now}); err != nil {
				return err
			}
		}
	}

	// 文章
	var postCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM posts`).Scan(&postCount); err != nil {
		return err
	}
	if postCount == 0 {
		tax := repo.NewTaxonomy(database, dbType)
		adminID := firstAdminID(database)
		authorID := any(nil)
		if adminID > 0 {
			authorID = adminID
		}

		for _, p := range seedPosts {
			catID, err := categoryID(database, dbType, p.categorySlug)
			if err != nil {
				return err
			}
			id, err := db.Insert(database, dbType, "posts",
				[]string{"title", "slug", "summary", "content", "status", "type",
					"category_id", "author_id", "created_at", "updated_at", "published_at"},
				[]any{p.title, p.slug, p.summary, p.content, "published", "post",
					catID, authorID, now, now, now})
			if err != nil {
				return err
			}
			// 关联标签
			for _, tagName := range p.tagNames {
				ids, err := tax.ResolveTagIDs([]string{tagName})
				if err != nil {
					continue
				}
				_ = tax.AddPostTag(id, ids[0])
			}
		}
		log.Print("[seed] 示例文章已写入")
	}

	// 页面
	var pageCount int
	if err := db.QueryRow(database, dbType,
		`SELECT COUNT(*) FROM posts WHERE type = ?`, "page").Scan(&pageCount); err != nil {
		return err
	}
	if pageCount == 0 {
		for i, p := range seedPages {
			if _, err := db.Insert(database, dbType, "posts",
				[]string{"title", "slug", "content", "status", "type",
					"menu_order", "created_at", "updated_at", "published_at"},
				[]any{p.title, p.slug, p.content, "published", "page",
					i, now, now, now}); err != nil {
				return err
			}
		}
		log.Print("[seed] 示例页面已写入")
	}

	// 评论
	var commentCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM comments`).Scan(&commentCount); err != nil {
		return err
	}
	if commentCount == 0 {
		for _, c := range seedComments {
			postID, err := postIDBySlug(database, dbType, c.postSlug)
			if err != nil || postID == 0 {
				continue
			}
			if _, err := db.Insert(database, dbType, "comments",
				[]string{"post_id", "author", "content", "status", "created_at"},
				[]any{postID, c.author, c.content, c.status, now}); err != nil {
				return err
			}
		}
	}
	return nil
}

func firstAdminID(database *sql.DB) int64 {
	var id int64
	_ = database.QueryRow("SELECT id FROM users ORDER BY id ASC LIMIT 1").Scan(&id)
	return id
}

func categoryID(database *sql.DB, dbType, slug string) (any, error) {
	var id int64
	err := db.QueryRow(database, dbType,
		"SELECT id FROM categories WHERE slug = ?", slug).Scan(&id)
	if err != nil {
		return nil, nil // 分类不存在时留空
	}
	return id, nil
}

func postIDBySlug(database *sql.DB, dbType, slug string) (int64, error) {
	var id int64
	err := db.QueryRow(database, dbType,
		"SELECT id FROM posts WHERE slug = ?", slug).Scan(&id)
	return id, err
}
