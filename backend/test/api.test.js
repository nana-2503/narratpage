import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

// 环境变量必须在 db.js 首次导入前就位，因此用顶层 await 动态导入
const DATA_DIR = mkdtempSync(join(tmpdir(), 'blog-test-'));
process.env.DATA_DIR = DATA_DIR;
process.env.JWT_SECRET = 'test-secret';
process.env.ADMIN_USERNAME = 'admin';
process.env.ADMIN_PASSWORD = 'test-password-123';

const { createApp } = await import('../src/app.js');
const { db } = await import('../src/db.js');

let server;
let base;
let adminToken;

/** 发起 API 请求；opts.token=true 时附带管理员 token，=false 明确不带 */
async function api(method, path, body, { token = false } = {}) {
  const headers = {};
  if (body !== undefined) headers['content-type'] = 'application/json';
  if (token === true) headers.authorization = `Bearer ${adminToken}`;
  else if (typeof token === 'string') headers.authorization = `Bearer ${token}`;
  const res = await fetch(`${base}${path}`, {
    method,
    headers,
    body: body === undefined ? undefined : typeof body === 'string' ? body : JSON.stringify(body),
  });
  const text = await res.text();
  return { status: res.status, body: text ? JSON.parse(text) : {} };
}

before(async () => {
  const app = createApp(); // 每个 before 调用都会 seeding，幂等
  await new Promise((resolve) => {
    server = app.listen(0, '127.0.0.1', resolve);
  });
  base = `http://127.0.0.1:${server.address().port}/api`;
});

after(() => {
  server.close();
  // 显式关闭连接，避免 better-sqlite3 语句在进程退出析构时崩溃
  db.close();
  rmSync(DATA_DIR, { recursive: true, force: true });
});

// ---------- 基础 ----------

test('GET /health 返回运行状态', async () => {
  const res = await api('GET', '/health');
  assert.equal(res.status, 200);
  assert.equal(res.body.status, 'ok');
  assert.ok(res.body.uptime >= 0);
});

test('未知 /api 路径返回 404', async () => {
  const res = await api('GET', '/no-such-route');
  assert.equal(res.status, 404);
});

test('畸形 JSON 请求体返回 400 而非 500', async () => {
  const r = await fetch(`${base}/auth/login`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: '{broken json',
  });
  assert.equal(r.status, 400);
  const body = await r.json();
  assert.match(body.error, /JSON/);
});

// ---------- 认证 ----------

test('未提供 token 访问 /auth/me 返回 401', async () => {
  const res = await api('GET', '/auth/me');
  assert.equal(res.status, 401);
});

test('错误 token 访问 /auth/me 返回 401', async () => {
  const res = await api('GET', '/auth/me', undefined, { token: 'not-a-valid-jwt' });
  assert.equal(res.status, 401);
});

test('错误密码登录返回 401', async () => {
  const res = await api('POST', '/auth/login', { username: 'admin', password: 'wrong' });
  assert.equal(res.status, 401);
});

test('缺少用户名或密码返回 400', async () => {
  const res = await api('POST', '/auth/login', { username: 'admin' });
  assert.equal(res.status, 400);
});

test('正确凭据登录返回 token', async () => {
  const res = await api('POST', '/auth/login', {
    username: 'admin',
    password: 'test-password-123',
  });
  assert.equal(res.status, 200);
  assert.ok(res.body.token);
  assert.equal(res.body.username, 'admin');
  adminToken = res.body.token;
});

test('携带有效 token 访问 /auth/me 返回用户名', async () => {
  const res = await api('GET', '/auth/me', undefined, { token: true });
  assert.equal(res.status, 200);
  assert.equal(res.body.username, 'admin');
});

test('未登录创建文章返回 401', async () => {
  const res = await api('POST', '/posts', { title: 'x', content: 'y' });
  assert.equal(res.status, 401);
});

// ---------- 文章 ----------

test('游客列表只含已发布文章', async () => {
  const res = await api('GET', '/posts?pageSize=50');
  assert.equal(res.status, 200);
  assert.ok(res.body.items.every((p) => p.status === 'published'));
  assert.ok(res.body.total >= 3); // seed 的示例文章
});

