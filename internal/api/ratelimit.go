package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// rateLimiter 是基于 IP 的固定窗口限流器。
type rateLimiter struct {
	mu       sync.Mutex
	window   time.Duration
	limit    int
	now      func() time.Time
	hits     map[string]int
	windowAt map[string]time.Time
}

// newRateLimiter 创建限流器（每 window 内每个 IP 最多 limit 次）。
func newRateLimiter(window time.Duration, limit int) *rateLimiter {
	return newRateLimiterWithClock(window, limit, time.Now)
}

// newRateLimiterWithClock 便于测试注入时钟。
func newRateLimiterWithClock(window time.Duration, limit int, now func() time.Time) *rateLimiter {
	return &rateLimiter{
		window:   window,
		limit:    limit,
		now:      now,
		hits:     make(map[string]int),
		windowAt: make(map[string]time.Time),
	}
}

// allow 判断该 IP 是否允许本次请求；到达窗口边界时自动重置计数。
func (r *rateLimiter) allow(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	if start := r.windowAt[ip]; start.IsZero() || now.Sub(start) >= r.window {
		r.windowAt[ip] = now
		r.hits[ip] = 1
		return true
	}
	if r.hits[ip] >= r.limit {
		return false
	}
	r.hits[ip]++
	return true
}

// rateLimitMiddleware 对超限请求返回 429。
func rateLimitMiddleware(rl *rateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.allow(c.ClientIP()) {
			c.JSON(http.StatusTooManyRequests, gin.H{"message": "请求过于频繁，请稍后再试"})
			c.Abort()
			return
		}
		c.Next()
	}
}
