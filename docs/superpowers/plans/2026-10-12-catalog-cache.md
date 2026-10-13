# 商品/分类缓存 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给分类、商品详情、商品列表加 Redis 缓存（读时回填 + 写时失效），消除高频读路径的重复 MySQL 查询。

**Architecture:** 扩展 `redisx` 加 `Del`；新增 `internal/api/catalog_cache.go` 用 package 级 `catalogCache` 缓存分类（DEL 失效）、商品详情（DEL 失效）、商品列表（全局 epoch 失效）；写操作后调用 `invalidateProductCache`/`invalidateCategoryCache`。

**Tech Stack:** Go 1.26、gin、gorm、go-redis/v9、miniredis/v2（测试）、glebarez/sqlite（测试 DB）。

**Spec:** `docs/superpowers/specs/2026-10-12-catalog-cache-design.md`

## Global Constraints

- `catalogCache` 为 nil 或 `Available()==false` 时，所有读路径直查 DB（零回归）。
- 每个 Redis 操作带 200ms 超时（`catalogOpTimeout`）。
- 失效：分类按 key `DEL`；商品详情按 key `DEL`；商品列表 `INCR product:epoch`。
- 缓存只回填**真实 DB 结果**，不回填 `FallbackProducts`/`model.Categories` 兜底数据。
- `productsHandler` 的 `search` 非空时直查 DB，不缓存。
- 现有 handler 测试（`catalogCache` 为 nil）必须继续走 DB 路径并通过。
- handler 缩进为 **2 tab**（`return func(c *gin.Context)` 嵌套），新增 `invalidate*` 调用与相邻 `c.JSON` 同缩进。

## Review Focus

1. **缓存命中返回了已下架/删除的商品** → 详情/列表读缓存后仍需保证不返回不该出现的商品（详情命中路径保留 `OnShelf` 检查；列表缓存 key 不区分 on_shelf，靠写时失效 + 5min TTL 兜底）。→ Task 3/4 测试。
2. **列表 epoch 竞态**（回填用「写后 epoch + 写前 items」脏组合）→ 与权限缓存同款修复：查库前捕获 epoch。→ Task 4 测试 `TestCachedProductsStaleEpoch`。
3. **`adminDeleteSku` 无 ProductID** → 需先查 SKU 再删，才能失效 `product:{pid}`。→ Task 3 修改。
4. **空结果回退不写缓存**（`FallbackProducts`/`Categories` 兜底不污染缓存）。→ Task 2/4 实现注意。
5. **分类与商品失效互不干扰**（分类改名不 bump 商品 epoch）。→ 各失效点只调对应 helper。

---

### Task 1: `redisx.Del`

**Files:**
- Modify: `internal/redisx/redisx.go`
- Test: `internal/redisx/redisx_test.go`

**Interfaces:**
- Produces: `(*Client).Del(ctx, key string) error`。Task 2/3 依赖。

- [ ] **Step 1: 写失败测试**

在 `internal/redisx/redisx_test.go` 追加：

```go
func TestDel(t *testing.T) {
	mr := miniredis.RunT(t)
	client := New(Config{Addr: mr.Addr()})
	ctx := context.Background()
	if err := client.Set(ctx, "k", "v", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := client.Del(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Get(ctx, "k"); !errors.Is(err, redis.Nil) {
		t.Fatalf("after del: got %v, want redis.Nil", err)
	}
	if err := client.Del(ctx, "nope"); err != nil {
		t.Fatalf("del missing: %v", err)
	}
}

func TestDelUnavailable(t *testing.T) {
	client := New(Config{Addr: "127.0.0.1:1"})
	if err := client.Del(context.Background(), "k"); err != ErrUnavailable {
		t.Fatalf("del: got %v, want ErrUnavailable", err)
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/redisx/ -run 'TestDel' -v`
Expected: FAIL（`client.Del undefined`）

- [ ] **Step 3: 实现**

在 `internal/redisx/redisx.go` 的 `Incr` 之后追加：

