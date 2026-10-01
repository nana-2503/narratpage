package api

import (
	"encoding/xml"
	"net/http"
	"strconv"
	"strings"
	"time"

	"narratpage/internal/config"
	"narratpage/internal/db"
	"narratpage/internal/httpx"
	"narratpage/internal/models"
	"narratpage/internal/repo"
)

// ---------- 归档 ----------

// archiveHandler GET /api/archive
// 返回按年/月聚合的文章数与分类分布，供归档页与侧栏使用。
func archiveHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		viewer := viewerFrom(r)

		years, err := deps.Posts.CountByYear(viewer)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		months, err := deps.Posts.CountByMonth(viewer, 24)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		categories, err := deps.Taxonomy.ListCategories(0, viewer)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		tags, err := deps.Taxonomy.ListTags("")
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}

		// 贡献者列表：按文章数排序
		authors, err := listAuthors(deps)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}

		total, _ := db.Count(deps.DB, deps.DBType,
			"SELECT COUNT(*) FROM posts WHERE status = 'published' AND deleted_at IS NULL")

		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"total":      total,
			"years":      years,
			"months":     months,
			"categories": categories,
			"tags":       tags,
			"authors":    authors,
		})
	}
}

// authorStat 贡献者统计。
type authorStat struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	PostCount   int    `json:"post_count"`
}

func listAuthors(deps Deps) ([]authorStat, error) {
	rows, err := db.Query(deps.DB, deps.DBType,
		`SELECT u.id, u.username, u.display_name, u.avatar_url, COUNT(p.id)
		 FROM users u JOIN posts p ON p.author_id = u.id
		 WHERE p.status = 'published' AND p.deleted_at IS NULL AND p.type = 'post'
		 GROUP BY u.id, u.username, u.display_name, u.avatar_url
		 ORDER BY COUNT(p.id) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []authorStat{}
	for rows.Next() {
		var a authorStat
		if err := rows.Scan(&a.ID, &a.Username, &a.DisplayName, &a.AvatarURL, &a.PostCount); err != nil {
			return nil, err
		}
		if a.DisplayName == "" {
			a.DisplayName = a.Username
		}
		out = append(out, a)
	}
	return out, nil
}

// ---------- 搜索 ----------

// searchHandler GET /api/search?q=
// 跨文章、页面、标签的聚合搜索，供命令面板与搜索页使用。
func searchHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if q == "" {
			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"results": []any{}, "total": 0,
			})
			return
		}
		viewer := viewerFrom(r)

		// 内容
		posts, err := deps.Posts.ListPosts(repo.PostQuery{
			Search:   q,
			Page:     1,
			PageSize: 20,
			Order:    "desc",
		}, viewer)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		// 标签
		tags, err := deps.Taxonomy.ListTags(q)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		// 分类
		categories, err := deps.Taxonomy.ListCategories(0, viewer)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		var matchedCats []models.Category
		lower := strings.ToLower(q)
		for _, c := range categories {
			if strings.Contains(strings.ToLower(c.Name), lower) ||
				strings.Contains(strings.ToLower(c.Slug), lower) {
				matchedCats = append(matchedCats, c)
			}
		}

		total := len(posts.Items) + len(tags) + len(matchedCats)
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"results":    posts.Items,
			"tags":       tags,
			"categories": matchedCats,
			"total":      total,
		})
	}
}

// ---------- 站点地图 ----------

type urlEntry struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod,omitempty"`
	ChangeFreq string `xml:"changefreq,omitempty"`
	Priority   string `xml:"priority,omitempty"`
}

type urlSet struct {
	XMLName xml.Name   `xml:"urlset"`
	Xmlns   string     `xml:"xmlns,attr"`
	URLs    []urlEntry `xml:"url"`
}

// sitemapHandler GET /api/sitemap.xml
func sitemapHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := siteURL(deps, config.Config{})
		viewer := repo.PublicViewer()
		now := time.Now().UTC().Format("2006-01-02")

		set := urlSet{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9"}
		set.URLs = append(set.URLs, urlEntry{
			Loc: base + "/", LastMod: now, ChangeFreq: "daily", Priority: "1.0",
		})

		posts, err := deps.Posts.ListPosts(repo.PostQuery{
			Type: models.TypePost, Page: 1, PageSize: 5000, Order: "desc",
		}, viewer)
		if err == nil {
			for _, p := range posts.Items {
				lastmod := models.ParseDate(models.Deref(p.PublishedAt))
				lm := now
				if !lastmod.IsZero() {
					lm = lastmod.Format("2006-01-02")
				}
				set.URLs = append(set.URLs, urlEntry{
					Loc: base + "/post/" + p.Slug, LastMod: lm,
					ChangeFreq: "monthly", Priority: "0.8",
				})
			}
		}

		pages, err := deps.Posts.ListPosts(repo.PostQuery{
			Type: models.TypePage, Page: 1, PageSize: 500, Order: "meta",
		}, viewer)
		if err == nil {
			for _, p := range pages.Items {
				set.URLs = append(set.URLs, urlEntry{
					Loc: base + "/page/" + p.Slug, LastMod: now,
					ChangeFreq: "monthly", Priority: "0.6",
				})
			}
		}

		cats, err := deps.Taxonomy.ListCategories(0, viewer)
		if err == nil {
			for _, c := range cats {
				if c.PostCount == 0 {
					continue
				}
				set.URLs = append(set.URLs, urlEntry{
					Loc: base + "/category/" + c.Slug, ChangeFreq: "weekly", Priority: "0.5",
				})
			}
		}

		tags, err := deps.Taxonomy.ListTags("")
		if err == nil {
			for _, t := range tags {
				if t.PostCount == 0 {
					continue
				}
				set.URLs = append(set.URLs, urlEntry{
					Loc: base + "/tag/" + t.Slug, ChangeFreq: "weekly", Priority: "0.4",
				})
			}
		}

		out, _ := xml.MarshalIndent(set, "", "  ")
		w.Header().Set("content-type", "application/xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(xml.Header))
		_, _ = w.Write(out)
	}
}

// robotsHandler GET /api/robots.txt
func robotsHandler(deps Deps, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := siteURL(deps, cfg)
		var b strings.Builder
		b.WriteString("User-agent: *\n")
		// 后台与接口无需抓取，省下的抓取预算留给内容
		b.WriteString("Disallow: /admin\n")
		b.WriteString("Disallow: /api/\n")
		b.WriteString("Allow: /api/feed.xml\n")
		b.WriteString("Allow: /api/sitemap.xml\n")
		b.WriteString("Sitemap: " + base + "/api/sitemap.xml\n")
		w.Header().Set("content-type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(b.String()))
	}
}

// ---------- 统计 ----------

// statsHandler GET /api/stats
func statsHandler(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		site := deps.Options.Stats()

		var views int
		_ = db.QueryRow(deps.DB, deps.DBType,
			"SELECT COALESCE(SUM(views), 0) FROM posts WHERE deleted_at IS NULL").Scan(&views)

		roleCounts, _ := deps.Users.CountByRole()
		mediaCount, mediaSize := deps.Media.Stats()

		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"posts":           site.PostCount,
			"pages":           site.PageCount,
			"comments":        site.CommentCount,
			"tags":            site.TagCount,
			"views":           views,
			"users":           roleCounts,
			"media":           map[string]any{"count": mediaCount, "size": mediaSize},
			"pendingComments": deps.Comments.CountPending(),
		})
	}
}

