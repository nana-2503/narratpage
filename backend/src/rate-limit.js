/**
 * 极简内存滑动窗口限流中间件（无第三方依赖）。
 * 单实例部署（如本项目 SQLite 单容器）足够；多实例水平扩展时应换为
 * Redis 等共享存储的限流方案。
 */
export function rateLimit({
  windowMs = 60_000,
  max = 10,
  message = '请求过于频繁，请稍后再试',
} = {}) {
  /** @type {Map<string, { count: number, resetAt: number }>} */
  const buckets = new Map();

  // 定期清理过期桶，防止 Map 无限增长；unref 避免阻塞进程退出
  const sweep = setInterval(() => {
    const now = Date.now();
    for (const [key, bucket] of buckets) {
      if (bucket.resetAt <= now) buckets.delete(key);
    }
  }, windowMs);
  sweep.unref();

  return function rateLimitMiddleware(req, res, next) {
    const now = Date.now();
    const key = req.ip || 'unknown';
    let bucket = buckets.get(key);
    if (!bucket || bucket.resetAt <= now) {
      bucket = { count: 0, resetAt: now + windowMs };
      buckets.set(key, bucket);
    }
    bucket.count += 1;

    const remaining = Math.max(0, max - bucket.count);
    res.setHeader('X-RateLimit-Remaining', String(remaining));
    if (bucket.count > max) {
      res.setHeader('Retry-After', String(Math.ceil((bucket.resetAt - now) / 1000)));
      return res.status(429).json({ error: message });
    }
    next();
  };
}