```go
// Del 删除 key（key 不存在也不报错）。
func (c *Client) Del(ctx context.Context, key string) error {
	if !c.available {
		return ErrUnavailable
	}
	return c.rdb.Del(ctx, key).Err()
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `go test ./internal/redisx/ -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add internal/redisx/redisx.go internal/redisx/redisx_test.go
git commit -m "feat: redisx 增加 Del"
```

---

### Task 2: 分类缓存

**Files:**
- Create: `internal/api/catalog_cache.go`（`catalogCache` 变量 + 分类原语）
- Modify: `internal/api/handlers_category.go`（`listCategories` 读缓存）
- Modify: `internal/api/handlers_admin.go`（3 个分类写操作加失效）
- Modify: `internal/api/server.go`（`catalogCache = rds`）
- Test: `internal/api/catalog_cache_test.go`（分类测试）

**Interfaces:**
- Consumes: `redisx.Client`/`Get`/`Set`/`Del`/`Available`（Task 1）。
- Produces: `catalogCache`、`cachedCategories`、`cacheCategories`、`invalidateCategoryCache`。Task 3/4 继续复用 `catalogCache`。

- [ ] **Step 1: 写失败测试**

创建 `internal/api/catalog_cache_test.go`：

```go
package api

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"shop/internal/model"
	"shop/internal/redisx"
)

func TestCachedCategoriesRoundTrip(t *testing.T) {
	mr := miniredis.RunT(t)
	catalogCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { catalogCache = nil })

	cats := []model.Category{{Key: "food", Name: "食品"}, {Key: "digital", Name: "数码"}}
	cacheCategories(cats)
	got, ok := cachedCategories()
	if !ok || len(got) != 2 || got[0].Key != "food" {
		t.Fatalf("round trip: got %v, ok=%v", got, ok)
	}
}

func TestInvalidateCategoryCache(t *testing.T) {
	mr := miniredis.RunT(t)
	catalogCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { catalogCache = nil })

	cacheCategories([]model.Category{{Key: "food", Name: "食品"}})
	invalidateCategoryCache()
	if _, ok := cachedCategories(); ok {
		t.Fatal("expected miss after invalidate")
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/api/ -run 'TestCachedCategories|TestInvalidateCategoryCache' -v`
Expected: FAIL（`undefined: catalogCache`/`cacheCategories`/`cachedCategories`/`invalidateCategoryCache`）

- [ ] **Step 3: 创建 `internal/api/catalog_cache.go`（分类部分）**

```go
package api

import (
	"context"
	"encoding/json"
	"time"

	"shop/internal/model"
	"shop/internal/redisx"
)

const (
	categoryKey      = "categories"
	categoryTTL      = 10 * time.Minute
	catalogOpTimeout = 200 * time.Millisecond
)

// catalogCache 是目录缓存的 Redis 客户端；nil 或不可用时走 DB。
var catalogCache *redisx.Client

// cachedCategories 读缓存分类；未命中/不可用返回 ok=false。
func cachedCategories() ([]model.Category, bool) {
	if catalogCache == nil || !catalogCache.Available() {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), catalogOpTimeout)
	defer cancel()
	val, err := catalogCache.Get(ctx, categoryKey)
	if err != nil {
		return nil, false
	}
	var cats []model.Category
	if err := json.Unmarshal([]byte(val), &cats); err != nil {
		return nil, false
	}
	return cats, true
}

// cacheCategories 回填分类缓存（best-effort）。
func cacheCategories(cats []model.Category) {
	if catalogCache == nil || !catalogCache.Available() {
		return
	}
	b, err := json.Marshal(cats)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), catalogOpTimeout)
	defer cancel()
	_ = catalogCache.Set(ctx, categoryKey, string(b), categoryTTL)
}

