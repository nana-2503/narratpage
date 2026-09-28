import { Router } from 'express';
import { db } from '../db.js';
import { requireAuth } from '../auth.js';
import { slugify } from '../utils.js';

export const categoriesRouter = Router();

categoriesRouter.get('/', (req, res) => {
  const rows = db
    .prepare(
      `SELECT c.id, c.name, c.slug,
              (SELECT COUNT(*) FROM posts p WHERE p.category_id = c.id AND p.status = 'published') AS post_count
       FROM categories c ORDER BY c.id`
    )
    .all();
  res.json({ items: rows });
});

categoriesRouter.post('/', requireAuth, (req, res) => {
  const name = (req.body?.name || '').trim();
  if (!name) return res.status(400).json({ error: '分类名称必填' });
  if (name.length > 50) return res.status(400).json({ error: '分类名最长 50 字符' });
  let slug = (req.body.slug || '').trim() || slugify(name, 'cat');
  if (db.prepare('SELECT id FROM categories WHERE slug = ?').get(slug)) {
    return res.status(409).json({ error: 'slug 已存在' });
  }
  const info = db.prepare('INSERT INTO categories (name, slug) VALUES (?, ?)').run(name, slug);
  res.status(201).json({ id: info.lastInsertRowid, name, slug });
});

categoriesRouter.put('/:id', requireAuth, (req, res) => {
  const cat = db.prepare('SELECT * FROM categories WHERE id = ?').get(req.params.id);
  if (!cat) return res.status(404).json({ error: '分类不存在' });
  const name = (req.body?.name || '').trim() || cat.name;
  const slug = (req.body?.slug || '').trim() || cat.slug;
  if (db.prepare('SELECT id FROM categories WHERE slug = ? AND id != ?').get(slug, cat.id)) {
    return res.status(409).json({ error: 'slug 已存在' });
  }
  db.prepare('UPDATE categories SET name = ?, slug = ? WHERE id = ?').run(name, slug, cat.id);
  res.json({ id: cat.id });
});

categoriesRouter.delete('/:id', requireAuth, (req, res) => {
  const info = db.prepare('DELETE FROM categories WHERE id = ?').run(req.params.id);
  if (info.changes === 0) return res.status(404).json({ error: '分类不存在' });
  res.json({ ok: true });
});
