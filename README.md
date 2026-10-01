# 叙页博客系统

自托管博客系统，功能对标 WordPress 的核心子集。后端 Go（标准库 net/http，无框架、无 ORM），
前端 React + Vite + shadcn/ui，支持 SQLite / MySQL / PostgreSQL 三种数据库，Docker 一键启动。

## 功能

**内容**
- 文章与页面（`/post/:slug`、`/page/:slug` 两套 URL 空间）
- 分类、标签（支持合并去重）、自定义元数据
- 草稿 / 待审核 / 已发布 / 私密 / 回收站五种状态，定时发布，置顶
- 访问密码保护的文章
- 编辑历史：内容变更自动存档，可回滚到任意版本
- 全文搜索（标题 + 摘要 + 正文），LIKE 通配符按字面量转义
- 按年月 / 分类 / 标签 / 作者归档

**媒体**
- 图片与 PDF 上传，按 magic bytes 判型（不信任客户端 MIME）
- SVG 脚本内容过滤，媒体库元数据（标题 / 替代文本 / 说明）

**评论**
- 先审后发（可在站点设置关闭），两级嵌套回复
- 蜜罐字段 + 频率限流 + 同 IP 冷却，IP 以加盐哈希存储
- 批量审核 / 标记垃圾 / 删除

**账号**
- 五种角色（管理员 / 编辑 / 作者 / 贡献者 / 订阅者），后端 RBAC 鉴权
- 多用户、会话列表、「登出所有设备」��改密后自动踢出其它会话

**站点**
- 后台可改站点标题 / 描述 / 每页条数 / 导航链接（无需重启容器）
- RSS 2.0 + Atom 1.0 订阅、sitemap.xml、robots.txt
- 301 重定向规则（改 slug 或迁移路径时保住外链）
- 明暗主题、圆角可调、JSON-LD 结构化数据与 OG 标签

## 部署

### 双容器（默认，前端 nginx + 后端 API）

```bash
cp .env.example .env
docker compose up --build -d
```

站点 http://localhost:8080 ，后台 `/admin`。

### 单容器一体化（一个镜像跑全部，适合小服务器）

```bash
cp .env.example .env
docker compose -f docker-compose.allinone.yml up --build -d
```

后端直接托管前端构建产物并处理 SPA 路由回退，不依赖 nginx。

### 纯 docker run

```bash
docker build -t blog .
docker run -d -p 8080:3000 -v blog-data:/data \
  -e JWT_SECRET=your-secret -e ADMIN_PASSWORD=your-password \
  --name blog blog
```

### 前后端分离

前端是纯静态产物，可部署到任意静态托管（CDN / Vercel / Netlify）：

```bash
cd frontend
VITE_API_BASE=https://api.example.com npm run build
# 将 dist/ 上传到静态托管
```

后端单独部署：

```bash
cd backend && docker build -t blog-api .
docker run -d -p 3000:3000 -v blog-data:/data -e JWT_SECRET=your-secret blog-api
```

前后端分离时后端已开启 CORS，无需额外配置。

## 数据库

三种数据库的数据互不通用，迁移需自行导出导入。

```bash
# SQLite（默认）：数据在卷的 /data/blog.db，零配置
docker compose up -d

# MySQL：内置 profile
docker compose -f docker-compose.yml -f docker-compose.mysql.yml --profile mysql up --build -d

# PostgreSQL：内置 profile
docker compose -f docker-compose.yml -f docker-compose.pgsql.yml --profile pgsql up --build -d
```

`DB_TYPE` 在进程启动时读取，切换数据库需重启容器。

### 关于多数据库支持

三库差异（占位符、自增写法、布尔类型、文本默认值、LIKE 转义、upsert 语法、
索引幂等创建）全部收敛在 `internal/dialect` 与 `internal/db` 两个包内，
业务代码只写一份 SQL。跨库行为由测试覆盖，需要真实的 MySQL / PostgreSQL 实例：

```bash
docker run -d --name np-mysql -e MYSQL_ROOT_PASSWORD=rootpass \
  -e MYSQL_DATABASE=blog -e MYSQL_USER=narratpage -e MYSQL_PASSWORD=narratpage \
  -p 13306:3306 mysql:8.0
docker run -d --name np-pg -e POSTGRES_USER=narratpage -e POSTGRES_PASSWORD=narratpage \
  -e POSTGRES_DB=blog -p 15432:5432 postgres:16-alpine

export NP_TEST_MYSQL_DSN='narratpage:narratpage@tcp(127.0.0.1:13306)/blog?charset=utf8mb4&parseTime=true&loc=Local'
export NP_TEST_PG_DSN='postgres://narratpage:narratpage@127.0.0.1:15432/blog?sslmode=disable'
cd backend && go test ./...
```

