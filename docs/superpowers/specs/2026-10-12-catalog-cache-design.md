# 商品/分类缓存设计

- 日期：2026-10-12
- 状态：待评审
- 规模：架构级（扩展 redisx 加 Del + 分类整表缓存 + 商品详情/列表缓存 + 写时失效）

## 目标

给商品目录（分类、商品详情、商品列表）加 Redis 缓存，消除高频读路径对 MySQL 的重复查询；写操作后立即失效（按 key DEL + 列表全局 epoch），Redis 不可用回退直查 DB。复用已建成的 `internal/redisx` 与 cache-aside 模式。

## 现状

- `internal/api/handlers_category.go:13` `listCategories` 每次读整表 `shop_categories`，DB 不可用/空时回退 `model.Categories`。
- `internal/api/handlers_product.go:19` `productDetailHandler` 查商品 + 该商品的 SKU。
- `internal/api/handlers_product.go:60` `productsHandler` 按 `search`/`category`/`featured` 过滤 + `sort` 排序查商品列表，空表回退 `model.FallbackProducts`。
- 分类 CRUD：`adminCreateCategory`/`adminUpdateCategory`/`adminDeleteCategory`；商品 CRUD：`adminCreateProduct`/`adminUpdateProduct`/`adminDeleteProduct`/`adminToggleFeatured`/`adminCreateSku`/`adminDeleteSku`；评价 `submitOrderReviews` 会更新商品 `rating`。
- 已建成：`redisx`（`Get`/`Set`/`Incr`/`IncrWithTTL`，本次补 `Del`）；权限缓存用 package 级 `permCache` + 全局 epoch。

## 关键决策

1. **失效机制分层**：
   - 分类（单键）与商品详情（按 id 单键）用 **DEL 精准失效**。
   - 商品列表（键空间 = featured×category×sort，一个商品改动会影响多个列表键）用 **全局 epoch**（`INCR product:epoch`），与权限缓存同一套模式，避免漏失效。
2. **列表不缓存 search**：`search` 是 `LIKE` 查询、key 空间无界且非热点，一律直查 DB。
3. **缓存只回填真实 DB 结果**，不回填 `FallbackProducts`/`Categories` 兜底数据。
4. **分类与商品失效互不干扰**：商品 `category` 是冗余字符串（非外键），分类改名不改变商品列表内容。
5. **接入方式：package 级 `catalogCache *redisx.Client`**，与 `permCache` 指向同一连接池、各自独立，测试互不污染。

## 后端设计

### `internal/redisx/redisx.go`（扩展）

新增：

```go
// Del 删除 key（无 key 也不报错）。
func (c *Client) Del(ctx context.Context, key string) error
```

### 分类缓存（`internal/api/handlers_category.go` + 新文件 `internal/api/catalog_cache.go`）

- 键 `categories`，值 JSON `[]model.Category`，TTL 10 分钟。
- `listCategories`：先读缓存（`catalogCache` 可用且命中即返回），miss 则查库回填。DB 为空/不可用时仍走现有兜底逻辑，不写缓存。
- `adminCreateCategory`/`adminUpdateCategory`/`adminDeleteCategory` 成功后调 `invalidateCategoryCache()`（DEL `categories`）。

### 商品详情缓存

- 键 `product:{id}`，值 JSON `model.Product`（含 `skus`，因 `Skus` 有 `json:"skus"` 标签可正常序列化），TTL 5 分钟。
- `productDetailHandler`：先读缓存，miss 查库 + SKU 回填；仅缓存「已上架」商品。
- 失效（DEL `product:{id}`）：`adminUpdateProduct`、`adminDeleteProduct`、`adminToggleFeatured`、`adminCreateSku`、`adminDeleteSku`、`submitOrderReviews`（每个被评商品）。

### 商品列表缓存

- 键 `products:{featured}:{category}:{sort}`，值 JSON `{"e":<epoch>,"items":[]model.Product}`，TTL 5 分钟；epoch 键 `product:epoch`。
- `productsHandler`：`search != ""` → 直查 DB；否则读缓存（epoch 校验命中即返回），miss 查库回填（空表仍回退兜底、不写缓存）。
- 失效（`INCR product:epoch`）：所有商品写操作（create/update/delete/toggleFeatured/createSku/deleteSku/评价）。

### 帮助函数（`internal/api/catalog_cache.go`）

- `invalidateProductCache(id int)`：DEL `product:{id}` + INCR `product:epoch`。
- `invalidateCategoryCache()`：DEL `categories`。

### `internal/api/server.go`（接线）

在现有 `permCache = rds` 之后加 `catalogCache = rds`。

## 键结构

| 键 | 类型 | 值 | TTL |
|---|---|---|---|
| `categories` | JSON | `[]Category` | 10 分钟 |
| `product:{id}` | JSON | `Product`（含 skus） | 5 分钟 |
| `products:{featured}:{category}:{sort}` | JSON | `{"e":epoch,"items":[...]}` | 5 分钟 |
| `product:epoch` | 整数 | 列表全局版本号 | 无 |

## 一致性 / 降级 / 超时

- 分类/详情：写后 DEL，立即生效；TTL 兜底清理。
- 列表：写后 INCR epoch，立即失效；TTL 兜底。
- `catalogCache` 为 nil 或 `Available()==false` → 直查 DB。
- 每个 Redis 操作 200ms 超时；fail-fast 熔断已作为非目标延后。

## 测试

- `redisx.Del`（miniredis：删后 `Get` 返回 `redis.Nil`；不可用返回 `ErrUnavailable`）。
- 分类缓存：读回填、`invalidateCategoryCache` 后回源、`catalogCache=nil` 走 DB。
- 商品详情缓存：回填 `product:{id}`、写后失效回源、已下架不缓存。
- 商品列表缓存：非搜索变体命中、search 直查、epoch 失配回源。

## 非目标（YAGNI）

- 不缓存 search（LIKE）结果。
- 不做商品列表按 sort 的全量预计算/预热。
- 不做按受影响列表键的精准失效（用全局 epoch）。
- 不做 redisx 的 fail-fast 熔断（已延后）。
