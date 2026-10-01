// Package ratelimit 提供请求限流。
package ratelimit

import (
	"net/http"
	"sync"
	"time"
)

// FixedWindowLimiter 内存固定窗口限流器。
//
// 定位：区分「每 IP 独立配额」与「全局配额」两类场景。
// 固定窗口在窗口边界可能瞬时放行 2 倍请求，对登录/评论这类
// 非资金场景足够，且实现与内存占用都远小于滑动窗口。
type FixedWindowLimiter struct {
	mu      sync.Mutex
	buckets map[string]int
	window  time.Duration
	max     int
	last    time.Time
}

// NewFixedWindow 创建限流器。
func NewFixedWindow(window time.Duration, max int) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		buckets: make(map[string]int),
		window:  window,
		max:     max,
		last:    time.Now(),
	}
}

// AllowKey 判断该键是否应被放行并记一次命中。
func (l *FixedWindowLimiter) AllowKey(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	// 窗口前移时整体清空：单实例小规模场景下比逐键过期更省内存，
	// 代价是同一键的计数会被一并重置，对防刷用途无实质影响。
	if now.Sub(l.last) >= l.window {
		l.buckets = make(map[string]int)
		l.last = now
	}
	l.buckets[key]++
	return l.buckets[key] <= l.max
}

// Allow 判断该请求是否应被放行（信任代理头的 X-Forwarded-For）。
func (l *FixedWindowLimiter) Allow(r *http.Request) bool {
	return l.AllowKey(ClientIP(r, true))
}
