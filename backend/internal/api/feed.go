package api

import (
	"database/sql"
	"html"
	"net/http"
	"strings"
	"time"

	"narratpage/internal/httpx"
)

const (
	siteTitle  = "叙页博客系统"
	maxFeedItm = 20
)

var gmt = time.FixedZone("GMT", 0)

// GET /api/rss.xml — 公开订阅源（最新 20 篇已发布文章）
func rssFeed(db *sql.DB, siteURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(
			`SELECT p.title, p.slug, p.summary, p.content, p.published_at, p.created_at
			 FROM posts p WHERE p.status = 'published'
			 ORDER BY COALESCE(p.published_at, p.created_at) DESC LIMIT ?`,
			maxFeedItm,
		)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "服务器内部错误")
			return
		}
		defer rows.Close()

		var items strings.Builder
		for rows.Next() {
			var title, slug, summary, content, createdAt string
			var publishedAt *string
			if err := rows.Scan(&title, &slug, &summary, &content, &publishedAt, &createdAt); err != nil {
				continue
			}
			url := siteURL + "/post/" + html.EscapeString(slug)
			pub := createdAt
			if publishedAt != nil && *publishedAt != "" {
				pub = *publishedAt
			}
			items.WriteString("    <item>\n")
			items.WriteString("      <title>" + html.EscapeString(title) + "</title>\n")
			items.WriteString("      <link>" + url + "</link>\n")
			items.WriteString("      <guid isPermaLink=\"true\">" + url + "</guid>\n")
			items.WriteString("      <pubDate>" + rfc822(pub) + "</pubDate>\n")
			items.WriteString("      <description>" + html.EscapeString(feedDescription(summary, content)) + "</description>\n")
			items.WriteString("    </item>\n")
		}

		xml := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<rss version=\"2.0\">\n  <channel>\n" +
			"    <title>" + html.EscapeString(siteTitle) + "</title>\n" +
			"    <link>" + html.EscapeString(siteURL) + "</link>\n" +
			"    <description>" + html.EscapeString(siteTitle) + " - 最新文章</description>\n" +
			"    <lastBuildDate>" + time.Now().UTC().In(gmt).Format(time.RFC1123) + "</lastBuildDate>\n" +
			items.String() +
			"  </channel>\n</rss>"

		w.Header().Set("content-type", "application/rss+xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(xml))
	}
}

// rfc822 SQLite/ISO 时间串 → RFC822（RSS pubDate 要求 GMT）
func rfc822(ts string) string {
	var t time.Time
	if strings.Contains(ts, "T") {
		t, _ = time.Parse(time.RFC3339Nano, ts)
	} else {
		t, _ = time.Parse("2006-01-02 15:04:05", ts)
	}
	if t.IsZero() {
		return time.Now().UTC().In(gmt).Format(time.RFC1123)
	}
	return t.UTC().In(gmt).Format(time.RFC1123)
}

// feedDescription 无摘要时取正文前 200 字符的粗略纯文本
func feedDescription(summary, content string) string {
	if summary != "" {
		return summary
	}
	var b strings.Builder
	n := 0
	for _, r := range content {
		if r == '\n' || r == '#' || r == '*' || r == '`' || r == '>' || r == '-' {
			r = ' '
		}
		b.WriteRune(r)
		n++
		if n >= 400 {
			break
		}
	}
	out := strings.Join(strings.Fields(b.String()), " ")
	if len(out) > 200 {
		return out[:200]
	}
	return out
}
