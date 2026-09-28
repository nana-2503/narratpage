# 叙页博客系统

Node.js (Express) + SQLite 后端，React (Vite + shadcn) 前端，Docker Compose 一键启动。

## 部署方式（三选一）

### 1. 双容器（默认，前端 nginx + 后端 API）

```bash
cp .env.example .env
docker compose up --build -d
```

### 2. 单容器一体化（一个镜像跑全部，适合小服务器）

```bash
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

> 三种方式的 SQLite 数据都在卷里，互不通用；迁移时拷贝 `blog.db` 即可。

## 访问

- 站点：http://localhost:8080（方式 1/2）或 http://localhost:3000（方式 3/4 后端直跑）
- 管理后台：`/admin`（默认 admin / admin123，首次登录后请尽快修改）

## 常用命令

```bash
docker compose logs -f          # 查看日志
docker compose ps               # 查看状态
docker compose down             # 停止
docker compose down -v          # 停止并清空 SQLite 数据
```

## 测试

后端自带 API 测试（node:test，无额外依赖）：

```bash
cd backend && npm test
```

覆盖认证、文章 CRUD/搜索转义/权限隔离、分类、评论审核流程、登录限流，
使用独立临时数据库，不影响 `sqlite-data` 卷中的数据。

## 数据

SQLite 数据库文件位于 `sqlite-data` 命名卷的 `/data/blog.db`，容器重建不丢失。

## 本地开发

```bash
# 后端 (:3000)
cd backend && npm install && npm run dev

# 前端 (:5173，/api 自动代理到 3000)
cd frontend && npm install && npm run dev
```

## 结构

```
backend/    Express + better-sqlite3，JWT 管理员认证，文章/分类/评论 API
            src/app.js 组装应用（可测试），src/rate-limit.js 登录限流
            test/      API 测试（node:test）
frontend/   Vite + React + Tailwind + shadcn，公开站点 + 管理后台
```

> 安全说明：登录接口限流（同 IP 每分钟 10 次）；生产环境务必通过
> `JWT_SECRET` / `ADMIN_PASSWORD` 环境变量覆盖默认值。
