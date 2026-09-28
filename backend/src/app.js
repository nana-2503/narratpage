import express from 'express';
import cors from 'cors';
import { existsSync } from 'node:fs';
import { join } from 'node:path';
import { db } from './db.js';
import { seed } from './seed.js';
import { optionalAuth } from './auth.js';
import { rateLimit } from './rate-limit.js';
import { feedRouter } from './routes/feed.js';
import { authRouter } from './routes/auth.js';
import { postsRouter } from './routes/posts.js';
import { categoriesRouter } from './routes/categories.js';
import { commentsRouter } from './routes/comments.js';
import { uploadsRouter, UPLOAD_DIR } from './routes/uploads.js';

export function createApp() {
  seed(db);

  const app = express();
  app.disable('x-powered-by');
  app.use(cors());
  app.use(express.json({ limit: '256kb' }));

  // 轻量请求日志（不引入额外依赖）
  app.use((req, res, next) => {
    const start = Date.now();
    res.on('finish', () => {
      console.log(`[http] ${req.method} ${req.originalUrl} ${res.statusCode} ${Date.now() - start}ms`);
    });
    next();
  });

  app.get('/api/health', (_req, res) => {
    res.json({ status: 'ok', uptime: Math.round(process.uptime()) });
  });

  // 登录限流：同一 IP 每分钟最多 10 次，防暴力破解（仅作用于 POST /api/auth/login）
  const loginLimiter = rateLimit({
    windowMs: 60_000,
    max: 10,
    message: '登录尝试过于频繁，请 1 分钟后再试',
  });

  app.post('/api/auth/login', loginLimiter);
  app.use('/api', feedRouter); // 公开 RSS 订阅源，无需登录与限流
  app.use('/api/auth', authRouter);
  app.use('/api/posts', optionalAuth, postsRouter);
  app.use('/api/categories', optionalAuth, categoriesRouter);
  app.use('/api/comments', optionalAuth, commentsRouter);
  app.use('/api/uploads', express.static(UPLOAD_DIR, { maxAge: '7d', immutable: true }));
  app.use('/api/uploads', uploadsRouter);

  // 统一 404 与错误处理
  app.use('/api', (_req, res) => {
    res.status(404).json({ error: '接口不存在' });
  });

  // 一体化部署：后端直接托管前端构建产物（单容器模式）
  // 未提供产物时（如双容器 compose）自动退化为纯 API 服务
  const FRONTEND_DIST =
    process.env.FRONTEND_DIST || join(import.meta.dirname, '..', 'frontend', 'dist');
  if (existsSync(join(FRONTEND_DIST, 'index.html'))) {
    app.use(express.static(FRONTEND_DIST, { index: false, maxAge: '1h' }));
    // SPA history 回退（/api 之外的 GET 请求返回 index.html）
    app.get(/^(?!\/api(\/|$)).*/, (_req, res) => {
      res.sendFile('index.html', { root: FRONTEND_DIST });
    });
    console.log(`[blog] 前端产物已托管: ${FRONTEND_DIST}`);
  } else {
    console.log('[blog] 未检测到前端产物，仅提供 API（可通过 FRONTEND_DIST 指定）');
  }

  // Express 通过四个参数识别错误处理中间件
  app.use((err, _req, res, _next) => {
    // body-parser 等客户端错误：保留语义化状态码（畸形 JSON 应为 400 而非 500）
    const status = Number(err?.status || err?.statusCode) || 500;
    if (err?.type === 'entity.parse.failed') {
      return res.status(400).json({ error: '请求体不是合法的 JSON' });
    }
    if (err?.type === 'entity.too.large') {
      return res.status(413).json({ error: '请求体过大' });
    }
    if (err?.code === 'LIMIT_FILE_SIZE') {
      return res.status(413).json({ error: '图片大小不能超过 5MB' });
    }
    if (status >= 500) {
      console.error('[error]', err);
      return res.status(500).json({ error: '服务器内部错误' });
    }
    res.status(status).json({ error: err?.message || '请求有误' });
  });

  return app;
}
