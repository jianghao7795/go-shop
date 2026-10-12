# 权限缓存（effectivePermissions）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给 `effectivePermissions` 加 Redis 缓存（读时回填 + 全局 epoch 写时失效），消除每个 admin 请求的 4 表 JOIN。

**Architecture:** 扩展 `internal/redisx` 加 `Get/Set/Incr`；`internal/api` 用 package 级 `permCache` 缓存每用户权限码（键 `perm:{username}`，值含 epoch），RBAC 写操作后 `INCR perm:epoch` 全量失效；Redis 不可用时回退直查 DB。

**Tech Stack:** Go 1.26、gin、gorm、go-redis/v9、miniredis/v2（测试）、glebarez/sqlite（测试 DB）。

**Spec:** `docs/superpowers/specs/2026-10-12-permission-cache-design.md`

## Global Constraints

- `permCache` 为 nil 或 `Available()==false` 时，`effectivePermissions` 必须直查 DB（现状行为，零回归）。
- 每个 Redis 操作带 200ms 超时（`permOpTimeout`）。
- 失效规则：**任何 RBAC 写操作成功后** `INCR perm:epoch`（6 个 handler）。
- `perm:epoch` 无 TTL；`perm:{username}` TTL 5 分钟（TTL 仅作清理/兜底，正确性靠 epoch）。
- `redisx.Get` 在 key 不存在时返回 `redis.Nil`、Redis 不可用时返回 `ErrUnavailable`；`Set` 对 ttl<1s 保底 1s。
- 不改 `requirePermission`/`meHandler`/`updateProfile`/`adminUpdateUserStatus` 的签名。

## Review Focus

1. **Redis 中途宕机**（`Available()` 仍 true 但 `Get` 报错）→ 必须走 DB、不 panic、不返回过期数据。→ Task 2 `TestEffectivePermissionsRedisDown`。
2. **空权限用户（无角色）的负缓存** → 缓存空 codes 时读回应 `ok=true` 且 codes 为空，`hasPermission` 正确返回 false。→ Task 2 `TestEmptyCodesCached`。
3. **损坏的缓存值（非 JSON）** → 应 miss 走 DB，不 panic。→ Task 2 `TestCachedPermsCorruptValue`。
4. **epoch 键缺失（首次）** → `Get perm:epoch` 返回 `redis.Nil` 应视为 0，回填/读一致。→ Task 2 `TestEffectivePermissionsUsesCache`（隐式）。
5. **RBAC 写失败路径不失效** → `invalidatePermCache()` 必须放在错误处理（`return` 分支）之后、成功响应之前。→ Task 3 实现时注意放置位置。

---

### Task 1: `redisx` 增加 Get/Set/Incr

**Files:**
- Modify: `internal/redisx/redisx.go`
- Test: `internal/redisx/redisx_test.go`

**Interfaces:**
- Consumes: 无新增（沿用现有 `Client`/`New`/`Config`）。
- Produces: `(*Client).Get(ctx, key string) (string, error)`（miss 返回 `redis.Nil`）、`(*Client).Set(ctx, key, value string, ttl time.Duration) error`、`(*Client).Incr(ctx, key string) (int64, error)`。Task 2 依赖这些。

- [ ] **Step 1: 写失败测试**

在 `internal/redisx/redisx_test.go` 追加：

```go
func TestGetSetIncr(t *testing.T) {
	mr := miniredis.RunT(t)
	client := New(Config{Addr: mr.Addr()})
	ctx := context.Background()

	// miss 返回 redis.Nil
	if _, err := client.Get(ctx, "nope"); !errors.Is(err, redis.Nil) {
		t.Fatalf("miss: got %v, want redis.Nil", err)
	}
	// set + get
	if err := client.Set(ctx, "k", "v", time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	if v, err := client.Get(ctx, "k"); err != nil || v != "v" {
		t.Fatalf("get: got %q, %v; want v, nil", v, err)
	}
	if ttl := mr.TTL("k"); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("ttl: got %v; want >0 and <=1m", ttl)
	}
	// incr
	if n, err := client.Incr(ctx, "c"); err != nil || n != 1 {
		t.Fatalf("incr 1: got %d, %v; want 1, nil", n, err)
	}
	if n, _ := client.Incr(ctx, "c"); n != 2 {
		t.Fatalf("incr 2: got %d; want 2", n)
	}
}

func TestGetSetIncrUnavailable(t *testing.T) {
	client := New(Config{Addr: "127.0.0.1:1"})
	ctx := context.Background()
	if _, err := client.Get(ctx, "k"); err != ErrUnavailable {
		t.Fatalf("get: got %v, want ErrUnavailable", err)
	}
	if err := client.Set(ctx, "k", "v", time.Minute); err != ErrUnavailable {
		t.Fatalf("set: got %v, want ErrUnavailable", err)
	}
	if _, err := client.Incr(ctx, "c"); err != ErrUnavailable {
		t.Fatalf("incr: got %v, want ErrUnavailable", err)
	}
}
```

