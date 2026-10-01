package ratelimit

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter 限流器（内存 + 可选 Redis 共享）。
type Limiter struct {
	window     time.Duration
	max        int
	message    string
	redisKey   string
	redisCli   *redis.Client
	mu         sync.Mutex
	buckets    map[string]*bucket
	trustProxy bool
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

// TrustProxy 声明后端位于可信反向代理之后。
//
// 重要：nginx 反向代理下所有请求的 RemoteAddr 都是代理容器自身的 IP，
// 若不解析 X-Forwarded-For，限流会退化为「全站共用一个配额」——
// 任意用户输错几次密码就会把其他所有人一起锁死。
func (l *Limiter) TrustProxy(v bool) *Limiter {
	l.trustProxy = v
	return l
}

// Allow 记一次命中并返回是否放行，以及剩余配额。
func (l *Limiter) allow(r *http.Request) (ok bool, remaining int, retryAfter int) {
	key := ClientIP(r, l.trustProxy)
	now := time.Now()
	max := l.max

	if l.redisCli != nil {
		count, err := l.redisAllow(key, now)
		if err != nil {
			// Redis 故障时降级为放行：可用性优先于限流精度
			return true, max, 0
		}
		if count > max {
			return false, 0, int(l.window.Seconds()) + 1
		}
		rem := max - count
		if rem < 0 {
			rem = 0
		}
		return true, rem, 0
	}

	l.mu.Lock()
	b, found := l.buckets[key]
	if !found || b.resetAt.Before(now) {
		b = &bucket{resetAt: now.Add(l.window)}
		l.buckets[key] = b
	}
	b.count++
	count := b.count
	ra := int(time.Until(b.resetAt).Seconds()) + 1
	l.mu.Unlock()

	if count > max {
		return false, 0, ra
	}
	rem := max - count
	if rem < 0 {
		rem = 0
	}
	return true, rem, 0
}

// redisAllow 用有序集合实现跨实例共享的滑动窗口。
//
// 键名形如 "login:<ip>"，可直接用 redis-cli --scan --pattern 'login:*'
// 观察。TTL 取窗口 + 1 分钟，保证窗口刚过就能清理。
func (l *Limiter) redisAllow(key string, now time.Time) (int, error) {
	ctx := context.Background()
	zKey := l.redisKey + ":" + key
	nowNano := now.UnixNano()

	pipe := l.redisCli.TxPipeline()
	pipe.ZRemRangeByScore(ctx, zKey, "0", strconv.FormatInt(now.Add(-l.window).UnixNano(), 10))
	pipe.ZAdd(ctx, zKey, redis.Z{Score: float64(nowNano), Member: nowNano})
	card := pipe.ZCard(ctx, zKey)
	pipe.Expire(ctx, zKey, l.window+time.Minute)

	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	n, err := card.Result()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// Wrap 包装 handler：超限返回 429。
func (l *Limiter) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ok, remaining, retryAfter := l.allow(r)
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		if !ok {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.Header().Set("content-type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": l.message})
			return
		}
		next(w, r)
	}
}

// Reset 清除某键的计数（登录成功后调用，避免正常用户被累积计数影响）。
func (l *Limiter) Reset(r *http.Request) {
	key := ClientIP(r, l.trustProxy)
	if l.redisCli != nil {
		l.redisCli.Del(context.Background(), l.redisKey+":"+key)
		return
	}
	l.mu.Lock()
	delete(l.buckets, key)
	l.mu.Unlock()
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

// ClientIP 提取客户端 IP。
//
// trustProxy 为 true 时优先取 X-Real-IP。该头由反向代理用
// $remote_addr 覆写，客户端无法伪造，因此是唯一可信的来源。
//
// 为什么不直接取 X-Forwarded-For 最左跳：nginx 的
// $proxy_add_x_forwarded_for 是「追加」而非「覆盖」，
// 客户端自带的头会被保留在前面，于是最左跳完全由客户端控制，
// 限流可被逐请求伪造 IP 轻易绕过。X-Forwarded-For 仅作为
// 缺少 X-Real-IP 时的兜底。
//
// trustProxy 为 false 时只认 RemoteAddr——此时 X-Real-IP 与
// X-Forwarded-For 都可能由客户端伪造。
func ClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
			return xr
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if i := indexByte(xff, ','); i > 0 {
				xff = xff[:i]
			}
			if ip := strings.TrimSpace(xff); ip != "" {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