test('管理员可用 status=all 查看草稿', async () => {
  const guest = await api('GET', '/posts?status=all&pageSize=50');
  const admin = await api('GET', '/posts?status=all&pageSize=50', undefined, { token: true });
  assert.ok(guest.body.items.every((p) => p.status === 'published'));
  assert.ok(admin.body.total >= guest.body.total);
});

test('创建文章：校验必填、分类存在、slug 唯一', async () => {
  // 缺标题
  let res = await api('POST', '/posts', { content: '正文' }, { token: true });
  assert.equal(res.status, 400);

  // 分类不存在
  res = await api(
    'POST',
    '/posts',
    { title: '测试文章', content: '正文', category_id: 99999 },
    { token: true },
  );
  assert.equal(res.status, 400);
  assert.match(res.body.error, /分类/);

  // 合法创建，slug 自动从标题生成
  res = await api('POST', '/posts', { title: 'Hello World 测试', content: '正文' }, { token: true });
  assert.equal(res.status, 201);
  assert.equal(res.body.slug, 'hello-world-测试');

  // 重复 slug 自动追加后缀
  res = await api('POST', '/posts', { title: 'Hello World 测试', content: '正文2' }, { token: true });
  assert.equal(res.status, 201);
  assert.notEqual(res.body.slug, 'hello-world-测试');
});

test('搜索转义 LIKE 通配符：q=% 不应匹配全部', async () => {
  const all = await api('GET', '/posts?pageSize=50');
  const percent = await api('GET', `/posts?q=${encodeURIComponent('%')}&pageSize=50`);
  assert.equal(percent.status, 200);
  assert.equal(percent.body.total, 0);
  assert.ok(all.body.total > 0);
});

test('搜索可命中标题与摘要', async () => {
  // 显式发布两篇，保证游客检索可见
  await api('POST', '/posts', { title: 'Searchable One', content: 'x', status: 'published' }, { token: true });
  await api('POST', '/posts', { title: 'Searchable Two', content: 'x', status: 'published' }, { token: true });
  const res = await api('GET', `/posts?q=${encodeURIComponent('Searchable')}&pageSize=50`);
  assert.equal(res.status, 200);
  assert.ok(res.body.items.length >= 2);
  assert.ok(res.body.items.every((p) => /searchable/i.test(p.title) || /searchable/i.test(p.summary || '')));
});

test('草稿对外不可见，作者本人可见', async () => {
  const draft = await api('POST', '/posts', { title: 'Only Draft', status: 'draft' }, { token: true });
  assert.equal(draft.status, 201);
  const guest = await api('GET', `/posts/${draft.body.slug}`);
  assert.equal(guest.status, 404);
  const owner = await api('GET', `/posts/${draft.body.slug}`, undefined, { token: true });
  assert.equal(owner.status, 200);
  assert.equal(owner.body.title, 'Only Draft');
});

test('通过 /posts/id/:id 可读取草稿正文', async () => {
  const created = await api('POST', '/posts', { title: 'ById Draft', content: '秘密内容' }, { token: true });
  const res = await api('GET', `/posts/id/${created.body.id}`, undefined, { token: true });
  assert.equal(res.status, 200);
  assert.equal(res.body.content, '秘密内容');
  // 游客不可访问
  const guest = await api('GET', `/posts/id/${created.body.id}`);
  assert.equal(guest.status, 401);
});

test('访问文章详情自增阅读数', async () => {
  // 自建一篇已发布文章，避免依赖 seed 数据被其他用例改动
  const created = await api(
    'POST',
    '/posts',
    { title: 'View Counter', content: 'x', status: 'published' },
    { token: true },
  );
  const slug = encodeURIComponent(created.body.slug);
  const before = await api('GET', `/posts/${slug}`);
  assert.equal(before.status, 200);
  const after = await api('GET', `/posts/${slug}`);
  assert.equal(after.body.views, before.body.views + 1);
});

