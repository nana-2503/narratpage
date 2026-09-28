# 多阶段构建：Go 编译产物 + alpine 运行（modernc.org/sqlite 纯 Go，无 CGO）
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.21
WORKDIR /app
ENV PORT=3000 DATA_DIR=/data
COPY --from=builder /out/server /app/server
RUN mkdir -p /data
EXPOSE 3000
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO /dev/null http://127.0.0.1:3000/api/health || exit 1
CMD ["/app/server"]
