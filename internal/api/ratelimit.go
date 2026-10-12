package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"shop/internal/logx"
	"shop/internal/redisx"
)

// Limiter 是限流器接口，中间件只依赖它。
type Limiter interface {
	Allow(ip string) bool
}

// rateLimiter 是基于 IP 的固定窗口限流器（内存实现，作为 Redis 不可用时的兜底）。
type rateLimiter struct {
	mu       sync.Mutex
	window   time.Duration
	limit    int
	now      func() time.Time
	hits     map[string]int
	windowAt map[string]time.Time
}

// newRateLimiter 创建内存限流器（每 window 内每个 IP 最多 limit 次）。
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

// Allow 判断该 IP 是否允许本次请求；到达窗口边界时自动重置计数。
func (r *rateLimiter) Allow(ip string) bool {
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

// redisLimiter 是基于 Redis 固定窗口的限流器（多实例共享计数）。
type redisLimiter struct {
	client    *redisx.Client
	window    time.Duration
	limit     int
	keyPrefix string
}

func newRedisLimiter(client *redisx.Client, keyPrefix string, window time.Duration, limit int) *redisLimiter {
	return &redisLimiter{client: client, keyPrefix: keyPrefix, window: window, limit: limit}
}

// tryAllow 尝试放行；Redis 不可用或出错时返回 error，由 fallbackLimiter 兜底。
func (r *redisLimiter) tryAllow(ip string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	n, err := r.client.IncrWithTTL(ctx, "rl:"+r.keyPrefix+":"+ip, r.window)
	if err != nil {
		return false, err
	}
	return n <= int64(r.limit), nil
}

// fallbackLimiter Redis 可用时走 Redis，否则回退到内存限流。
type fallbackLimiter struct {
	redis     *redisLimiter
	memory    Limiter
	available func() bool
	logErr    func(err error) // Redis 出错时记录（限频）；nil 表示不记录
}

func (f *fallbackLimiter) Allow(ip string) bool {
	if f.available() {
		if ok, err := f.redis.tryAllow(ip); err == nil {
			return ok
		} else if f.logErr != nil {
			f.logErr(err)
		}
	}
	return f.memory.Allow(ip)
}

// newFallbackLimiter 组装 Redis 限流与内存兜底：Redis 可用走 Redis，否则内存。
func newFallbackLimiter(client *redisx.Client, keyPrefix string, window time.Duration, limit int) Limiter {
	return &fallbackLimiter{
		redis:     newRedisLimiter(client, keyPrefix, window, limit),
		memory:    newRateLimiter(window, limit),
		available: client.Available,
		logErr:    newThrottledErrLogger(time.Now, 10*time.Second, logx.L().Warnf),
	}
}

// newThrottledErrLogger 返回限频的错误日志函数：同一 period 内最多记录一条，
// 避免 Redis 宕机期间每个请求都刷一条 warn。
func newThrottledErrLogger(now func() time.Time, period time.Duration, logf func(string, ...any)) func(err error) {
	var mu sync.Mutex
	var last time.Time
	return func(err error) {
		mu.Lock()
		defer mu.Unlock()
		t := now()
		if t.Sub(last) >= period {
			last = t
			logf("redis rate limiter error, falling back to in-memory: %v", err)
		}
	}
}

// rateLimitMiddleware 对超限请求返回 429。
func rateLimitMiddleware(l Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.Allow(c.ClientIP()) {
			c.JSON(http.StatusTooManyRequests, gin.H{"message": "请求过于频繁，请稍后再试"})
			c.Abort()
			return
		}
		c.Next()
	}
}
