import bcrypt from 'bcryptjs';

const ADMIN_USERNAME = process.env.ADMIN_USERNAME || 'admin';
const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD || 'admin123';

const seedCategories = [
  { name: '技术', slug: 'tech' },
  { name: '生活', slug: 'life' },
  { name: '随笔', slug: 'notes' },
];

const seedPosts = [
  {
    title: '为什么选择 SQLite 作为博客数据库',
    slug: 'why-sqlite-for-blog',
    summary: '单文件、零运维、读多写少的场景下，SQLite 往往是博客系统最务实的后端存储选择。',
    content: `## 单文件的魅力

SQLite 把整个数据库放进一个文件，备份就是拷贝文件，迁移就是挂载卷。对个人博客这种读多写少的场景，它没有连接池调优，没有慢查询监控，运维成本趋近于零。

## 写放大不是问题

博客的写入集中在作者本人：一天几篇文章、几十条评论。WAL 模式下读写不互斥，这个量级绰绰有余。

## 什么时候该换

当出现多实例部署、写入并发超过百 QPS、或需要全文检索排名时，再迁移到 PostgreSQL 也不迟——届时数据导出只是一条 \`.dump\` 的事。`,
    category_slug: 'tech',
  },
  {
    title: 'Markdown 写作流水线',
    slug: 'markdown-writing-pipeline',
    summary: '从草稿到发布，一套基于纯文本的写作流程：本地编辑、版本管理、一键发布。',
    content: `## 纯文本优先

文章用 Markdown 书写，Git 做版本管理，发布只是推送到仓库。不依赖任何专有格式，十年后依然打得开。

## 结构即大纲

先列标题再填内容。H2 是章节，H3 是论点，写作前大纲先成立，文章就不会散。`,
    category_slug: 'notes',
  },
  {
    title: '重新开始写博客',
    slug: 'restart-blogging',
    summary: '清理掉收藏夹里的教程，关掉永远在配置的编辑器，先把第一篇发出去。',
    content: `## 完成比完美重要

搭博客的真正风险不是选错技术栈，而是把全部时间花在配置环境上。先让最小版本跑起来，剩下的在路上迭代。

## 写给未来的自己

保持记录的习惯。三年后回看，这些文字就是时间存在的证据。`,
    category_slug: 'life',
  },
];

export function seed(db) {
  const userCount = db.prepare('SELECT COUNT(*) AS n FROM users').get().n;
  if (userCount === 0) {
    const hash = bcrypt.hashSync(ADMIN_PASSWORD, 10);
    db.prepare('INSERT INTO users (username, password_hash) VALUES (?, ?)').run(
      ADMIN_USERNAME,
      hash
    );
    console.log(`[seed] 默认管理员已创建: ${ADMIN_USERNAME}`);
    if (ADMIN_PASSWORD === 'admin123') {
      console.warn('[seed] 警告: 正在使用默认密码 admin123，请尽快通过 ADMIN_PASSWORD 环境变量修改');
    }
  }

  const catCount = db.prepare('SELECT COUNT(*) AS n FROM categories').get().n;
  if (catCount === 0) {
    const insert = db.prepare('INSERT INTO categories (name, slug) VALUES (?, ?)');
    for (const c of seedCategories) insert.run(c.name, c.slug);
  }

  const postCount = db.prepare('SELECT COUNT(*) AS n FROM posts').get().n;
  if (postCount === 0) {
    const insert = db.prepare(`
      INSERT INTO posts (title, slug, summary, content, category_id, status, published_at)
      VALUES (?, ?, ?, ?, (SELECT id FROM categories WHERE slug = ?), 'published', datetime('now'))
    `);
    for (const p of seedPosts) {
      insert.run(p.title, p.slug, p.summary, p.content, p.category_slug);
    }
    console.log('[seed] 示例文章已写入');
  }

  const commentCount = db.prepare('SELECT COUNT(*) AS n FROM comments').get().n;
  if (commentCount === 0) {
    const insert = db.prepare(`
      INSERT INTO comments (post_id, author, content, status)
      VALUES ((SELECT id FROM posts WHERE slug = ?), ?, ?, ?)
    `);
    insert.run('why-sqlite-for-blog', '读者甲', '写得很好，已经准备把博客迁到 SQLite 了。', 'approved');
    insert.run('markdown-writing-pipeline', '路过乙', '大纲先行这点很认同。', 'approved');
    insert.run('restart-blogging', '游客丙', '占位待审核的一条评论。', 'pending');
  }
}