并在文件 import 块加 `"errors"` 与 `"github.com/redis/go-redis/v9"`（现 import 有 `context`/`testing`/`time`/miniredis）。

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/redisx/ -run 'TestGetSetIncr' -v`
Expected: FAIL（`undefined: redis.Nil` 或 `client.Get/Set/Incr undefined`）

- [ ] **Step 3: 实现**

在 `internal/redisx/redisx.go` 的 `IncrWithTTL` 之后追加：

```go
// Get 读取 key 的值；key 不存在返回 redis.Nil，Redis 不可用返回 ErrUnavailable。
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	if !c.available {
		return "", ErrUnavailable
	}
	return c.rdb.Get(ctx, key).Result()
}

// Set 写入 key 的值并设置过期时间。
func (c *Client) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if !c.available {
		return ErrUnavailable
	}
	if ttl < time.Second {
		ttl = time.Second
	}
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// Incr 对 key 自增 1（无过期时间）。
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	if !c.available {
		return 0, ErrUnavailable
	}
	return c.rdb.Incr(ctx, key).Result()
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./internal/redisx/ -v`
Expected: PASS（新旧测试全过）

- [ ] **Step 5: 提交**

```bash
git add internal/redisx/redisx.go internal/redisx/redisx_test.go
git commit -m "feat: redisx 增加 Get/Set/Incr"
```

---

### Task 2: 权限缓存逻辑

**Files:**
- Modify: `internal/api/permission.go`
- Create: `internal/api/permission_cache.go`
- Test: `internal/api/permission_cache_test.go`

**Interfaces:**
- Consumes: `redisx.Client`、`redisx.New`、`redisx.Config`、`(*Client).Get/Set/Incr/Available`、`redis.Nil`（来自 Task 1）。
- Produces: `effectivePermissions(db, username)`（签名不变，语义改为「优先缓存」）、`invalidatePermCache()`。Task 3 依赖 `invalidatePermCache()`。

- [ ] **Step 1: 写失败测试**

创建 `internal/api/permission_cache_test.go`：

```go
package api

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"shop/internal/model"
	"shop/internal/redisx"
)

func permTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	db.AutoMigrate(&model.User{}, &model.Role{}, &model.Permission{}, &model.UserRole{}, &model.RolePermission{})
	return db
}

func TestEffectivePermissionsUsesCache(t *testing.T) {
	mr := miniredis.RunT(t)
	permCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { permCache = nil })

	db := permTestDB(t) // 空库，无任何数据

	cacheEffectivePermissions("alice", []string{"product:manage"})
	got := effectivePermissions(db, "alice")
	if !hasPermission(got, "product:manage") {
		t.Fatalf("expected cached permission product:manage, got %v", got)
	}
}

func TestCachedPermsStaleAfterInvalidate(t *testing.T) {
	mr := miniredis.RunT(t)
	permCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { permCache = nil })

	cacheEffectivePermissions("alice", []string{"product:manage"})
	if _, ok := cachedEffectivePermissions("alice"); !ok {
		t.Fatal("expected cache hit before invalidation")
	}
	invalidatePermCache()
	if _, ok := cachedEffectivePermissions("alice"); ok {
		t.Fatal("expected cache miss after invalidation")
	}
}

func TestEffectivePermissionsRedisDown(t *testing.T) {
	mr := miniredis.RunT(t)
	permCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { permCache = nil })

	db := permTestDB(t)
	user := model.User{Username: "alice", Password: "x"}
	db.Create(&user)
	role := model.Role{Name: "staff"}
	db.Create(&role)
	p := model.Permission{Code: "order:manage"}
	db.Create(&p)
	db.Create(&model.UserRole{UserID: user.ID, RoleID: role.ID})
	db.Create(&model.RolePermission{RoleID: role.ID, PermissionID: p.ID})

	mr.Close() // 模拟 Redis 中途宕机（Available 仍为 true）

	got := effectivePermissions(db, "alice")
	if !hasPermission(got, "order:manage") {
		t.Fatalf("expected DB fallback to return order:manage, got %v", got)
	}
}

