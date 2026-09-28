import { Router } from 'express';
import { db } from '../db.js';
import { requireAuth } from '../auth.js';

export const commentsRouter = Router();

const MAX_AUTHOR = 30;
const MAX_CONTENT = 1000;

// GET /api/comments?postId=1&status=pending(仅管理员)
commentsRouter.get('/', (req, res) => {
  const { postId } = req.query;
  const isAdmin = !!req.user;
  const where = [];
  const params = [];
  if (postId) {
    where.push('cm.post_id = ?');
    params.push(postId);
  }
  // 仅管理员显式传 status 时按状态筛选，其余情况（游客、管理员未指定）一律只看已通过
  if (isAdmin && req.query.status) {
    where.push('cm.status = ?');
    params.push(req.query.status);
  } else {
    where.push("cm.status = 'approved'");
  }
  const rows = db
    .prepare(
      `SELECT cm.id, cm.post_id, cm.author, cm.content, cm.status, cm.created_at,
              p.title AS post_title
       FROM comments cm JOIN posts p ON p.id = cm.post_id
       WHERE ${where.join(' AND ')} ORDER BY cm.created_at DESC LIMIT 200`
    )
    .all(...params);
  res.json({ items: rows });
});

commentsRouter.post('/', (req, res) => {
  const { postId } = req.body || {};
  const author = String(req.body?.author || '').trim();
  const content = String(req.body?.content || '').trim();
  if (!postId || !db.prepare('SELECT id FROM posts WHERE id = ?').get(postId)) {
    return res.status(400).json({ error: '文章不存在' });
  }
  if (!author || author.length > MAX_AUTHOR) {
    return res.status(400).json({ error: `昵称必填且最长 ${MAX_AUTHOR} 字符` });
  }
  if (!content || content.length > MAX_CONTENT) {
    return res.status(400).json({ error: `评论内容必填且最长 ${MAX_CONTENT} 字符` });
  }
  const info = db
    .prepare('INSERT INTO comments (post_id, author, content) VALUES (?, ?, ?)')
    .run(postId, author, content);
  res.status(201).json({ id: info.lastInsertRowid, status: 'pending' });
});

commentsRouter.put('/:id', requireAuth, (req, res) => {
  const comment = db.prepare('SELECT * FROM comments WHERE id = ?').get(req.params.id);
  if (!comment) return res.status(404).json({ error: '评论不存在' });
  const { status } = req.body || {};
  if (!['pending', 'approved', 'rejected'].includes(status)) {
    return res.status(400).json({ error: '非法状态' });
  }
  db.prepare('UPDATE comments SET status = ? WHERE id = ?').run(status, comment.id);
  res.json({ id: comment.id, status });
});

commentsRouter.delete('/:id', requireAuth, (req, res) => {
  const info = db.prepare('DELETE FROM comments WHERE id = ?').run(req.params.id);
  if (info.changes === 0) return res.status(404).json({ error: '评论不存在' });
  res.json({ ok: true });
});
