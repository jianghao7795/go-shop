# 权限缓存（effectivePermissions）设计

- 日期：2026-10-12
- 状态：待评审
- 规模：架构级（扩展 redisx + 权限读路径缓存 + RBAC 写时失效）

## 目标

给权限点查询 `effectivePermissions` 加 Redis 缓存，消除「每个 admin 请求跑一次 4 表 JOIN」的重复查询；RBAC 变更后**立即失效**（写时失效 + TTL 兜底），权限不可用/不可达时回退直查 DB。

## 现状

- `internal/api/permission.go:17` 的 `effectivePermissions(db, username)` 用一条 4 表 JOIN（permissions × role_permissions × user_roles × users）查用户有效权限码。
- 调用方：`requirePermission`（`middleware.go:38`，挂在全部 admin 路由上）、`meHandler`/`updateProfile`（`handlers_user.go`）、`adminUpdateUserStatus`（`handlers_admin.go:75`）。
- 已建成 `internal/redisx` 基础层（`New`/`Available`/`IncrWithTTL`），本需求补 `Get`/`Set`/`Incr`。
- RBAC 写操作：`adminCreateRole`/`adminUpdateRole`/`adminDeleteRole`/`adminCreatePermission`/`adminDeletePermission`/`adminSetUserRoles`（`handlers_admin.go`）。
- 现有测试 `permission_test.go::TestEffectivePermissions` 用内存 sqlite 直接调 `effectivePermissions(db, "alice")` 走 DB。

## 关键决策

1. **失效机制：全局版本号 epoch**。任何 RBAC 写操作 `INCR perm:epoch`，缓存值记录写入时的 epoch，读时比对，不一致即回源。相比「按受影响用户逐个 DEL」，无需枚举受影响用户（角色变更会影响该角色下所有用户），一行 `INCR` 即全量失效、不可能漏。
2. **缓存粒度：按用户**，键 `perm:{username}`，值 JSON `{"e":epoch,"c":[codes]}`，TTL 5 分钟（TTL 仅作兜底/清理旧键，正确性靠 epoch）。
3. **接入方式：package 级 `permCache *redisx.Client`**，`Start()` 赋值，`effectivePermissions`/`invalidatePermCache` 内部读写。理由：`requirePermission` 有 ~30 处调用，显式传参会牵动大量签名；且代码库已有 `logx`/`statusText` 的 package 级先例，这是单例 Redis 客户端。
4. **降级：permCache 为 nil 或不可用 → 直接查 DB**（现状行为，零回归）。
5. **统一规则：任何 RBAC 写都 bump epoch**（含 create，即便它不改变任何人权限），牺牲一次多余回源换取「不会漏失效」的简单性。

## 后端设计

### `internal/redisx/redisx.go`（扩展）

新增三方法，均在 `available=false` 时返回 `ErrUnavailable`：

```go
// Get 读取 key 的值；key 不存在返回 redis.Nil。
func (c *Client) Get(ctx context.Context, key string) (string, error)

// Set 写入 key 的值并设置过期时间。
func (c *Client) Set(ctx context.Context, key, value string, ttl time.Duration) error

// Incr 对 key 自增 1（无过期时间）。
func (c *Client) Incr(ctx context.Context, key string) (int64, error)
```

### `internal/api/permission.go`（改造 + 新增缓存逻辑）

- `var permCache *redisx.Client`（package 级）。
- 常量：`permEpochKey = "perm:epoch"`、`permKeyPrefix = "perm:"`、`permTTL = 5 * time.Minute`、`permOpTimeout = 200 * time.Millisecond`。
- `type permCacheEntry struct { Epoch int64 `json:"e"`; Codes []string `json:"c"` }`。
- `effectivePermissions(db, username)` 改为：先 `cachedEffectivePermissions(username)`（命中即返回），否则 `queryEffectivePermissions(db, username)`（原 JOIN 逻辑抽成此函数）并 `cacheEffectivePermissions(username, codes)`。
- `cachedEffectivePermissions(username) ([]string, bool)`：permCache 为 nil/不可用 → `(nil,false)`；读 epoch（`Get perm:epoch`，miss 视为 0，错误视为不可用）→ 读 `perm:{username}` → 解析 JSON、比对 epoch，一致则返回，否则 `(nil,false)`。
- `cacheEffectivePermissions(username, codes)`：best-effort 读当前 epoch 并 `Set perm:{username}`（TTL 5min），忽略所有错误。
- `invalidatePermCache()`：`Incr perm:epoch`（best-effort，忽略错误）。
- `requirePermission`、`meHandler`、`updateProfile`、`adminUpdateUserStatus` 签名不变。

### `internal/api/handlers_admin.go`（失效点）

6 个 RBAC 写操作在事务/更新成功后调用 `invalidatePermCache()`：
`adminCreateRole`、`adminUpdateRole`、`adminDeleteRole`、`adminCreatePermission`、`adminDeletePermission`、`adminSetUserRoles`。

### `internal/api/server.go`（接线）

在现有 `rds := redisx.New(rdsCfg)` 之后加 `permCache = rds`（复用同一客户端/连接池）。

## 键结构

| 键 | 类型 | 值 | TTL |
|---|---|---|---|
| `perm:epoch` | string(整数) | 全局版本号 | 无 |
| `perm:{username}` | string(JSON) | `{"e":<epoch>,"c":["code",...]}` | 5 分钟 |

## 一致性 / 降级 / 超时

- RBAC 写成功后 `INCR perm:epoch`，所有已缓存权限码 epoch 失配 → 下次访问回源，立即生效。
- TTL 兜底清理长期不活跃用户的旧键；epoch 只增不减，旧键过期后不再占空间。
- permCache 为 nil 或 `Available()==false` → `cachedEffectivePermissions` 立即返回 miss，走 DB。
- 每个 Redis 操作带 200ms 超时；中途宕机时每请求最多等 200ms（fail-fast 熔断已作为 minor 延后，与限流一致）。

## 测试

- `internal/redisx`：miniredis 测 `Get`（命中/`redis.Nil`/不可用 `ErrUnavailable`）、`Set`（写后 `Get` 一致、TTL 生效）、`Incr`。
- `internal/api`：
  - 现有 `TestEffectivePermissions`（permCache=nil 走 DB）保持通过。
  - 新增：首次调用后 `perm:alice` 存在（回填）；`invalidatePermCache()` 后 `perm:epoch` 递增；epoch 失配导致回源（改 epoch 后重新查库）。
  - 缓存命中不触发 DB：通过「先回填、再调用、断言第二次读的是缓存」或 miniredis 键存在性断言。

## 非目标（YAGNI）

- 不做商品/分类/券缓存（后续单独切片，复用同一 `redisx` 与 cache-aside 模式）。
- 不做按受影响用户精准失效（用全局 epoch 代替）。
- 不做缓存预热、批量加载、布隆过滤。
- 不做 redisx 的 `Del`（epoch 方案不需要删除）。
- 不做 fail-fast 熔断（已延后）。