test('更新文章：部分字段与发布时间语义', async () => {
  const created = await api('POST', '/posts', { title: 'To Update', content: 'v1' }, { token: true });
  const { id } = created.body;

  // 部分更新：仅改摘要，标题保持
  let res = await api('PUT', `/posts/${id}`, { summary: '新摘要' }, { token: true });
  assert.equal(res.status, 200);
  let detail = await api('GET', `/posts/id/${id}`, undefined, { token: true });
  assert.equal(detail.body.summary, '新摘要');
  assert.equal(detail.body.title, 'To Update');
  assert.equal(detail.body.published_at, null);

  // 发布后写入 published_at
  res = await api('PUT', `/posts/${id}`, { status: 'published' }, { token: true });
  assert.equal(res.status, 200);
  detail = await api('GET', `/posts/id/${id}`, undefined, { token: true });
  assert.ok(detail.body.published_at);

  // 更新不存在文章 404
  res = await api('PUT', '/posts/999999', { title: 'x' }, { token: true });
  assert.equal(res.status, 404);
});

test('删除文章：幂等 404', async () => {
  const created = await api('POST', '/posts', { title: 'To Delete' }, { token: true });
  let res = await api('DELETE', `/posts/${created.body.id}`, undefined, { token: true });
  assert.equal(res.status, 200);
  res = await api('DELETE', `/posts/${created.body.id}`, undefined, { token: true });
  assert.equal(res.status, 404);
});

// ---------- 分类 ----------

test('分类：创建/查重/删除', async () => {
  let res = await api('POST', '/categories', { name: '测试分类' }, { token: true });
  assert.equal(res.status, 201);
  const { id, slug } = res.body;
  assert.equal(slug, '测试分类');

  // 重名 slug 409
  res = await api('POST', '/categories', { name: '测试分类' }, { token: true });
  assert.equal(res.status, 409);

  // 改名默认保留 slug（外链不失效），显式传新 slug 才释放原 slug
  res = await api('PUT', `/categories/${id}`, { name: '分类新名' }, { token: true });
  assert.equal(res.status, 200);
  res = await api('POST', '/categories', { name: '测试分类' }, { token: true });
  assert.equal(res.status, 409); // slug 仍被占用

  res = await api('PUT', `/categories/${id}`, { name: '分类新名', slug: 'cat-renamed' }, { token: true });
  assert.equal(res.status, 200);
  res = await api('POST', '/categories', { name: '测试分类' }, { token: true });
  assert.equal(res.status, 201);

  res = await api('DELETE', `/categories/${id}`, undefined, { token: true });
  assert.equal(res.status, 200);
  res = await api('DELETE', `/categories/${id}`, undefined, { token: true });
  assert.equal(res.status, 404);
});

test('删除分类后文章变为未分类', async () => {
  const cat = await api('POST', '/categories', { name: '临时分类' }, { token: true });
  const post = await api(
    'POST',
    '/posts',
    { title: 'Cat Post', category_id: cat.body.id },
    { token: true },
  );
  await api('DELETE', `/categories/${cat.body.id}`, undefined, { token: true });
  const detail = await api('GET', `/posts/id/${post.body.id}`, undefined, { token: true });
  assert.equal(detail.body.category_id, null);
  assert.equal(detail.body.category_name, null);
});

test('游客不能增删改分类', async () => {
  let res = await api('POST', '/categories', { name: 'x' });
  assert.equal(res.status, 401);
  res = await api('DELETE', '/categories/1');
  assert.equal(res.status, 401);
});

// ---------- 评论 ----------

test('评论：提交后待审核，游客只见已通过', async () => {
  const post = await api('POST', '/posts', { title: 'Comment Target' }, { token: true });
  const postId = post.body.id;

  let res = await api('POST', '/comments', {
    postId,
    author: '读者',
    content: '待审核的评论',
  });
  assert.equal(res.status, 201);
  assert.equal(res.body.status, 'pending');
  const commentId = res.body.id;

  // 游客默认只看 approved
  res = await api('GET', `/comments?postId=${postId}`);
  assert.equal(res.status, 200);
  assert.equal(res.body.items.length, 0);

  // 空内容 / 超长昵称 400
  res = await api('POST', '/comments', { postId, author: '', content: 'x' });
  assert.equal(res.status, 400);
  res = await api('POST', '/comments', { postId, author: 'x'.repeat(31), content: 'x' });
  assert.equal(res.status, 400);

  // 不存在的文章 400
  res = await api('POST', '/comments', { postId: 999999, author: 'a', content: 'b' });
  assert.equal(res.status, 400);

  // 管理员按状态筛选可见 pending，且带文章标题
  res = await api('GET', `/comments?postId=${postId}&status=pending`, undefined, { token: true });
  assert.equal(res.status, 200);
  assert.equal(res.body.items.length, 1);
  assert.equal(res.body.items[0].post_title, 'Comment Target');

  // 审核通过后游客可见
  res = await api('PUT', `/comments/${commentId}`, { status: 'approved' }, { token: true });
  assert.equal(res.status, 200);
  res = await api('GET', `/comments?postId=${postId}`);
  assert.equal(res.body.items.length, 1);

  // 游客不能审核 / 删除
  res = await api('PUT', `/comments/${commentId}`, { status: 'rejected' });
  assert.equal(res.status, 401);
  res = await api('DELETE', `/comments/${commentId}`);
  assert.equal(res.status, 401);

  // 非法状态 400
  res = await api('PUT', `/comments/${commentId}`, { status: 'deleted' }, { token: true });
  assert.equal(res.status, 400);

  // 删除
  res = await api('DELETE', `/comments/${commentId}`, undefined, { token: true });
  assert.equal(res.status, 200);
});