// invalidateCategoryCache 使分类缓存失效（DEL）。
func invalidateCategoryCache() {
	if catalogCache == nil || !catalogCache.Available() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), catalogOpTimeout)
	defer cancel()
	_, _ = catalogCache.Del(ctx, categoryKey)
}
```

- [ ] **Step 4: 改 `internal/api/handlers_category.go`**

把 `listCategories` 主体替换为：

```go
// listCategories 返回商品分类列表：优先读缓存，miss 则查库并回填；库不可用/为空回退种子数据。
func listCategories(db *gorm.DB, databaseReady bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !databaseReady || db == nil {
			c.JSON(http.StatusOK, model.Categories)
			return
		}
		if cats, ok := cachedCategories(); ok {
			c.JSON(http.StatusOK, cats)
			return
		}
		var cats []model.Category
		if err := db.Order("id ASC").Find(&cats).Error; err != nil || len(cats) == 0 {
			c.JSON(http.StatusOK, model.Categories)
			return
		}
		cacheCategories(cats)
		c.JSON(http.StatusOK, cats)
	}
}
```

- [ ] **Step 5: 3 个分类写操作加失效（`internal/api/handlers_admin.go`）**

在以下三处的成功响应前加一行 `invalidateCategoryCache()`（缩进 2 tab）：

1. `adminCreateCategory`：`c.JSON(http.StatusOK, item)` 之前。
2. `adminUpdateCategory`：`c.JSON(http.StatusOK, gin.H{"message": "更新成功"})` 之前（用 `if isDuplicateKey(err)` 上下文定位到分类的，不要改到商品的）。
3. `adminDeleteCategory`：`c.JSON(http.StatusOK, gin.H{"message": "删除成功"})` 之前（用 `gin.H{"message": "分类不存在"}` 上下文定位）。

示例（`adminDeleteCategory` 尾部）：

```go
		if res.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"message": "分类不存在"})
			return
		}
		invalidateCategoryCache()
		c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
```

- [ ] **Step 6: `server.go` 接线**

在 `internal/api/server.go` 的 `permCache = rds` 之后加一行：

```go
	catalogCache = rds
```

- [ ] **Step 7: 运行测试，确认通过**

Run: `go test ./internal/api/ -v`
Expected: PASS（新增分类测试 + 现有 `TestListCategories` 等全过）

- [ ] **Step 8: 提交**

```bash
git add internal/api/catalog_cache.go internal/api/handlers_category.go internal/api/handlers_admin.go internal/api/server.go internal/api/catalog_cache_test.go
git commit -m "feat: 分类列表加 Redis 缓存（写时 DEL 失效）"
```

---

### Task 3: 商品详情缓存

**Files:**
- Modify: `internal/api/catalog_cache.go`（加商品详情原语 + `invalidateProductCache`）
- Modify: `internal/api/handlers_product.go`（`productDetailHandler` 读缓存）
- Modify: `internal/api/handlers_admin.go`（6 个商品写操作加失效）
- Modify: `internal/api/handlers_review.go`（`submitOrderReviews` 加失效）
- Test: `internal/api/catalog_cache_test.go`（详情测试）

**Interfaces:**
- Consumes: `catalogCache`、`redisx.Del`/`Incr`（Task 1/2）。
- Produces: `cachedProduct`、`cacheProduct`、`invalidateProductCache`。Task 4 依赖 `invalidateProductCache`（含 epoch INCR）。

- [ ] **Step 1: 写失败测试**

在 `internal/api/catalog_cache_test.go` 追加：

```go
func TestCachedProductRoundTrip(t *testing.T) {
	mr := miniredis.RunT(t)
	catalogCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { catalogCache = nil })

	p := model.Product{ID: 7, Name: "耳机", OnShelf: true}
	cacheProduct(p)
	got, ok := cachedProduct(7)
	if !ok || got.Name != "耳机" {
		t.Fatalf("round trip: got %+v, ok=%v", got, ok)
	}
}

func TestInvalidateProductCache(t *testing.T) {
	mr := miniredis.RunT(t)
	catalogCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { catalogCache = nil })

	cacheProduct(model.Product{ID: 7, Name: "耳机"})
	invalidateProductCache(7)
	if _, ok := cachedProduct(7); ok {
		t.Fatal("expected miss after invalidate")
	}
}
```

（文件 import 需加 `"shop/internal/model"`，已含。）

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/api/ -run 'TestCachedProduct|TestInvalidateProductCache' -v`
Expected: FAIL（`undefined: cacheProduct`/`cachedProduct`/`invalidateProductCache`）

- [ ] **Step 3: 扩展 `internal/api/catalog_cache.go`（详情部分）**

追加常量与函数：