func TestCachedPermsCorruptValue(t *testing.T) {
	mr := miniredis.RunT(t)
	permCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { permCache = nil })

	_ = permCache.Set(context.Background(), "perm:alice", "not-json", time.Minute)
	if _, ok := cachedEffectivePermissions("alice"); ok {
		t.Fatal("expected miss for corrupt cache value")
	}
}

func TestEmptyCodesCached(t *testing.T) {
	mr := miniredis.RunT(t)
	permCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { permCache = nil })

	cacheEffectivePermissions("nobody", nil)
	codes, ok := cachedEffectivePermissions("nobody")
	if !ok {
		t.Fatal("expected cache hit for empty codes")
	}
	if len(codes) != 0 {
		t.Fatalf("expected empty codes, got %v", codes)
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/api/ -run 'TestEffectivePermissionsUsesCache|TestCachedPerms|TestEmptyCodes' -v`
Expected: FAIL（`undefined: permCache` / `cacheEffectivePermissions` / `cachedEffectivePermissions` / `invalidatePermCache`）

- [ ] **Step 3: 改写 `internal/api/permission.go`**

把 `effectivePermissions` 拆成「编排 + 查询」，用以下内容整体替换 `internal/api/permission.go`：

```go
package api

import (
	"slices"

	"gorm.io/gorm"

	"shop/internal/model"
)

// hasPermission 判断权限列表是否包含某权限点。
func hasPermission(perms []string, perm string) bool {
	return slices.Contains(perms, perm)
}

// effectivePermissions 返回用户在数据库中的有效权限点并集（优先读缓存，miss 则查库回填）。
func effectivePermissions(db *gorm.DB, username string) []string {
	if codes, ok := cachedEffectivePermissions(username); ok {
		return codes
	}
	codes := queryEffectivePermissions(db, username)
	cacheEffectivePermissions(username, codes)
	return codes
}

// queryEffectivePermissions 用一次 JOIN 查用户的有效权限码（避免 N+1）。
func queryEffectivePermissions(db *gorm.DB, username string) []string {
	var codes []string
	db.Model(&model.Permission{}).
		Select("DISTINCT shop_permissions.code").
		Joins("JOIN shop_role_permissions rp ON rp.permission_id = shop_permissions.id").
		Joins("JOIN shop_user_roles ur ON ur.role_id = rp.role_id").
		Joins("JOIN shop_users u ON u.id = ur.user_id").
		Where("u.username = ?", username).
		Scan(&codes)
	return codes
}
```

- [ ] **Step 4: 创建 `internal/api/permission_cache.go`**

```go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"shop/internal/redisx"
)

const (
	permEpochKey  = "perm:epoch"
	permKeyPrefix = "perm:"
	permTTL       = 5 * time.Minute
	permOpTimeout = 200 * time.Millisecond
)

// permCache 是权限缓存的 Redis 客户端；nil 或不可用时走 DB。
var permCache *redisx.Client

// permCacheEntry 是 perm:{username} 的缓存值：写入时的 epoch + 权限码。
type permCacheEntry struct {
	Epoch int64    `json:"e"`
	Codes []string `json:"c"`
}

// cachedEffectivePermissions 读取并校验缓存的权限码；未命中/不可用返回 ok=false。
func cachedEffectivePermissions(username string) ([]string, bool) {
	if permCache == nil || !permCache.Available() {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), permOpTimeout)
	defer cancel()

	epoch := int64(0)
	if s, err := permCache.Get(ctx, permEpochKey); err == nil {
		epoch, _ = strconv.ParseInt(s, 10, 64)
	} else if !errors.Is(err, redis.Nil) {
		return nil, false
	}

	val, err := permCache.Get(ctx, permKeyPrefix+username)
	if err != nil {
		return nil, false
	}
	var entry permCacheEntry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		return nil, false
	}
	if entry.Epoch != epoch {
		return nil, false
	}
	return entry.Codes, true
}

