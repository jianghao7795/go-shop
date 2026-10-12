# Redis 缓存基础层 + 限流接入设计

- 日期：2026-10-12
- 状态：待评审
- 规模：架构级（引入 Redis 依赖 + 缓存基础层，限流为第一个消费者）

## 目标

给 shop 后端引入 Redis 缓存基础设施，第一个消费者是登录/注册限流。目标：

1. 建立可复用的 Redis 访问层（`internal/redisx`），后续权限/商品/分类等缓存都基于它。
2. 把登录/注册限流从「进程内存 fixed-window」升级为「Redis 固定窗口」，解决内存限流**重启丢失、多实例不共享**的问题。
3. Redis 未配置或不可用时**优雅降级**：回退到现有内存限流器，不影响登录/注册可用性。

## 现状

- 后端为 Gin 单进程 + GORM/MySQL（`internal/database/database.go`），无任何 Redis 依赖。
- 登录/注册限流用内存 map 实现：`internal/api/ratelimit.go` 的 `rateLimiter`（`hits` + `windowAt` 两个 map，`sync.Mutex` 保护），固定窗口（`newRateLimiter(time.Minute, 10)`）。
- `internal/api/server.go:124-127` 为 login/register 各建一个 `newRateLimiter(time.Minute, 10)`，经 `rateLimitMiddleware` 生效。
- 已有「降级」先例：`databaseReady` 判断 DB 可用性，不可用时商品/分类/券回退到 `model.Fallback*` 内存数据。
- 现有测试 `internal/api/ratelimit_test.go` 直接调用 `rateLimiter.allow(ip)`（小写方法）验证固定窗口行为。

## 关键决策

1. **客户端库：`github.com/redis/go-redis/v9`**（de facto 标准），测试用 `github.com/alicebob/miniredis/v2`（内存 Redis，测试不依赖真实实例）。
2. **分层**：`internal/redisx` 只做「原始 Redis 访问」（连接、健康检查、`IncrWithTTL`），不懂业务；限流逻辑留在 `internal/api`。包名用 `redisx` 避免与 go-redis 的 import 名 `redis` 冲突。
3. **限流抽接口**：`type Limiter interface { Allow(ip string) bool }`，三种实现——内存 `rateLimiter`（保留现有逻辑）、`redisLimiter`（Redis 固定窗口）、`fallbackLimiter`（按可用性在两者间切换）。
4. **Redis 限流算法：固定窗口 Lua 原子操作**（`INCR` + 首次 `EXPIRE`），与现有内存 fixed-window 语义一致，不引入滑动窗口（YAGNI）。
5. **降级语义：回退内存限流**（用户已确认）。Redis 不可用时单机仍限流，只是多实例不共享；不 fail-open（裸奔）也不 fail-closed（拒所有登录）。
6. **v1 不自动重连**：启动时 Ping 一次，失败即全程降级直到进程重启（已知局限，记录在案）。

## 后端设计

### `internal/redisx/redisx.go`（新增）

- `type Config struct { Addr, Password string; DB int }`；`ConfigFromEnv()` 读 `REDIS_ADDR`（默认 `192.168.57.47:6379`）、`REDIS_PASSWORD`（默认空）、`REDIS_DB`（默认 0）。
- `type Client struct { rdb *redis.Client; available bool }`。
- `New(cfg Config) *Client`：`redis.NewClient` + Ping（`context.WithTimeout` 1s）。失败则 `available=false` 并记 warn 日志，**不 panic**，返回的 `*Client` 仍可用（所有方法返回错误）。
- `Available() bool`：返回当前健康状态。
- `IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error)`：`available=false` 时返回 `ErrUnavailable`；否则跑 Lua 脚本（`INCR`，计数为 1 时 `EXPIRE`），返回计数。
- 预留扩展位：后续权限/商品缓存在此加 `Get` / `Set` / `Del`（本次不实现）。

### `internal/api/ratelimit.go`（改）

- `type Limiter interface { Allow(ip string) bool }`。
- 内存 `rateLimiter`：逻辑不变，方法 `allow` 导出为 `Allow`（实现接口）。`newRateLimiter` / `newRateLimiterWithClock` 保留。
- `type redisLimiter struct { client *redisx.Client; window time.Duration; limit int; keyPrefix string }`：
  - `tryAllow(ip string) (bool, error)`：`IncrWithTTL(ctx, "rl:"+keyPrefix+":"+ip, window)`，ctx 带 200ms 超时；`n <= limit` 放行，Redis 错误返回 err。
  - `Allow(ip)` 包一层：错误时返回 false（由 fallback 兜底，不应被直接使用）。
- `type fallbackLimiter struct { redis *redisLimiter; memory Limiter; available func() bool }`：
  - `Allow(ip) bool`：`available()` 为真则 `redis.tryAllow`，无错就用其结果；否则用 `memory.Allow`。
- 构造器 `newFallbackLimiter(client *redisx.Client, scope string, window time.Duration, limit int) Limiter`：组装 redisLimiter（keyPrefix=scope）与内存 rateLimiter。
- `rateLimitMiddleware(l Limiter)`：签名由 `*rateLimiter` 改为接口，429 行为不变。

### `internal/api/server.go`（改）

- 启动处 `rds := redisx.New(redisx.ConfigFromEnv())`。
- `loginLimiter := newFallbackLimiter(rds, "login", time.Minute, 10)`；`registerLimiter := newFallbackLimiter(rds, "register", time.Minute, 10)`。
- 替换原 `newRateLimiter(time.Minute, 10)` 两处。

### 依赖

- `go.mod`：新增 `github.com/redis/go-redis/v9`（运行）、`github.com/alicebob/miniredis/v2`（测试）。

## 配置

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `REDIS_ADDR` | `192.168.57.47:6379` | Redis 地址（host:port） |
| `REDIS_PASSWORD` | 空 | 密码 |
| `REDIS_DB` | `0` | 库号 |

未配置/连不上时行为：全程内存限流，日志打 warn，服务正常运行。

## 错误处理与超时

- Redis 每次操作带 200ms 超时（`context.WithTimeout`），避免 Redis 卡死拖慢登录请求。
- Redis 调用报错时 `redisLimiter.tryAllow` 返回 err，`fallbackLimiter` 落到内存兜底并记日志，不影响登录可用性。
- `rateLimitMiddleware` 对超限仍返回 429（行为不变）。

## 测试

- `internal/redisx`：miniredis 测 `IncrWithTTL`（计数递增、首次设 TTL、原子性）；`New` 指向不可达地址时 `Available()==false`。
- `internal/api`：
  - `redisLimiter` 固定窗口：放行至 limit → 超限拒绝 → 窗口过后重置（用 miniredis，可用 `FastForward` 推进时间或直接删 key 模拟）。
  - `fallbackLimiter`：available 走 Redis；unavailable 或 Redis 报错走内存。
  - 现有 `TestRateLimiter` 的 `allow` 调用改为 `Allow`。
- 全部测试无需真实 Redis。

## 非目标（YAGNI）

- 不做权限/商品/分类/券等缓存（后续单独切片，复用本基础层）。
- 不做 Redis 滑动窗口/令牌桶限流。
- 不做自动重连 / 健康自愈（v1 启动判一次，恢复需重启）。
- 不做 Redis 连接池细调、哨兵/集群、TLS（用 go-redis 默认连接池即可）。
- 不在 `redisx` 实现通用 `Get/Set/Del`（本次只实现 `IncrWithTTL`，接口留位）。
