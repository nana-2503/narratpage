package ratelimit

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter 限流器（内存 + 可选 Redis）。
type Limiter struct {
	window    time.Duration
	max       int
	message   string
	redisKey  string
	redisCli  *redis.Client
	mu        sync.Mutex
	buckets   map[string]*bucket
}

type bucket struct {
	count   int
	resetAt time.Time
}

// New 创建限流器；redisCli 为 nil 时使用纯内存实现。
func New(window time.Duration, max int, message string, redisCli *redis.Client, redisKey string) *Limiter {
	l := &Limiter{
		window:   window,
		max:      max,
		message:  message,
		redisKey: redisKey,
		redisCli: redisCli,
		buckets:  make(map[string]*bucket),
	}
	if l.redisCli == nil {
		go l.sweep()
	}
	return l
}

// Wrap 包装 handler：超限返回 429。
func (l *Limiter) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := clientIP(r)
		now := time.Now()

		var count int
		var retryAfter int

		if l.redisCli != nil {
			// Redis 滑动窗口实现
			ctx := context.Background()
			redisKey := l.redisKey + ":" + key
			pipe := l.redisCli.Pipeline()
			// 使用有序集合记录每次请求时间戳
			zKey := "ratelimit:" + redisKey
			pipe.ZRemRangeByScore(ctx, zKey, "0", strconv.FormatInt(now.Add(-l.window).UnixNano(), 10))
			pipe.ZAdd(ctx, zKey, redis.Z{Score: float64(now.UnixNano()), Member: now.UnixNano()})
			pipe.ZCard(ctx, zKey)
			pipe.Expire(ctx, zKey, l.window+time.Minute)
			cmds, err := pipe.Exec(ctx)
			if err == nil {
				for _, cmd := range cmds {
					if cmd != nil {
						if zc, ok := cmd.(*redis.IntCmd); ok {
							if n, err := zc.Result(); err == nil {
								count = int(n)
							}
						}
					}
				}
			}
			if count > l.max {
				retryAfter = int(l.window.Seconds()) + 1
			}
		} else {
			// 内存滑动窗口
			l.mu.Lock()
			b, ok := l.buckets[key]
			if !ok || b.resetAt.Before(now) {
				b = &bucket{resetAt: now.Add(l.window)}
				l.buckets[key] = b
			}
			b.count++
			count = b.count
			retryAfter = int(time.Until(b.resetAt).Seconds()) + 1
			l.mu.Unlock()
		}

		remaining := l.max - count
		if remaining < 0 {
			remaining = 0
		}
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		if count > l.max {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.Header().Set("content-type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": l.message})
			return
		}
		next(w, r)
	}
}

func (l *Limiter) sweep() {
	t := time.NewTicker(l.window)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		l.mu.Lock()
		for k, b := range l.buckets {
			if b.resetAt.Before(now) {
				delete(l.buckets, k)
			}
		}
		l.mu.Unlock()
	}
}

func clientIP(r *http.Request) string {
	addr := r.RemoteAddr
	if i := len(addr) - 1; i >= 0 {
		for j := len(addr) - 1; j >= 0; j-- {
			if addr[j] == ':' {
				return addr[:j]
			}
		}
	}
	return addr
}
