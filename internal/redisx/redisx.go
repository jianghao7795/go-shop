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
		Addr:     envOr("REDIS_ADDR", "192.168.57.47:6379"),
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