```go
const (
	productEpochKey  = "product:epoch"
	productKeyPrefix = "product:"
	productTTL       = 5 * time.Minute
)

// cachedProduct 读缓存商品；未命中/不可用返回 ok=false。
func cachedProduct(id int) (model.Product, bool) {
	if catalogCache == nil || !catalogCache.Available() {
		return model.Product{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), catalogOpTimeout)
	defer cancel()
	val, err := catalogCache.Get(ctx, productKeyPrefix+strconv.Itoa(id))
	if err != nil {
		return model.Product{}, false
	}
	var p model.Product
	if err := json.Unmarshal([]byte(val), &p); err != nil {
		return model.Product{}, false
	}
	return p, true
}

// cacheProduct 回填商品详情缓存（best-effort）。
func cacheProduct(p model.Product) {
	if catalogCache == nil || !catalogCache.Available() {
		return
	}
	b, err := json.Marshal(p)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), catalogOpTimeout)
	defer cancel()
	_ = catalogCache.Set(ctx, productKeyPrefix+strconv.Itoa(p.ID), string(b), productTTL)
}

// invalidateProductCache 商品变更：DEL 详情 + INCR 列表 epoch（epoch 由 Task 4 的列表缓存消费）。
func invalidateProductCache(id int) {
	if catalogCache == nil || !catalogCache.Available() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), catalogOpTimeout)
	defer cancel()
	_, _ = catalogCache.Del(ctx, productKeyPrefix+strconv.Itoa(id))
	_, _ = catalogCache.Incr(ctx, productEpochKey)
}
```

并在 import 块加 `"strconv"`。

- [ ] **Step 4: 改 `productDetailHandler`（`internal/api/handlers_product.go`）**

在 `id` 解析成功、`databaseReady` 检查之后，插入缓存读；miss 查库后回填。把 `productDetailHandler` 主体替换为：

```go
func productDetailHandler(db *gorm.DB, databaseReady bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !databaseReady {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "database unavailable"})
			return
		}
		id, parseErr := strconv.Atoi(c.Param("id"))
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid product id"})
			return
		}
		if item, ok := cachedProduct(id); ok {
			if !item.OnShelf {
				c.JSON(http.StatusNotFound, gin.H{"message": "product not found"})
				return
			}
			c.JSON(http.StatusOK, item)
			return
		}
		var item model.Product
		if queryErr := db.First(&item, id).Error; queryErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "product not found"})
			return
		}
		if !item.OnShelf {
			c.JSON(http.StatusNotFound, gin.H{"message": "product not found"})
			return
		}
		db.Where("product_id = ?", id).Find(&item.Skus)
		cacheProduct(item)
		c.JSON(http.StatusOK, item)
	}
}
```

- [ ] **Step 5: 商品写操作加失效（`internal/api/handlers_admin.go`）**

在以下六处成功响应前加 `invalidateProductCache(<id>)`（缩进 2 tab）：

1. `adminCreateProduct`：`c.JSON(http.StatusOK, item)` 之前 → `invalidateProductCache(item.ID)`。
2. `adminUpdateProduct`：`c.JSON(http.StatusOK, gin.H{"message": "更新成功"})` 之前 → `invalidateProductCache(id)`（用 `db.Model(&model.Product{})` 上下文定位）。
3. `adminDeleteProduct`：`c.JSON(http.StatusOK, gin.H{"message": "删除成功"})` 之前 → `invalidateProductCache(id)`（用 `gin.H{"message": "商品不存在"}` 定位）。
4. `adminToggleFeatured`：`c.JSON(http.StatusOK, gin.H{"message": "已更新"})` 之前 → `invalidateProductCache(id)`。
5. `adminCreateSku`：`c.JSON(http.StatusOK, sku)` 之前 → `invalidateProductCache(pid)`。
6. `adminDeleteSku`：**先查 SKU 取 ProductID**，再删，再失效。把整个 handler 主体替换为：

```go
func adminDeleteSku(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "非法规格 ID"})
			return
		}
		if !dbRequired(c, db) {
			return
		}
		var sku model.ProductSku
		if err := db.First(&sku, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "规格不存在"})
			return
		}
		if err := db.Delete(&model.ProductSku{}, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "删除失败"})
			return
		}
		invalidateProductCache(sku.ProductID)
		c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
	}
}
```

