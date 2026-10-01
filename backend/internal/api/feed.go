package api

import (
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"narratpage/internal/config"
	"narratpage/internal/models"
	"narratpage/internal/repo"
)

const (
	feedMaxItems = 20
	siteTitle    = "叙页博客系统"
)

var gmt = time.FixedZone("GMT", 0)

// siteURL 返回站点对外地址。
// 优先使用后台设置的站点地址，缺省回落到环境变量，
// 这样在后台改地址无需重启容器。
func siteURL(deps Deps, cfg config.Config) string {
	if u := strings.TrimSpace(deps.Options.Site().URL); u != "" {
		return strings.TrimRight(u, "/")
	}
	if deps.SiteURL != "" {
		return strings.TrimRight(deps.SiteURL, "/")
	}
	return strings.TrimRight(cfg.SiteURL, "/")
}

// feedItem RSS/Atom 条目。
type feedItem struct {
	Title       string
	Link        string
	GUID        string
	Description string
	PubDate     time.Time
	Author      string
}

func buildFeedItems(deps Deps, cfg config.Config) ([]feedItem, string, string, error) {
	base := siteURL(deps, cfg)
	site := deps.Options.Site()

	title := site.Title
	if title == "" {
		title = siteTitle
	}
	desc := site.Description
	if desc == "" {
		desc = site.Tagline
	}
	if desc == "" {
		desc = title
	}

	result, err := deps.Posts.ListPosts(repo.PostQuery{
		Type:     models.TypePost,
		Page:     1,
		PageSize: feedMaxItems,
		Order:    "desc",
	}, repo.PublicViewer())
	if err != nil {
		return nil, "", "", err
	}

	items := make([]feedItem, 0, len(result.Items))
	for _, p := range result.Items {
		link := base + "/post/" + p.Slug
		pub := models.ParseDate(models.Deref(p.PublishedAt))
		if pub.IsZero() {
			pub = models.ParseDate(p.CreatedAt)
		}
		author := ""
		if p.Author != nil {
			author = p.Author.DisplayName
		}
		items = append(items, feedItem{
			Title:       p.Title,
			Link:        link,
			GUID:        link,
			Description: p.Summary,
			PubDate:     pub,
			Author:      author,
		})
	}
	return items, title, desc, nil
}

// feedHandler GET /api/feed.xml — RSS 2.0
func feedHandler(deps Deps, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, title, desc, err := buildFeedItems(deps, cfg)
		if err != nil {
			http.Error(w, "服务器内部错误", http.StatusInternalServerError)
			return
		}
		base := siteURL(deps, cfg)

		var b strings.Builder
		b.WriteString(xml.Header)
		b.WriteString(`<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">` + "\n")
		b.WriteString("<channel>\n")
		fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(title))
		fmt.Fprintf(&b, "<link>%s</link>\n", html.EscapeString(base))
		fmt.Fprintf(&b, "<description>%s</description>\n", html.EscapeString(desc))
		fmt.Fprintf(&b, "<language>%s</language>\n", html.EscapeString(deps.Options.Site().Locale))
		b.WriteString(`<atom:link href="` + html.EscapeString(base+"/api/feed.xml") +
			`" rel="self" type="application/rss+xml"/>` + "\n")
		b.WriteString("<lastBuildDate>" + time.Now().UTC().In(gmt).Format(time.RFC1123Z) + "</lastBuildDate>\n")
		// 让订阅器能立即发现新文章
		b.WriteString("<ttl>60</ttl>\n")

		for _, it := range items {
			b.WriteString("<item>\n")
			fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(it.Title))
			fmt.Fprintf(&b, "<link>%s</link>\n", html.EscapeString(it.Link))
			fmt.Fprintf(&b, `<guid isPermaLink="true">%s</guid>`+"\n", html.EscapeString(it.GUID))
			if !it.PubDate.IsZero() {
				fmt.Fprintf(&b, "<pubDate>%s</pubDate>\n", it.PubDate.UTC().In(gmt).Format(time.RFC1123Z))
			}
			if it.Author != "" {
				fmt.Fprintf(&b, "<author>%s</author>\n", html.EscapeString(it.Author))
			}
			if it.Description != "" {
				fmt.Fprintf(&b, "<description>%s</description>\n", html.EscapeString(it.Description))
			}
			b.WriteString("</item>\n")
		}
		b.WriteString("</channel>\n</rss>")

		w.Header().Set("content-type", "application/rss+xml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=600")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(b.String()))
	}
}

// atomHandler GET /api/feed/atom.xml — Atom 1.0
func atomHandler(deps Deps, cfg config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, title, _, err := buildFeedItems(deps, cfg)
		if err != nil {
			http.Error(w, "服务器内部错误", http.StatusInternalServerError)
			return
		}
		base := siteURL(deps, cfg)
		updated := time.Now().UTC()
		if len(items) > 0 {
			updated = items[0].PubDate.UTC()
		}

		var b strings.Builder
		b.WriteString(xml.Header)
		b.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom">` + "\n")
		fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(title))
		fmt.Fprintf(&b, `<id>%s</id>`+"\n", html.EscapeString(base+"/"))
		fmt.Fprintf(&b, `<link href="%s"/>`+"\n", html.EscapeString(base))
		fmt.Fprintf(&b, `<link rel="self" href="%s"/>`+"\n",
			html.EscapeString(base+"/api/feed/atom.xml"))
		fmt.Fprintf(&b, "<updated>%s</updated>\n", updated.Format(time.RFC3339))

		for _, it := range items {
			b.WriteString("<entry>\n")
			fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(it.Title))
			fmt.Fprintf(&b, `<id>%s</id>`+"\n", html.EscapeString(it.GUID))
			fmt.Fprintf(&b, `<link href="%s"/>`+"\n", html.EscapeString(it.Link))
			if !it.PubDate.IsZero() {
				fmt.Fprintf(&b, "<updated>%s</updated>\n", it.PubDate.UTC().Format(time.RFC3339))
				fmt.Fprintf(&b, "<published>%s</published>\n", it.PubDate.UTC().Format(time.RFC3339))
			}
			if it.Author != "" {
				fmt.Fprintf(&b, "<author><name>%s</name></author>\n", html.EscapeString(it.Author))
			}
			if it.Description != "" {
				fmt.Fprintf(&b, `<summary type="html">%s</summary>`+"\n",
					html.EscapeString(it.Description))
			}
			b.WriteString("</entry>\n")
		}
		b.WriteString("</feed>")

		w.Header().Set("content-type", "application/atom+xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(b.String()))
	}
}
