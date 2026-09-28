package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"narratpage/internal/api"
	"narratpage/internal/config"
	"narratpage/internal/db"
	"narratpage/internal/seed"
)

func main() {
	log.SetFlags(log.LstdFlags)
	cfg := config.Load()

	database, err := db.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("[blog] 数据库打开失败: %v", err)
	}
	defer database.Close()

	if err := seed.Run(database, cfg); err != nil {
		log.Fatalf("[blog] 种子数据失败: %v", err)
	}

	uploadDir := filepath.Join(cfg.DataDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		log.Fatalf("[blog] 上传目录创建失败: %v", err)
	}

	if cfg.FrontendDist == "" {
		cfg.FrontendDist = resolveFrontendDist()
	}

	handler := api.NewRouter(cfg, api.Deps{DB: database, UploadDir: uploadDir})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("[blog] API listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[blog] 服务异常: %v", err)
		}
	}()

	// 优雅退出：SIGINT/SIGTERM 时等待在途请求结束
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Print("[blog] 正在关闭…")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[blog] 关闭超时: %v", err)
	}
	database.Close()
}

// resolveFrontendDist 未显式指定时按可执行文件目录推断（一体化部署约定）
func resolveFrontendDist() string {
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "frontend", "dist")
		if fileExists(filepath.Join(candidate, "index.html")) {
			return candidate
		}
	}
	if fileExists(filepath.Join("frontend", "dist", "index.html")) {
		return filepath.Join("frontend", "dist")
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
