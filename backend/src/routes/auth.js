import { Router } from 'express';
import bcrypt from 'bcryptjs';
import { db } from '../db.js';
import { signToken, requireAuth } from '../auth.js';

export const authRouter = Router();

authRouter.post('/login', (req, res) => {
  const { username, password } = req.body || {};
  if (!username || !password) {
    return res.status(400).json({ error: '用户名和密码必填' });
  }
  const user = db.prepare('SELECT * FROM users WHERE username = ?').get(username);
  if (!user || !bcrypt.compareSync(String(password), user.password_hash)) {
    return res.status(401).json({ error: '用户名或密码错误' });
  }
  res.json({ token: signToken(user), username: user.username });
});

authRouter.get('/me', requireAuth, (req, res) => {
  res.json({ username: req.user.username });
});

const MIN_PASSWORD = 8;
const MAX_PASSWORD = 72; // bcrypt 仅使用前 72 字节

authRouter.post('/password', requireAuth, (req, res) => {
  const oldPassword = String(req.body?.oldPassword || '');
  const newPassword = String(req.body?.newPassword || '');
  if (!oldPassword || !newPassword) {
    return res.status(400).json({ error: '原密码和新密码必填' });
  }
  if (newPassword.length < MIN_PASSWORD || newPassword.length > MAX_PASSWORD) {
    return res.status(400).json({ error: `新密码长度需在 ${MIN_PASSWORD}-${MAX_PASSWORD} 位之间` });
  }
  const user = db.prepare('SELECT * FROM users WHERE id = ?').get(req.user.sub);
  if (!user) {
    return res.status(404).json({ error: '用户不存在' });
  }
  if (!bcrypt.compareSync(oldPassword, user.password_hash)) {
    return res.status(401).json({ error: '原密码错误' });
  }
  db.prepare('UPDATE users SET password_hash = ? WHERE id = ?').run(
    bcrypt.hashSync(newPassword, 10),
    user.id
  );
  res.json({ ok: true });
});
