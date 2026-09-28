# 一体化镜像：前端构建产物 + 后端 API（Go），单容器运行
# 用法: docker build -t blog . && docker run -p 8080:3000 -v blog-data:/data blog

# ---- 前端构建 ----
FROM node:22-bookworm-slim AS web-build
WORKDIR /web
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci
COPY frontend/ .
# 前后端分离部署时可改为: RUN VITE_API_BASE=https://api.example.com npm run build
RUN npm run build

# ---- 后端编译（纯 Go，无 CGO）----
FROM golang:1.23-alpine AS api-build
WORKDIR /api
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /api-server ./cmd/server

# ---- 运行 ----
FROM alpine:3.21
WORKDIR /app
ENV PORT=3000 DATA_DIR=/data
COPY --from=api-build /api-server /app/server
COPY --from=web-build /web/dist /app/frontend/dist
RUN mkdir -p /data
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO /dev/null http://127.0.0.1:3000/api/health || exit 1
CMD ["/app/server"]
