package api

import (
	"time"

	"narratpage/internal/ratelimit"
)

// newCommentLimiter 创建评论限流器：同 IP 每 10 秒最多 5 次。
//
// 与登录限流分开维护，避免正常浏览+评论的用户消耗掉登录配额。
func newCommentLimiter() *ratelimit.FixedWindowLimiter {
	return ratelimit.NewFixedWindow(10*time.Second, 5)
}
