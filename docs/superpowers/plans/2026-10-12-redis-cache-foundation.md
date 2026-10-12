# Redis 缓存基础层 + 限流接入 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给 shop 后端引入 Redis 访问基础层，并把登录/注册限流升级为「Redis 固定窗口 + 内存兜底降级」。

**Architecture:** 新增 `internal/redisx` 包封装 go-redis（连接、健康探测、原子 `INCR`+`EXPIRE`），限流逻辑留在 `internal/api`，抽 `Limiter` 接口，实现内存 `rateLimiter`（保留现有逻辑）、`redisLimiter`、`fallbackLimiter`（按可用性切换）；`server.go` 用环境变量构建。

**Tech Stack:** Go 1.26、gin、gorm、`github.com/redis/go-redis/v9`（运行）、`github.com/alicebob/miniredis/v2`（测试）。

**Spec:** `docs/superpowers/specs/2026-10-12-redis-cache-foundation-design.md`

## Global Constraints

- `REDIS_ADDR` 默认 `192.168.57.47:6379`、`REDIS_PASSWORD` 默认空、`REDIS_DB` 默认 `0`（环境变量，缺省可用）。
- 降级语义：Redis 未配置/不可用时**回退内存限流**（不 fail-open、不 fail-closed）。
- v1 **不自动重连**：启动时 Ping 一次，失败即全程降级直到进程重启。
- 限流算法为**固定窗口**（`INCR` + 首次 `EXPIRE`），与现有内存限流语义一致；不引入滑动窗口。
- 每个 Redis 操作带 **200ms 超时**，避免拖慢登录请求。
- 服务启动不得因 Redis 不可用而 panic 或退出。

## Review Focus

1. **Redis 启动后中途宕机**（`available` 固定为 true 但操作报错）→ 必须落到内存兜底，登录仍可用。→ Task 2 `TestFallbackLimiterDegradesOnRedisError`。
2. **TTL 小于 1 秒**时 `EXPIRE 0` 在 Redis 语义里是「删除 key」，会导致固定窗口失效、无限放行 → `IncrWithTTL` 保底 1s。→ Task 1 `TestIncrWithTTLShortTTL`。
3. **login 与 register 两个 scope 的 key 隔离**（前缀不同），否则登录和注册会共用计数。→ Task 2 `TestRedisLimiterKeyIsolation`。
4. **Redis 地址不可达**时 `redisx.New` 不 panic、返回 `Available()==false`。→ Task 1 `TestNewUnavailable`。
5. **接口化后中间件行为不变**：降级（内存路径）下超限请求仍返回 429。→ Task 2 `TestRateLimitMiddlewareRejectsOverLimit`。

---

### Task 1: `internal/redisx` 基础层

**Files:**
- Create: `internal/redisx/redisx.go`
- Test: `internal/redisx/redisx_test.go`
- Modify: `go.mod`、`go.sum`（`go get` / `go mod tidy`）

**Interfaces:**
- Consumes: 无（本任务无前置依赖）。
- Produces: `redisx.Config`、`redisx.ConfigFromEnv() Config`、`redisx.New(cfg Config) *Client`、`(*Client).Available() bool`、`(*Client).IncrWithTTL(ctx, key string, ttl time.Duration) (int64, error)`、`redisx.ErrUnavailable`。后续任务（Task 2/3）依赖这些名字与签名。

- [ ] **Step 1: 添加依赖**

```bash
go get github.com/redis/go-redis/v9@latest github.com/alicebob/miniredis/v2@latest
```

- [ ] **Step 2: 写失败测试**

创建 `internal/redisx/redisx_test.go`：

