// NarratPage —— 叙页博客系统服务端。
//
// 启动流程：加载配置 → 打开数据库并迁移 → 写入默认设置与示例数据
// → 按需连接 Redis → 组装路由 → 监听端口 → 等待信号优雅退出。
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
	"narratpage/internal/redis"
	"narratpage/internal/repo"
	"narratpage/internal/seed"
)

func main() {
	log.SetFlags(log.LstdFlags)
	cfg := config.Load()

	if err := run(cfg); err != nil {
		log.Fatalf("[blog] 启动失败: %v", err)
	}
}

func run(cfg config.Config) error {
	// 数据目录需先于数据库存在
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}

	database, err := db.Open(cfg)
	if err != nil {
		return err
	}
	defer database.Close()

	// 首次启动：写入示例数据与默认设置
	installed, err := api.CheckInstalled(database)
	if err != nil {
		return err
	}
	if !installed {
		if err := seed.Run(database, cfg); err != nil {
			return err
		}
	}

	options := repo.NewOptions(database, cfg.DBType)
	if err := options.EnsureDefaults(); err != nil {
		return err
	}

	redisCli, err := redis.New(cfg)
	if err != nil {
		return err
	}
	if redisCli != nil {
		defer redisCli.Close()
	}

	uploadDir := filepath.Join(cfg.DataDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return err
	}

	if cfg.FrontendDist == "" {
		cfg.FrontendDist = resolveFrontendDist()
	}

	handler := api.NewRouter(cfg, api.Deps{
		DB:         database,
		DBType:     cfg.DBType,
		UploadDir:  uploadDir,
		SiteURL:    cfg.SiteURL,
		JWTSecret:  cfg.JWTSecret,
		JWTExpiry:  cfg.JWTExpiry,
		TrustProxy: cfg.TrustProxy,
		Redis:      redisCli,

		Posts:     repo.New(database, cfg.DBType),
		Options:   options,
		Users:     repo.NewUsers(database, cfg.DBType),
		Taxonomy:  repo.NewTaxonomy(database, cfg.DBType),
		Media:     repo.NewMedia(database, cfg.DBType),
		PostMeta:  repo.NewPostMeta(database, cfg.DBType),
		Revisions: repo.NewRevisions(database, cfg.DBType),
		Redirects: repo.NewRedirects(database, cfg.DBType),
		Comments:  repo.NewComments(database, cfg.DBType),
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		// 写超时留空：上传大文件与长连接不应被服务端截断
		IdleTimeout: 120 * time.Second,
	}

	go func() {
		log.Printf("[blog] 数据库: %s", cfg.DBType)
		log.Printf("[blog] 监听 :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[blog] 服务异常: %v", err)
		}
	}()

	// 优雅退出：收到 SIGINT/SIGTERM 后等待在途请求结束
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Print("[blog] 正在关闭…")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[blog] 关闭超时: %v", err)
	}
	if err := database.Close(); err != nil {
		log.Printf("[blog] 数据库关闭异常: %v", err)
	}
	return nil
}

// resolveFrontendDist 未显式指定时按可执行文件目录推断（一体化部署约定）。
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