- [ ] **Step 6: 评价提交加失效（`internal/api/handlers_review.go`）**

在 `submitOrderReviews` 的评分更新循环里，`db.Model(&model.Product{}).Where("id = ?", r.ProductID).Update("rating", avg)` 之后加一行 `invalidateProductCache(r.ProductID)`。

- [ ] **Step 7: 运行测试，确认通过**

Run: `go test ./internal/api/ -v`
Expected: PASS（新增详情测试 + 现有商品/评价测试全过）

- [ ] **Step 8: 提交**

```bash
git add internal/api/catalog_cache.go internal/api/handlers_product.go internal/api/handlers_admin.go internal/api/handlers_review.go internal/api/catalog_cache_test.go
git commit -m "feat: 商品详情加 Redis 缓存（写时失效）"
```

---

### Task 4: 商品列表缓存

**Files:**
- Modify: `internal/api/catalog_cache.go`（加列表 + epoch 原语）
- Modify: `internal/api/handlers_product.go`（`productsHandler` 读缓存，非搜索变体）
- Test: `internal/api/catalog_cache_test.go`（列表测试）

**Interfaces:**
- Consumes: `catalogCache`、`productEpochKey`（Task 3 已 INCR）。
- Produces: `cachedProducts`、`cacheProducts`、`productEpoch`、`productListKey`。

- [ ] **Step 1: 写失败测试**

在 `internal/api/catalog_cache_test.go` 追加：

```go
func TestCachedProductsRoundTrip(t *testing.T) {
	mr := miniredis.RunT(t)
	catalogCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { catalogCache = nil })

	items := []model.Product{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}}
	cacheProducts(true, "", "sales_desc", items, 0)
	got, ok := cachedProducts(true, "", "sales_desc")
	if !ok || len(got) != 2 || got[0].Name != "A" {
		t.Fatalf("round trip: got %v, ok=%v", got, ok)
	}
}

func TestCachedProductsStaleEpoch(t *testing.T) {
	mr := miniredis.RunT(t)
	catalogCache = redisx.New(redisx.Config{Addr: mr.Addr()})
	t.Cleanup(func() { catalogCache = nil })

	cacheProducts(false, "", "id", []model.Product{{ID: 1, Name: "A"}}, 0)
	invalidateProductCache(1) // INCR product:epoch
	if _, ok := cachedProducts(false, "", "id"); ok {
		t.Fatal("expected miss after epoch bump")
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `go test ./internal/api/ -run 'TestCachedProducts' -v`
Expected: FAIL（`undefined: cacheProducts`/`cachedProducts`）

- [ ] **Step 3: 扩展 `internal/api/catalog_cache.go`（列表部分）**

追加常量与函数：

```go
const productListPrefix = "products:"

// productListEntry 是商品列表缓存值：写入时的 epoch + 列表。
type productListEntry struct {
	Epoch int64           `json:"e"`
	Items []model.Product `json:"items"`
}

// productEpoch 返回列表全局版本号。Redis 不可用返回 ok=false；epoch 键缺失视为 0。
func productEpoch() (int64, bool) {
	if catalogCache == nil || !catalogCache.Available() {
		return 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), catalogOpTimeout)
	defer cancel()
	s, err := catalogCache.Get(ctx, productEpochKey)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, true
		}
		return 0, false
	}
	epoch, _ := strconv.ParseInt(s, 10, 64)
	return epoch, true
}

// productListKey 生成列表缓存键。
func productListKey(featured bool, category, sort string) string {
	return productListPrefix + strconv.FormatBool(featured) + ":" + category + ":" + sort
}

// cachedProducts 读取并校验商品列表缓存；未命中/不可用返回 ok=false。
func cachedProducts(featured bool, category, sort string) ([]model.Product, bool) {
	if catalogCache == nil || !catalogCache.Available() {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), catalogOpTimeout)
	defer cancel()
	val, err := catalogCache.Get(ctx, productListKey(featured, category, sort))
	if err != nil {
		return nil, false
	}
	var entry productListEntry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		return nil, false
	}
	epoch, ok := productEpoch()
	if !ok {
		return nil, false
	}
	if entry.Epoch != epoch {
		return nil, false
	}
	return entry.Items, true
}