// ---------- RSS 订阅 ----------

test('GET /rss.xml 返回合法 RSS 且内容已转义', async () => {
  const res = await fetch(`${base}/rss.xml`);
  assert.equal(res.status, 200);
  assert.match(res.headers.get('content-type') || '', /application\/rss\+xml/);
  const xml = await res.text();
  assert.match(xml, /<\?xml version="1\.0" encoding="UTF-8"\?>/);
  assert.match(xml, /<rss version="2\.0">/);
  assert.match(xml, /<channel>/);
  assert.match(xml, /叙页博客系统/);
  assert.match(xml, /<lastBuildDate>/);
  // seed 的已发布文章标题出现在 feed 中
  assert.ok(xml.includes('为什么选择 SQLite 作为博客数据库'));

  // 标题中的 XML 特殊字符必须转义，不能原样输出
  await api('POST', '/posts', {
    title: 'XML <b>转义</b> "引号" & 符号',
    status: 'published',
  }, { token: true });
  const after = await (await fetch(`${base}/rss.xml`)).text();
  assert.ok(after.includes('XML &lt;b&gt;转义&lt;/b&gt;'));
  assert.ok(!after.includes('<b>转义</b>'));
});

// ---------- 上下篇 ----------

test('GET /posts/:slug/neighbors 返回按时间排序的上下篇', async () => {
  // 依次创建三篇已发布文章（published_at 带毫秒，创建序即时序）
  const slugs = [];
  for (const title of ['邻居文章甲', '邻居文章乙', '邻居文章丙']) {
    const created = await api('POST', '/posts', { title, status: 'published' }, { token: true });
    slugs.push(created.body.slug);
  }
  const res = await api('GET', `/posts/${slugs[1]}/neighbors`);
  assert.equal(res.status, 200);
  assert.equal(res.body.prev.slug, slugs[2]); // 更新的
  assert.equal(res.body.next.slug, slugs[0]); // 更旧的

  // 三篇之中：较旧的两篇的 prev 只能是更新的（本篇或外部更新的文章），不可能反向
  const first = await api('GET', `/posts/${slugs[2]}/neighbors`);
  assert.ok(first.body.prev === null || !slugs.includes(first.body.prev.slug));
  const last = await api('GET', `/posts/${slugs[0]}/neighbors`);
  assert.ok(last.body.next === null || !slugs.includes(last.body.next.slug));

  // 游客可访问；不存在/未发布的文章 404
  const guest = await api('GET', `/posts/${slugs[1]}/neighbors`);
  assert.equal(guest.status, 200);
  const missing = await api('GET', '/posts/no-such-post/neighbors');
  assert.equal(missing.status, 404);
  const draft = await api('POST', '/posts', { title: '草稿邻居' }, { token: true });
  const draftRes = await api('GET', `/posts/${draft.body.slug}/neighbors`);
  assert.equal(draftRes.status, 404);

  // 同一秒发布的文章用 id 兜底形成全序：neighbors 必须与列表排序互为一致
  const list = await api('GET', '/posts?pageSize=50');
  const order = list.body.items.map((p) => p.slug);
  for (const s of ['why-sqlite-for-blog', 'markdown-writing-pipeline', 'restart-blogging']) {
    const idx = order.indexOf(s);
    assert.ok(idx >= 0, `${s} 应在已发布列表中`);
    const r = await api('GET', `/posts/${s}/neighbors`);
    const expectPrev = idx > 0 ? order[idx - 1] : null; // 列表中更靠前 = 更新
    const expectNext = idx < order.length - 1 ? order[idx + 1] : null;
    assert.equal(r.body.prev?.slug ?? null, expectPrev);
    assert.equal(r.body.next?.slug ?? null, expectNext);
  }
});

