package redis

import (
	"context"
	"log"
	"time"

	"narratpage/internal/config"

	"github.com/redis/go-redis/v9"
)

// Client 可选 Redis 客户端封装。
type Client struct {
	rdb *redis.Client
}

// New 根据配置创建 Redis 客户端；未启用时返回 nil。
func New(cfg config.Config) (*Client, error) {
	if !cfg.RedisEnabled || cfg.RedisURL == "" {
		return nil, nil
	}
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, err
	}
	rdb := redis.NewClient(opt)
	// 快速连通性检查
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	log.Printf("[redis] 已连接: %s", cfg.RedisURL)
	return &Client{rdb: rdb}, nil
}

// RDB 返回底层客户端（nil 安全）。
func (c *Client) RDB() *redis.Client {
	if c == nil {
		return nil
	}
	return c.rdb
}

// Close 关闭连接。
func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}
