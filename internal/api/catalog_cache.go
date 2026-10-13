package api

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

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
	_ = catalogCache.Del(ctx, categoryKey)
}

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

// invalidateProductCache 商品变更：DEL 详情 + INCR 列表 epoch（epoch 由列表缓存消费）。
func invalidateProductCache(id int) {
	if catalogCache == nil || !catalogCache.Available() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), catalogOpTimeout)
	defer cancel()
	_ = catalogCache.Del(ctx, productKeyPrefix+strconv.Itoa(id))
	_, _ = catalogCache.Incr(ctx, productEpochKey)
}

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