// ---------- 修改密码 ----------

test('POST /auth/password 修改密码并即时生效', async () => {
  // 未登录 401
  let res = await api('POST', '/auth/password', { oldPassword: 'x', newPassword: 'y' });
  assert.equal(res.status, 401);

  // 原密码错误 401
  res = await api('POST', '/auth/password', { oldPassword: 'wrong-old', newPassword: 'newpass123' }, { token: true });
  assert.equal(res.status, 401);

  // 新密码过短 400
  res = await api('POST', '/auth/password', { oldPassword: 'test-password-123', newPassword: 'short' }, { token: true });
  assert.equal(res.status, 400);

  // 正常修改
  res = await api('POST', '/auth/password', { oldPassword: 'test-password-123', newPassword: 'brand-new-pass-456' }, { token: true });
  assert.equal(res.status, 200);

  // 新密码可登录，旧密码失效
  res = await api('POST', '/auth/login', { username: 'admin', password: 'brand-new-pass-456' });
  assert.equal(res.status, 200);
  assert.ok(res.body.token);
  res = await api('POST', '/auth/login', { username: 'admin', password: 'test-password-123' });
  assert.equal(res.status, 401);
});

// ---------- 图片上传 ----------

const PNG_1x1 = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
  'base64',
);

/** multipart 上传请求（Node 原生 FormData，勿手动设 content-type） */
async function upload(buffer, mime, name, { auth = true } = {}) {
  const form = new FormData();
  form.append('file', new Blob([buffer], { type: mime }), name);
  return fetch(`${base}/uploads`, {
    method: 'POST',
    headers: auth ? { authorization: `Bearer ${adminToken}` } : {},
    body: form,
  });
}

test('POST /uploads：仅管理员可上传，返回地址可静态访问', async () => {
  // 游客 401
  let res = await upload(PNG_1x1, 'image/png', 'dot.png', { auth: false });
  assert.equal(res.status, 401);

  // 管理员上传 201，随机文件名 + 原扩展名
  res = await upload(PNG_1x1, 'image/png', 'dot.png');
  assert.equal(res.status, 201);
  const { url } = await res.json();
  assert.match(url, /^\/api\/uploads\/[a-f0-9]{16}\.png$/);

  // 匿名可读（文章插图场景）
  const img = await fetch(`http://127.0.0.1:${server.address().port}${url}`);
  assert.equal(img.status, 200);
  assert.match(img.headers.get('content-type') || '', /image\/png/);
  const bytes = Buffer.from(await img.arrayBuffer());
  assert.equal(bytes.equals(PNG_1x1), true);
});

test('POST /uploads：拒绝非图片 MIME 与伪造内容', async () => {
  // 声明 MIME 非图片 → 400
  let res = await upload(Buffer.from('hello'), 'text/plain', 'a.txt');
  assert.equal(res.status, 400);

  // 声明 image/png 但内容不是图片（magic bytes 兜底）→ 400 且不落库
  res = await upload(Buffer.from('<html>not an image</html>'), 'image/png', 'fake.png');
  assert.equal(res.status, 400);
  assert.match((await res.json()).error, /不是有效图片/);

  // 超过 5MB → 413
  res = await upload(Buffer.alloc(6 * 1024 * 1024, 1), 'image/png', 'big.png');
  assert.equal(res.status, 413);
});

// ---------- 限流 ----------

test('登录限流：第 11 次尝试返回 429', async () => {
  const app = createApp(); // 独立实例，避免与其他用例共享计数器
  await new Promise((resolve) => {
    server.close(() => {
      server = app.listen(0, '127.0.0.1', resolve);
    });
  });
  base = `http://127.0.0.1:${server.address().port}/api`;

  let last;
  for (let i = 0; i < 11; i++) {
    last = await api('POST', '/auth/login', { username: 'admin', password: 'wrong' });
  }
  assert.equal(last.status, 429);
  assert.match(last.body.error, /频繁/);
});
