import { Router } from 'express';
import { db } from '../db.js';
import { requireAuth } from '../auth.js';
import { slugify, escapeLike } from '../utils.js';

export const postsRouter = Router();

const listSelect = `
  SELECT p.id, p.title, p.slug, p.summary, p.cover_url, p.status, p.views,
         p.created_at, p.updated_at, p.published_at,
         c.id AS category_id, c.name AS category_name, c.slug AS category_slug
  FROM posts p
  LEFT JOIN categories c ON c.id = p.category_id
`;

const detailSelect = `
  SELECT p.*, c.name AS category_name, c.slug AS category_slug
  FROM posts p LEFT JOIN categories c ON c.id = p.category_id
`;

const categoryExists = (id) => !!db.prepare('SELECT id FROM categories WHERE id = ?').get(id);

/** slug 被占用时追加时间戳后缀，保证可写 */
function uniqueSlug(slug) {
  return db.prepare('SELECT id FROM posts WHERE slug = ?').get(slug)
    ? `${slug}-${Date.now().toString(36)}`
    : slug;
}

// GET /api/posts?page&pageSize&category&q&status=all(仅管理员)
postsRouter.get('/', (req, res) => {
  const page = Math.max(1, parseInt(req.query.page, 10) || 1);
  const pageSize = Math.min(50, Math.max(1, parseInt(req.query.pageSize, 10) || 10));
  const { category, q } = req.query;

  const wantAll = req.query.status === 'all' && req.user;
  const where = wantAll ? ['1 = 1'] : ["p.status = 'published'"];
  const params = [];

  if (category) {
    where.push('c.slug = ?');
    params.push(category);
  }
  if (q) {
    where.push('(p.title LIKE ? ESCAPE \'\\\' OR p.summary LIKE ? ESCAPE \'\\\')');
    params.push(`%${escapeLike(q)}%`, `%${escapeLike(q)}%`);
  }

  const whereSql = `WHERE ${where.join(' AND ')}`;
  const total = db
    .prepare(`SELECT COUNT(*) AS n FROM posts p LEFT JOIN categories c ON c.id = p.category_id ${whereSql}`)
    .get(...params).n;

  const items = db
    .prepare(
      `${listSelect} ${whereSql}
       ORDER BY COALESCE(p.published_at, p.created_at) DESC
       LIMIT ? OFFSET ?`
    )
    .all(...params, pageSize, (page - 1) * pageSize);

  res.json({ items, total, page, pageSize, totalPages: Math.ceil(total / pageSize) });
});

// GET /api/posts/id/:id（后台编辑用，含正文；需在 /:slug 之前注册）
postsRouter.get('/id/:id', requireAuth, (req, res) => {
  const post = db.prepare(`${detailSelect} WHERE p.id = ?`).get(req.params.id);
  if (!post) {
    return res.status(404).json({ error: '文章不存在' });
  }
  res.json(post);
});

// GET /api/posts/:slug（详情含正文 content）
postsRouter.get('/:slug', (req, res) => {
  const isAdmin = !!req.user;
  const post = db
    .prepare(`${detailSelect} WHERE p.slug = ? ${isAdmin ? '' : "AND p.status = 'published'"}`)
    .get(req.params.slug);
  if (!post) {
    return res.status(404).json({ error: '文章不存在' });
  }
  db.prepare('UPDATE posts SET views = views + 1 WHERE id = ?').run(post.id);
  post.views += 1;
  res.json(post);
});

/**
 * 校验并规范化文章字段，创建与更新共用。
 * @param partial true 时忽略请求中未提供的字段（更新语义）
 * @returns {{ errors: string[], patch: object }} patch 仅含已提供且合法的字段
 */
function validatePost(body, partial = false) {
  const errors = [];
  const patch = {};
  const provided = (key) => !partial || body[key] !== undefined;

  if (provided('title')) {
    const title = (body.title || '').trim();
    if (!title) errors.push('标题必填');
    else if (title.length > 200) errors.push('标题最长 200 字符');
    patch.title = title;
  }
  if (provided('summary')) {
    const summary = (body.summary || '').trim();
    if (summary.length > 500) errors.push('摘要最长 500 字符');
    patch.summary = summary;
  }
  if (provided('content')) patch.content = body.content || '';
  if (provided('cover_url')) patch.cover_url = (body.cover_url || '').trim();
  if (provided('category_id')) patch.category_id = body.category_id || null;
  if (provided('status')) {
    const status = body.status || 'draft';
    if (!['draft', 'published'].includes(status)) errors.push('非法状态');
    patch.status = status;
  }
  return { errors, patch };
}

postsRouter.post('/', requireAuth, (req, res) => {
  const { errors, patch } = validatePost(req.body || {});
  if (errors.length) return res.status(400).json({ error: errors[0] });

  if (patch.category_id && !categoryExists(patch.category_id)) {
    return res.status(400).json({ error: '分类不存在' });
  }

  const slug = uniqueSlug((req.body.slug || '').trim() || slugify(patch.title, 'post'));
  const info = db
    .prepare(
      `INSERT INTO posts (title, slug, summary, content, cover_url, category_id, status, published_at)
       VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
    )
    .run(
      patch.title, slug, patch.summary, patch.content, patch.cover_url, patch.category_id,
      patch.status, patch.status === 'published' ? new Date().toISOString() : null
    );
  res.status(201).json({ id: info.lastInsertRowid, slug });
});

postsRouter.put('/:id', requireAuth, (req, res) => {
  const post = db.prepare('SELECT * FROM posts WHERE id = ?').get(req.params.id);
  if (!post) return res.status(404).json({ error: '文章不存在' });

  const { errors, patch } = validatePost(req.body || {}, true);
  if (errors.length) return res.status(400).json({ error: errors[0] });

  if (patch.category_id && !categoryExists(patch.category_id)) {
    return res.status(400).json({ error: '分类不存在' });
  }

  // 未提供的字段保持原值；从草稿首次发布时记录发布时间
  const next = { ...post, ...patch };
  const published_at =
    next.status === 'published' && post.status !== 'published'
      ? new Date().toISOString()
      : post.published_at;

  db.prepare(
    `UPDATE posts SET title = ?, summary = ?, content = ?, cover_url = ?, category_id = ?,
       status = ?, published_at = ?, updated_at = datetime('now')
     WHERE id = ?`
  ).run(
    next.title, next.summary, next.content, next.cover_url, next.category_id,
    next.status, published_at, post.id
  );

  res.json({ id: post.id });
});

postsRouter.delete('/:id', requireAuth, (req, res) => {
  const info = db.prepare('DELETE FROM posts WHERE id = ?').run(req.params.id);
  if (info.changes === 0) return res.status(404).json({ error: '文章不存在' });
  res.json({ ok: true });
});