未设置这两个变量时相关用例自动跳过，SQLite 用例始终运行。

## Redis（可选）

Redis 仅用于登录限流的多实例共享；单实例部署不必启用，内存限流已足够。

```bash
docker compose -f docker-compose.yml -f docker-compose.redis.yml --profile redis up --build -d
```

## 首次安装

首次访问 `/admin` 时若数据库尚无用户，会引导至安装向导：
选择数据库 → 填写连接信息 → 设置管理员账号 → （可选）启用 Redis。

安装向导会**实际创建管理员账号并写入站点设置**。若向导中选择了与当前进程不同的
数据库类型，则无法运行期切换，此时向导会输出可直接复制的 `.env` 片段，
需写入后重启容器再登录。

## 安全

- 登录接口限流：同 IP 每分钟 10 次；Redis 模式下跨实例共享
- **反向代理注意事项**：位于 nginx 之后须设 `TRUST_PROXY=true` 才能按真实 IP 限流；
  直接对外时必须为 `false`，否则客户端可伪造 `X-Forwarded-For` 绕过限流
- JWT + bcrypt（cost 10）；会话登记在 `sessions` 表，支持登出即失效
- SQL 全参数化；LIKE 通配符转义
- 上传按文件头判型、随机文件名、按年月分目录、5–10MB 限制、SVG 脚本过滤
- 最后一个管理员不能被降级、停用或删除
- 自助修改资料接口不接受 role 字段，避免越权提权
- 评论 IP 以加盐哈希存储，不保留原始地址

> 生产环境务必通过 `JWT_SECRET` / `ADMIN_PASSWORD` 覆盖默认值。

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `PORT` | 监听端口 | `3000` |
| `DATA_DIR` | 数据目录（数据库与上传文件） | `data` |
| `JWT_SECRET` | JWT 签名密钥 | `dev-only-secret-change-me` |
| `JWT_EXPIRES_IN` | token 有效期 | `168h` |
| `ADMIN_USERNAME` | 初始管理员用户名 | `admin` |
| `ADMIN_PASSWORD` | 初始管理员密码 | `admin123` |
| `SITE_URL` | 站点对外地址（环境变量兜底，后台可覆盖） | `http://localhost:8080` |
| `TRUST_PROXY` | 是否信任 `X-Forwarded-For` | `false` |
| `DB_TYPE` | `sqlite` / `mysql` / `pgsql` | `sqlite` |
| `DB_DSN` | 数据库连接串 | 空 |
| `REDIS_ENABLED` | 是否启用 Redis | `false` |
| `REDIS_URL` | Redis 地址 | 空 |
| `FRONTEND_DIST` | 前端产物目录（一体化部署自动推断） | 自动 |

## 常用命令

```bash
docker compose logs -f          # 查看日志
docker compose ps               # 查看状态
docker compose down             # 停止
docker compose down -v          # 停止并清空数据卷
```

## 测试

```bash
cd backend && go test ./...     # 含跨库用例（需按上文配置 DSN）
cd frontend && npm run build    # 类型检查 + 构建
```

后端测试覆盖：建表与三库方言差异、插入取 ID 的跨库分支、LIKE 转义、
权限矩阵、可见性规则（草稿/私密/回收站）、回收站与修订回滚、评论审核与嵌套、
上传类型判定与 SVG 过滤、限流 IP 解析、评论树构造、slug 稳定性。

## 本地开发

```bash
# 后端 (:3000)
cd backend && go run ./cmd/server

# 前端 (:5173，/api 自动代理到 3000)
cd frontend && npm install && npm run dev
```

## 结构

```
backend/
  cmd/server/          入口、依赖装配、优雅退出
  internal/
    dialect/           三库差异：占位符、自增、类型、DDL 构造器
    db/                连接池、幂等迁移、跨库查询辅助、存量库补列
    repo/              仓储层：内容、评论、分类标签、用户、设置、媒体
    models/            领域模型与 API 契约类型
    api/               路由与 handler，按关注点分文件
    auth/              JWT、bcrypt、角色与权限矩阵
    ratelimit/         限流：滑动窗口（Redis）+ 固定窗口（内存）
    config/ httpx/ redis/ seed/
  internal/api/*_test.go       API 集成测试（httptest + 真实数据库）
  internal/repo/*_test.go      仓储单元测试
frontend/src/
  pages/              公开站 6 个 + 后台 15 个
  components/         编辑器、表单与后台通用组件 + shadcn/ui
  lib/                API 契约、Markdown、SEO、外观设置
  hooks/              use-auth / use-site / use-async / use-selection
```

## 许可

MIT