```go
package redisx

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestIncrWithTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	client := New(Config{Addr: mr.Addr()})
	if !client.Available() {
		t.Fatal("client should be available against miniredis")
	}
	ctx := context.Background()

	n, err := client.IncrWithTTL(ctx, "k", time.Minute)
	if err != nil || n != 1 {
		t.Fatalf("first incr: got %d, %v; want 1, nil", n, err)
	}
	if n, _ = client.IncrWithTTL(ctx, "k", time.Minute); n != 2 {
		t.Fatalf("second incr: got %d; want 2", n)
	}
	if ttl := mr.TTL("k"); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("ttl: got %v; want >0 and <=1m", ttl)
	}
}

func TestIncrWithTTLShortTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	client := New(Config{Addr: mr.Addr()})
	// ttl 不足 1s 时保底 1s，避免 EXPIRE 0 立即删除导致窗口失效
	if _, err := client.IncrWithTTL(context.Background(), "k", time.Millisecond); err != nil {
		t.Fatalf("short ttl incr: %v", err)
	}
	if ttl := mr.TTL("k"); ttl <= 0 {
		t.Fatalf("short ttl should still set expiry, got %v", ttl)
	}
}

func TestNewUnavailable(t *testing.T) {
	// 端口 1 无监听，连接立即被拒绝
	client := New(Config{Addr: "127.0.0.1:1"})
	if client.Available() {
		t.Fatal("client should be unavailable against closed port")
	}
	if _, err := client.IncrWithTTL(context.Background(), "k", time.Minute); err != ErrUnavailable {
		t.Fatalf("IncrWithTTL when unavailable: got %v; want ErrUnavailable", err)
	}
}
```

- [ ] **Step 3: 运行测试，确认失败**

Run: `go test ./internal/redisx/ -v`
Expected: FAIL（`undefined: New` / `undefined: Config` / `undefined: ErrUnavailable`）

- [ ] **Step 4: 写最小实现**

创建 `internal/redisx/redisx.go`：

```go
package redisx

import (
	"context"
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrUnavailable 表示 Redis 未配置或不可用，调用方应据此降级。
var ErrUnavailable = errors.New("redis unavailable")

// Config 是 Redis 连接配置。
type Config struct {
	Addr     string
	Password string
	DB       int
}

// ConfigFromEnv 从环境变量读取配置：REDIS_ADDR / REDIS_PASSWORD / REDIS_DB。
func ConfigFromEnv() Config {
	db, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
	return Config{
		Addr:     envOr("REDIS_ADDR", "localhost:6379"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       db,
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Client 是 go-redis 客户端的薄封装：启动时探测一次可用性，
// 之后 Redis 不可用时所有操作返回 ErrUnavailable，由调用方降级。
type Client struct {
	rdb       *redis.Client
	available bool
}

// New 创建客户端并 Ping 一次确认可用性；失败不返回错误，仅标记不可用。
func New(cfg Config) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	c := &Client{rdb: rdb}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err == nil {
		c.available = true
	}
	return c
}

// Available 返回 Redis 当前是否可用（启动时探测的结果）。
func (c *Client) Available() bool { return c.available }

// incrWithTTLScript 原子地 INCR 并在首次创建时设置过期时间，用于固定窗口计数。
var incrWithTTLScript = redis.NewScript(`
local c = redis.call('INCR', KEYS[1])
if c == 1 then
	redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return c
`)

// IncrWithTTL 对 key 计数 +1，并在首次创建时设置 ttl 过期；返回自增后的值。
func (c *Client) IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	if !c.available {
		return 0, ErrUnavailable
	}
	if ttl < time.Second {
		ttl = time.Second
	}
	return incrWithTTLScript.Run(ctx, c.rdb, []string{key}, int(ttl.Seconds())).Int64()
}
```

- [ ] **Step 5: 运行测试，确认通过**

Run: `go test ./internal/redisx/ -v`
Expected: PASS（3 个测试全过）

- [ ] **Step 6: 整理依赖并提交**

```bash
go mod tidy
git add go.mod go.sum internal/redisx/redisx.go internal/redisx/redisx_test.go
git commit -m "feat: 新增 redisx 基础层（连接 + IncrWithTTL）"
```

---

### Task 2: 限流接口 + Redis/内存双实现

**Files:**
- Modify: `internal/api/ratelimit.go`（抽接口、重命名 `allow`→`Allow`、加 redisLimiter/fallbackLimiter、改中间件签名）
- Modify: `internal/api/ratelimit_test.go`（`allow`→`Allow`、加中间件 429 测试）
- Test: `internal/api/ratelimit_redis_test.go`（新增）

