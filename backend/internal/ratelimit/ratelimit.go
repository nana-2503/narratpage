package ratelimit

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Limiter 极简内存滑动窗口限流（与 Node 版语义一致，无第三方依赖）。
// 单实例部署足够；多实例水平扩展时应换为共享存储方案。
type Limiter struct {
	window  time.Duration
	max     int
	message string

	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	count   int
	resetAt time.Time
}

func New(window time.Duration, max int, message string) *Limiter {
	l := &Limiter{
		window:  window,
		max:     max,
		message: message,
		buckets: make(map[string]*bucket),
	}
	go l.sweep()
	return l
}

// Wrap 包装 handler：超限返回 429
func (l *Limiter) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := clientIP(r)
		now := time.Now()

		l.mu.Lock()
		b, ok := l.buckets[key]
		if !ok || b.resetAt.Before(now) {
			b = &bucket{resetAt: now.Add(l.window)}
			l.buckets[key] = b
		}
		b.count++
		count := b.count
		retryAfter := int(time.Until(b.resetAt).Seconds()) + 1
		l.mu.Unlock()

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
