package api

import (
	"context"
	"encoding/json"
	"strconv"
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

// invalidateProductCache 商品变更：DEL 详情 + INCR 列表 epoch（epoch 由 Task 4 的列表缓存消费）。
func invalidateProductCache(id int) {
	if catalogCache == nil || !catalogCache.Available() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), catalogOpTimeout)
	defer cancel()
	_ = catalogCache.Del(ctx, productKeyPrefix+strconv.Itoa(id))
	_, _ = catalogCache.Incr(ctx, productEpochKey)
}