// cacheProducts 用查库前捕获的 epoch 回填列表缓存（best-effort）。
func cacheProducts(featured bool, category, sort string, items []model.Product, epoch int64) {
	if catalogCache == nil || !catalogCache.Available() {
		return
	}
	b, err := json.Marshal(productListEntry{Epoch: epoch, Items: items})
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), catalogOpTimeout)
	defer cancel()
	_ = catalogCache.Set(ctx, productListKey(featured, category, sort), string(b), productTTL)
}
```

并在 import 块加 `"errors"` 与 `"github.com/redis/go-redis/v9"`。

- [ ] **Step 4: 改 `productsHandler`（`internal/api/handlers_product.go`）**

把 `productsHandler` 主体替换为：

```go
func productsHandler(db *gorm.DB, databaseReady bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !databaseReady {
			c.JSON(http.StatusOK, model.FallbackProducts)
			return
		}
		search := c.Query("search")
		category := c.Query("category")
		if category == "all" {
			category = ""
		}
		featured := c.Query("featured") == "true"
		sort := c.Query("sort")
		if featured {
			sort = "sales_desc"
		}
		hasFilter := search != "" || category != "" || featured

		if search == "" {
			if items, ok := cachedProducts(featured, category, sort); ok {
				c.JSON(http.StatusOK, items)
				return
			}
		}

		// 查库前捕获 epoch，回填用它：避免「写后 epoch + 写前 items」的脏组合（同权限缓存）。
		var epoch int64
		if search == "" {
			epoch, _ = productEpoch()
		}

		query := db.Model(&model.Product{}).Where("on_shelf = ?", true)
		if featured {
			query = query.Where("featured = ?", true)
		}
		if search != "" {
			query = query.Where("name LIKE ? OR description LIKE ?", "%"+search+"%", "%"+search+"%")
		}
		if category != "" {
			query = query.Where("category = ?", category)
		}
		var items []model.Product
		if err := query.Order(productSortOrder(sort)).Find(&items).Error; err != nil {
			items = nil
		}
		if len(items) == 0 && !hasFilter {
			var total int64
			db.Model(&model.Product{}).Count(&total)
			if total == 0 {
				items = model.FallbackProducts
			}
		}
		if search == "" && len(items) > 0 {
			cacheProducts(featured, category, sort, items, epoch)
		}
		c.JSON(http.StatusOK, items)
	}
}
```

- [ ] **Step 5: 运行测试，确认通过**

Run: `go test ./internal/api/ -v`
Expected: PASS（新增列表测试 + 现有商品列表测试全过）

- [ ] **Step 6: 提交**

```bash
git add internal/api/catalog_cache.go internal/api/handlers_product.go internal/api/catalog_cache_test.go
git commit -m "feat: 商品列表加 Redis 缓存（非搜索变体，epoch 失效）"
```

---

## 自检记录（写作时已完成）

- **Spec 覆盖**：redisx.Del → Task 1；分类缓存 + 接线 → Task 2；商品详情 + 失效点 → Task 3；商品列表 + epoch → Task 4。无遗漏。
- **占位符扫描**：无 TBD/TODO；每步含实际代码。
- **类型一致性**：`Del(ctx,key) error`、`cachedCategories() ([]model.Category,bool)`、`cacheCategories([]model.Category)`、`invalidateCategoryCache()`、`cachedProduct(int) (model.Product,bool)`、`cacheProduct(model.Product)`、`invalidateProductCache(int)`、`cachedProducts(bool,string,string) ([]model.Product,bool)`、`cacheProducts(bool,string,string,[]model.Product,int64)`、`productEpoch() (int64,bool)` 在 Task 2/3/4 间一致。
- **Review Focus**：5 项均有对应测试或实现注意点。
- **注意**：Task 4 列表回填的 epoch 在**查库前**捕获（与权限缓存同款修复），避免「写后 epoch + 写前 items」脏组合。
