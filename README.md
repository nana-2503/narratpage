# 叙页博客系统

Go (net/http) + SQLite/MySQL/PostgreSQL 后端，React (Vite + shadcn) 前端，Docker Compose 一键启动。

## 部署方式（三选一）

### 1. 双容器（默认，前端 nginx + 后端 API）

```bash
cp .env.example .env
docker compose up --build -d
```

### 2. 单容器一体化（一个镜像跑全部，适合小服务器）

```bash
cp .env.example .env
docker compose -f docker-compose.allinone.yml up --build -d
```

后端 Express 直接托管前端构建产物，无 nginx 依赖。

### 3. 纯 docker run（不依赖 compose）

```bash
docker build -t blog .
docker run -d -p 8080:3000 -v blog-data:/data \
  -e JWT_SECRET=your-secret -e ADMIN_USERNAME=admin -e ADMIN_PASSWORD=your-password \
  --name blog blog
```

### 4. 前后端分离部署

前端是纯静态产物，可扔到任意静态托管（OSS/CDN、Vercel、Netlify）：

```bash
cd frontend
VITE_API_BASE=https://api.example.com npm run build   # 指向独立后端
# 将 dist/ 上传到静态托管即可
```

后端单独部署：

```bash
cd backend && docker build -t blog-api .
docker run -d -p 3000:3000 -v blog-data:/data -e JWT_SECRET=your-secret blog-api
```

## 数据库

### SQLite（默认）

无需额外配置，数据存储在卷的 `/data/blog.db`。

```bash
# .env 保持默认即可
DB_TYPE=sqlite
```

### MySQL

使用 Docker Compose 一键启动 MySQL：

```bash
docker compose --profile mysql up --build -d
```

或使用外部 MySQL：

```env
DB_TYPE=mysql
DB_DSN=user:password@tcp(db-host:3306)/blog?charset=utf8mb4&parseTime=true&loc=Local
```

### PostgreSQL

使用 Docker Compose 一键启动 PostgreSQL：

```bash
docker compose --profile pgsql up --build -d
```

或使用外部 PostgreSQL：

```env
DB_TYPE=pgsql
DB_DSN=postgres://user:password@db-host:5432/blog?sslmode=disable
```

> 三种数据库的数据互不通用；迁移时需导出数据并导入新数据库。

## Redis（可选）

Redis 用于登录限流和多实例部署。启用方式：

```bash
docker compose --profile redis up --build -d
```

或连接外部 Redis：

```env
REDIS_ENABLED=true
REDIS_URL=redis://redis-host:6379
```

## 访问

- 站点：http://localhost:8080（方式 1/2）或 http://localhost:3000（方式 3/4 后端直跑）
- 管理后台：`/admin`（默认 admin / admin123，首次登录后请尽快修改）
- RSS 订阅：`/api/rss.xml`（最新 20 篇已发布文章，链接地址由 `SITE_URL` 决定）

## 首次使用

首次访问管理后台时，系统会检测是否已安装。如果未安装，将自动跳转到安装向导页面。

安装向导将引导你：
1. 选择数据库类型（SQLite / MySQL / PostgreSQL）
2. 配置数据库连接信息
3. 设置管理员账号
4. （可选）启用 Redis

## 功能

- 公开站点：文章列表（分类筛选 / 标题搜索 / 分页）、Markdown 正文、
  上下篇导航、阅读时长、SEO meta、明暗主题切换
- 管理后台：文章编辑（Tiptap 富文本编辑器，Markdown 双向转换、实时预览）、
  草稿/发布、评论审核、分类管理、账号密码修改
- 安全：登录限流（同 IP 每分钟 10 次，支持 Redis 多实例共享）、JWT 认证、评论审核机制
- 多数据库：支持 SQLite / MySQL / PostgreSQL，通过环境变量 `DB_TYPE` 切换

## 常用命令

```bash
docker compose logs -f          # 查看日志
docker compose ps               # 查看状态
docker compose down             # 停止
docker compose down -v          # 停止并清空数据卷
```

## 测试

后端自带 API 测试（Go testing，无额外依赖）：

```bash
cd backend && go test ./...
```

覆盖认证、文章 CRUD/搜索转义/权限隔离、分类、评论审核流程、登录限流、图片上传，
使用独立临时数据库，不影响数据卷中的数据。

## 数据

- SQLite 数据库文件位于数据卷的 `/data/blog.db`，容器重建不丢失
- MySQL/PostgreSQL 数据位于对应服务的卷中
- 编辑器上传的图片位于数据卷的 `/data/uploads/`（随机文件名，匿名可读，同样持久化）

## 本地开发

```bash
# 后端 (:3000)
cd backend && go run ./cmd/server

# 前端 (:5173，/api 自动代理到 3000)
cd frontend && npm install && npm run dev
```

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `DB_TYPE` | 数据库类型：`sqlite` / `mysql` / `pgsql` | `sqlite` |
| `DB_DSN` | 数据库连接字符串 | 空（SQLite 使用文件） |
| `REDIS_ENABLED` | 是否启用 Redis | `false` |
| `REDIS_URL` | Redis 连接地址 | 空 |
| `JWT_SECRET` | JWT 签名密钥 | `dev-only-secret-change-me` |
| `ADMIN_USERNAME` | 管理员用户名 | `admin` |
| `ADMIN_PASSWORD` | 管理员密码 | `admin123` |
| `SITE_URL` | 站点对外地址 | `http://localhost:8080` |

## 结构

```
backend/    Go 标准库 net/http + modernc.org/sqlite（纯 Go 无 CGO），
            JWT 管理员认证，文章/分类/评论 API
            cmd/server/       入口（优雅退出）；internal/ 按关注点分层
            （api 路由与 handler / auth / ratelimit / seed / db / config / dialect / redis）
            internal/api/*_test.go  API 测试（go test）
frontend/   Vite + React + Tailwind + shadcn，公开站点 + 管理后台
```

> 安全说明：登录接口限流（同 IP 每分钟 10 次，Redis 模式下支持多实例）；生产环境务必通过
> `JWT_SECRET` / `ADMIN_PASSWORD` 环境变量覆盖默认值。
