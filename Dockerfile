# 一体化镜像：前端构建产物 + 后端 API，单容器运行
# 用法: docker build -t blog . && docker run -p 8080:3000 -v blog-data:/data blog

# ---- 前端构建 ----
FROM node:22-bookworm-slim AS web-build
WORKDIR /web
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci
COPY frontend/ .
# 前后端分离部署时可改为: RUN VITE_API_BASE=https://api.example.com npm run build
RUN npm run build

# ---- 后端依赖 ----
FROM node:22-bookworm-slim AS api-build
WORKDIR /api
RUN apt-get update \
  && apt-get install -y --no-install-recommends python3 make g++ ca-certificates \
  && rm -rf /var/lib/apt/lists/*
COPY backend/package.json backend/package-lock.json* ./
RUN npm install --omit=dev

# ---- 运行 ----
FROM node:22-bookworm-slim
WORKDIR /app
ENV NODE_ENV=production \
    PORT=3000 \
    DATA_DIR=/data
COPY --from=api-build /api/node_modules ./node_modules
COPY backend/package.json ./
COPY backend/src ./src
COPY --from=web-build /web/dist ./frontend/dist
RUN mkdir -p /data
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD node -e "fetch('http://127.0.0.1:'+(process.env.PORT||3000)+'/api/health').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"
CMD ["node", "src/index.js"]
