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
	_ = catalogCache.Del(ctx, categoryKey)
}
