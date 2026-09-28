import { Router } from 'express';
import { db } from '../db.js';
import { xmlEscape } from '../utils.js';

export const feedRouter = Router();

const SITE_TITLE = '叙页博客系统';
const SITE_URL = (process.env.SITE_URL || 'http://localhost:8080').replace(/\/+$/, '');
const MAX_ITEMS = 20;

/** SQLite 的 UTC 时间串转 RFC 822（RSS pubDate 要求） */
function toRFC822(iso) {
  const d = new Date(iso.includes('T') ? iso : `${iso.replace(' ', 'T')}Z`);
  return Number.isNaN(d.getTime()) ? new Date().toUTCString() : d.toUTCString();
}

/** 无摘要时取正文前 200 字符做粗略纯文本摘要 */
function plainExcerpt(content) {
  return content
    .replace(/```[\s\S]*?```/g, ' ')
    .replace(/[#>*`\-[\]()!]/g, '')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, 200);
}

// GET /api/rss.xml — 公开订阅源（最新 20 篇已发布文章）
feedRouter.get('/rss.xml', (_req, res) => {
  const posts = db
    .prepare(
      `SELECT p.title, p.slug, p.summary, p.content, p.published_at, p.created_at
       FROM posts p WHERE p.status = 'published'
       ORDER BY COALESCE(p.published_at, p.created_at) DESC
       LIMIT ?`
    )
    .all(MAX_ITEMS);

  const items = posts
    .map((p) => {
      const url = `${SITE_URL}/post/${encodeURIComponent(p.slug)}`;
      const description = p.summary || plainExcerpt(p.content);
      return `    <item>
      <title>${xmlEscape(p.title)}</title>
      <link>${xmlEscape(url)}</link>
      <guid isPermaLink="true">${xmlEscape(url)}</guid>
      <pubDate>${toRFC822(p.published_at || p.created_at)}</pubDate>
      <description>${xmlEscape(description)}</description>
    </item>`;
    })
    .join('\n');

  const xml = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>${xmlEscape(SITE_TITLE)}</title>
    <link>${xmlEscape(SITE_URL)}</link>
    <description>${xmlEscape(SITE_TITLE)} - 最新文章</description>
    <lastBuildDate>${new Date().toUTCString()}</lastBuildDate>
${items}
  </channel>
</rss>`;

  res.type('application/rss+xml').send(xml);
});