**Interfaces:**
- Consumes: `redisx.Client`、`redisx.New`、`redisx.Config`、`(*Client).Available()`、`(*Client).IncrWithTTL`（来自 Task 1）。
- Produces: `api.Limiter` 接口 `Allow(ip string) bool`；`api.newFallbackLimiter(client *redisx.Client, keyPrefix string, window time.Duration, limit int) Limiter`；`api.rateLimitMiddleware(l Limiter) gin.HandlerFunc`。Task 3 依赖这些。

- [ ] **Step 1: 写失败测试（新文件）**

创建 `internal/api/ratelimit_redis_test.go`：

```go
package api

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"shop/internal/redisx"
)

func TestRedisLimiterFixedWindow(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redisx.New(redisx.Config{Addr: mr.Addr()})
	rl := newRedisLimiter(client, "test", time.Minute, 3)
	ip := "1.2.3.4"

	for i := 0; i < 3; i++ {
		ok, err := rl.tryAllow(ip)
		if err != nil || !ok {
			t.Fatalf("request %d should be allowed, got ok=%v err=%v", i+1, ok, err)
		}
	}
	if ok, _ := rl.tryAllow(ip); ok {
		t.Fatal("4th request within window should be rejected")
	}
	// 窗口过后重置
	mr.FastForward(time.Minute)
	if ok, err := rl.tryAllow(ip); err != nil || !ok {
		t.Fatalf("after window should be allowed again, got ok=%v err=%v", ok, err)
	}
	// 不同 IP 互不影响
	if ok, _ := rl.tryAllow("5.6.7.8"); !ok {
		t.Fatal("different IP should be allowed")
	}
}

func TestRedisLimiterKeyIsolation(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redisx.New(redisx.Config{Addr: mr.Addr()})
	login := newRedisLimiter(client, "login", time.Minute, 3)
	register := newRedisLimiter(client, "register", time.Minute, 3)
	if _, err := login.tryAllow("1.2.3.4"); err != nil {
		t.Fatalf("login tryAllow: %v", err)
	}
	if !mr.Exists("rl:login:1.2.3.4") || mr.Exists("rl:register:1.2.3.4") {
		t.Fatal("login scope should not affect register scope key")
	}
}

func TestFallbackLimiterUsesRedisWhenAvailable(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redisx.New(redisx.Config{Addr: mr.Addr()})
	l := newFallbackLimiter(client, "login", time.Minute, 3)
	if !l.Allow("1.2.3.4") {
		t.Fatal("first request should be allowed")
	}
	// 走的是 Redis：key 应已写入
	if !mr.Exists("rl:login:1.2.3.4") {
		t.Fatal("expected redis key rl:login:1.2.3.4 to exist")
	}
}

func TestFallbackLimiterUsesMemoryWhenUnavailable(t *testing.T) {
	// available=false 直接走内存；redis 为 nil 也不应被触碰。
	l := &fallbackLimiter{
		redis:     nil,
		memory:    newRateLimiter(time.Minute, 1),
		available: func() bool { return false },
	}
	if !l.Allow("1.2.3.4") {
		t.Fatal("first request should be allowed")
	}
	if l.Allow("1.2.3.4") {
		t.Fatal("second request should be rejected by memory limiter")
	}
}

func TestFallbackLimiterDegradesOnRedisError(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redisx.New(redisx.Config{Addr: mr.Addr()})
	l := newFallbackLimiter(client, "login", time.Minute, 1)
	// 启动时 available 已固定为 true；关闭 Redis 模拟中途宕机，tryAllow 返回错误
	mr.Close()
	if !l.Allow("1.2.3.4") {
		t.Fatal("first request should fall back to memory and be allowed")
	}
	if l.Allow("1.2.3.4") {
		t.Fatal("second request should be rejected by memory limiter")
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/api/ -run 'TestRedisLimiter|TestFallbackLimiter' -v`
Expected: FAIL（`undefined: newRedisLimiter` / `undefined: newFallbackLimiter` / `undefined: fallbackLimiter`）

- [ ] **Step 3: 改写 `internal/api/ratelimit.go`**

用以下内容整体替换 `internal/api/ratelimit.go`：

```go
package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

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
}

func (f *fallbackLimiter) Allow(ip string) bool {
	if f.available() {
		if ok, err := f.redis.tryAllow(ip); err == nil {
			return ok
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
```