// cacheEffectivePermissions 回填缓存（best-effort，忽略错误）。
func cacheEffectivePermissions(username string, codes []string) {
	if permCache == nil || !permCache.Available() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), permOpTimeout)
	defer cancel()

	epoch := int64(0)
	if s, err := permCache.Get(ctx, permEpochKey); err == nil {
		epoch, _ = strconv.ParseInt(s, 10, 64)
	}
	b, err := json.Marshal(permCacheEntry{Epoch: epoch, Codes: codes})
	if err != nil {
		return
	}
	_ = permCache.Set(ctx, permKeyPrefix+username, string(b), permTTL)
}

// invalidatePermCache 使所有缓存权限失效（bump 全局 epoch）。
func invalidatePermCache() {
	if permCache == nil || !permCache.Available() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), permOpTimeout)
	defer cancel()
	_, _ = permCache.Incr(ctx, permEpochKey)
}
```

- [ ] **Step 5: 运行测试，确认通过**

Run: `go test ./internal/api/ -v`
Expected: PASS（新增 5 个测试 + 现有 `TestEffectivePermissions` 等全过）

- [ ] **Step 6: 提交**

```bash
git add internal/api/permission.go internal/api/permission_cache.go internal/api/permission_cache_test.go
git commit -m "feat: effectivePermissions 加 Redis 缓存（epoch 写时失效）"
```

---

### Task 3: 失效点接线

**Files:**
- Modify: `internal/api/handlers_admin.go`（6 处加 `invalidatePermCache()`）
- Modify: `internal/api/server.go`（加 `permCache = rds`）

**Interfaces:**
- Consumes: `invalidatePermCache()`（来自 Task 2）、`redisx`（server.go 已有 `rds`）。

- [ ] **Step 1: `server.go` 接线**

在 `internal/api/server.go` 中，把：

```go
	if !rds.Available() {
		logx.L().Warnf("redis unavailable at %s, rate limiter falls back to in-memory", rdsCfg.Addr)
	}
	loginLimiter := newFallbackLimiter(rds, "login", time.Minute, 10)
```

改为：

```go
	if !rds.Available() {
		logx.L().Warnf("redis unavailable at %s, rate limiter falls back to in-memory", rdsCfg.Addr)
	}
	permCache = rds
	loginLimiter := newFallbackLimiter(rds, "login", time.Minute, 10)
```

- [ ] **Step 2: 6 个 RBAC 写操作加失效**

在 `internal/api/handlers_admin.go` 中，每个 handler 的**成功响应之前**加一行 `invalidatePermCache()`（放在 `return` 错误分支之后）：

1. `adminCreateRole`（`c.JSON(http.StatusOK, toRoleView(db, item))` 之前）
2. `adminUpdateRole`（`c.JSON(http.StatusOK, gin.H{"message": "更新成功"})` 之前）
3. `adminDeleteRole`（`c.JSON(http.StatusOK, gin.H{"message": "删除成功"})` 之前）
4. `adminCreatePermission`（`c.JSON(http.StatusOK, item)` 之前）
5. `adminDeletePermission`（`c.JSON(http.StatusOK, gin.H{"message": "删除成功"})` 之前）
6. `adminSetUserRoles`（`c.JSON(http.StatusOK, gin.H{"message": "已更新"})` 之前）

示例（`adminCreateRole` 尾部）：

```go
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "创建失败"})
		return
	}
	invalidatePermCache()
	c.JSON(http.StatusOK, toRoleView(db, item))
```

- [ ] **Step 3: 验证**

Run: `go vet ./... && go test ./... && go build ./...`
Expected: 全部通过。

- [ ] **Step 4: 提交**

```bash
git add internal/api/handlers_admin.go internal/api/server.go
git commit -m "feat: RBAC 写操作后失效权限缓存"
```

---

## 自检记录（写作时已完成）

- **Spec 覆盖**：redisx `Get/Set/Incr` → Task 1；缓存键/epoch/读回填/失效 → Task 2；`permCache=rds` 接线 + 6 个失效点 → Task 3；降级/超时/测试 → Task 2 测试。无遗漏。
- **占位符扫描**：无 TBD/TODO；每步含实际代码与命令。
- **类型一致性**：`Get(ctx,key)(string,error)`、`Set(ctx,key,value,ttl) error`、`Incr(ctx,key)(int64,error)`、`invalidatePermCache()`、`effectivePermissions(db,username)` 在 Task 1/2/3 间一致。
- **Review Focus**：5 项均有对应测试或实现注意点（见各 Task）。