// ---------- 站点设置 ----------

// siteOptions GET /api/site
// 公开可读，供前端渲染站名、导航与功能开关。
func siteOptions(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		site := deps.Options.Site()
		httpx.WriteJSON(w, http.StatusOK, site)
	}
}

// siteSettings GET /api/settings — 后台可见统计
func siteSettings(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, deps.Options.Stats())
	}
}

// updateSiteSettings PUT /api/settings
func updateSiteSettings(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var site models.SiteOptions
		if err := httpx.DecodeJSON(w, r, &site); err != nil {
			return
		}
		if msg := validateSite(&site); msg != "" {
			badRequest(w, msg)
			return
		}
		if site.PostsPerPage < 1 || site.PostsPerPage > 100 {
			site.PostsPerPage = 10
		}
		if len(site.Links) > 50 {
			badRequest(w, "站点链接最多 50 条")
			return
		}
		if err := deps.Options.SaveSite(site); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, deps.Options.Stats())
	}
}

func validateSite(site *models.SiteOptions) string {
	if site.Title == "" {
		return "站点标题必填"
	}
	if len(site.Title) > 100 {
		return "站点标题最长 100 字符"
	}
	if len(site.Tagline) > 200 {
		return "站点副标题最长 200 字符"
	}
	if len(site.Description) > 500 {
		return "站点描述最长 500 字符"
	}
	if site.URL != "" && !strings.HasPrefix(site.URL, "http://") &&
		!strings.HasPrefix(site.URL, "https://") {
		return "站点地址须以 http:// 或 https:// 开头"
	}
	for _, l := range site.Links {
		if strings.TrimSpace(l.Label) == "" {
			return "链接名称必填"
		}
		if l.URL != "" && !strings.HasPrefix(l.URL, "http") &&
			!strings.HasPrefix(l.URL, "/") {
			return "链接地址须以 http 开头或为站内路径"
		}
		switch l.Position {
		case "header", "footer", "social":
		default:
			return "链接位置非法"
		}
	}
	return ""
}

// ---------- 重定向 ----------

type redirectInput struct {
	FromPath string `json:"from_path"`
	ToPath   string `json:"to_path"`
}

// listRedirects GET /api/redirects
func listRedirects(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := deps.Redirects.List()
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
	}
}

// createRedirect POST /api/redirects
func createRedirect(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body redirectInput
		if err := httpx.DecodeJSON(w, r, &body); err != nil {
			return
		}
		from := normalizePath(strings.TrimSpace(body.FromPath))
		to := strings.TrimSpace(body.ToPath)
		if from == "/" || to == "" {
			badRequest(w, "来源路径与目标路径必填，且来源不能是根路径")
			return
		}
		if to == from {
			badRequest(w, "来源与目标不能相同")
			return
		}
		if existing, ok := deps.Redirects.Find(from); ok && existing != "" {
			httpx.WriteError(w, http.StatusConflict, "该来源路径已存在")
			return
		}
		id, err := deps.Redirects.Create(from, to)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusCreated, map[string]any{"id": id})
	}
}

// deleteRedirect DELETE /api/redirects/{id}
func deleteRedirect(deps Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			notFound(w, "重定向不存在")
			return
		}
		if _, err := deps.Redirects.Delete(id); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}