- [ ] **Step 4: 更新现有 `internal/api/ratelimit_test.go`**

把 `TestRateLimiter` 里的 5 处 `rl.allow(` 改为 `rl.Allow(`，并新增中间件测试。整体替换 `internal/api/ratelimit_test.go`：

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimiter(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rl := newRateLimiterWithClock(time.Minute, 3, func() time.Time { return now })
	ip := "1.2.3.4"

	for i := 0; i < 3; i++ {
		if !rl.Allow(ip) {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if rl.Allow(ip) {
		t.Fatal("4th request within window should be rejected")
	}
	// 窗口过后重置
	now = now.Add(time.Minute)
	if !rl.Allow(ip) {
		t.Fatal("after window should be allowed again")
	}
	// 不同 IP 互不影响
	if !rl.Allow("5.6.7.8") {
		t.Fatal("different IP should be allowed")
	}
}

func TestRateLimitMiddlewareRejectsOverLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := newRateLimiter(time.Minute, 1)
	router := gin.New()
	router.POST("/x", rateLimitMiddleware(rl), func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("POST", "/x", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("first request: got %d, want 200", w.Code)
	}
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, httptest.NewRequest("POST", "/x", nil))
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: got %d, want 429", w2.Code)
	}
}
```

- [ ] **Step 5: 运行测试，确认通过**

Run: `go test ./internal/api/ -v`
Expected: PASS（含新的 6 个测试与原有 handler 测试）

- [ ] **Step 6: 提交**

```bash
git add internal/api/ratelimit.go internal/api/ratelimit_test.go internal/api/ratelimit_redis_test.go
git commit -m "feat: 限流抽接口，新增 Redis 限流与内存兜底"
```

---

### Task 3: 接线 `server.go`

**Files:**
- Modify: `internal/api/server.go`（第 124-127 行附近的限流器构造）

**Interfaces:**
- Consumes: `redisx.ConfigFromEnv()`、`redisx.New`、`(*Client).Available()`、`newFallbackLimiter`（来自 Task 1/2）。

- [ ] **Step 1: 改 `server.go` 接线**

在 `internal/api/server.go` 中，把：

```go
	loginLimiter := newRateLimiter(time.Minute, 10)
	registerLimiter := newRateLimiter(time.Minute, 10)
```

替换为：

```go
	rdsCfg := redisx.ConfigFromEnv()
	rds := redisx.New(rdsCfg)
	if !rds.Available() {
		logx.L().Warnf("redis unavailable at %s, rate limiter falls back to in-memory", rdsCfg.Addr)
	}
	loginLimiter := newFallbackLimiter(rds, "login", time.Minute, 10)
	registerLimiter := newFallbackLimiter(rds, "register", time.Minute, 10)
```

并在 `internal/api/server.go` 的 import 块（`"shop/internal/logx"` 等）中新增一行：

```go
	"shop/internal/redisx"
```

（`logx`、`time` 已在 import 中，无需新增。）

- [ ] **Step 2: 验证编译与全量测试**

Run: `go vet ./... && go test ./... && go build ./...`
Expected: 全部通过，无编译错误。

- [ ] **Step 3: 提交**

```bash
git add internal/api/server.go
git commit -m "feat: 登录/注册限流接入 Redis（不可用时回退内存）"
```

---

## 自检记录（写作时已完成）

- **Spec 覆盖**：spec 中 `internal/redisx` 包（Config/New/Available/IncrWithTTL）→ Task 1；`Limiter` 接口 + 三实现 + 中间件签名 → Task 2；`server.go` 接线 → Task 3；降级/超时/测试 → Task 1&2 测试。无遗漏。
- **占位符扫描**：无 TBD/TODO；每步含实际代码与命令。
- **类型一致性**：`IncrWithTTL(ctx, key, ttl) (int64, error)`、`Available() bool`、`newFallbackLimiter(client, keyPrefix, window, limit) Limiter`、`rateLimitMiddleware(l Limiter)` 在 Task 1/2/3 间一致。
- **Review Focus**：5 项均有对应测试（见各 Task 测试名）。
