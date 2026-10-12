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

// permEpoch 返回当前全局版本号。Redis 不可用返回 ok=false；epoch 键缺失视为 0。
func permEpoch() (int64, bool) {
	if permCache == nil || !permCache.Available() {
		return 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), permOpTimeout)
	defer cancel()
	s, err := permCache.Get(ctx, permEpochKey)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, true
		}
		return 0, false
	}
	epoch, _ := strconv.ParseInt(s, 10, 64)
	return epoch, true
}

// cachedEffectivePermissions 读取并校验缓存的权限码；未命中/不可用返回 ok=false。
func cachedEffectivePermissions(username string) ([]string, bool) {
	if permCache == nil || !permCache.Available() {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), permOpTimeout)
	defer cancel()

	// 先读用户键、再读 epoch：若 RBAC 写落在两次读之间，entry.Epoch != epoch → miss（保守正确），
	// 不会出现「读旧 epoch + 读到已过期 entry」的错误命中。
	val, err := permCache.Get(ctx, permKeyPrefix+username)
	if err != nil {
		return nil, false
	}
	var entry permCacheEntry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		return nil, false
	}

	epoch, ok := permEpoch()
	if !ok {
		return nil, false
	}
	if entry.Epoch != epoch {
		return nil, false
	}
	return entry.Codes, true
}

// cacheEffectivePermissions 用查库前捕获的 epoch 回填缓存（best-effort，忽略错误）。
func cacheEffectivePermissions(username string, codes []string, epoch int64) {
	if permCache == nil || !permCache.Available() {
		return
	}
	b, err := json.Marshal(permCacheEntry{Epoch: epoch, Codes: codes})
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), permOpTimeout)
	defer cancel()
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
