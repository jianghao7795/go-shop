package api

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"shop/internal/redisx"
)

func TestRedisLimiterFixedWindow(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redisx.New(redisx.Config{Addr: mr.Addr()})
	rl := newRedisLimiter(client, "test", time.Minute, 3)
	ip := "1.2.3.4"

	for i := 0; i < 3; i++ {
		ok, err := rl.tryAllow(ip)
		if err != nil || !ok {
			t.Fatalf("request %d should be allowed, got ok=%v err=%v", i+1, ok, err)
		}
	}
	if ok, _ := rl.tryAllow(ip); ok {
		t.Fatal("4th request within window should be rejected")
	}
	// 窗口过后重置
	mr.FastForward(time.Minute)
	if ok, err := rl.tryAllow(ip); err != nil || !ok {
		t.Fatalf("after window should be allowed again, got ok=%v err=%v", ok, err)
	}
	// 不同 IP 互不影响
	if ok, _ := rl.tryAllow("5.6.7.8"); !ok {
		t.Fatal("different IP should be allowed")
	}
}

func TestRedisLimiterKeyIsolation(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redisx.New(redisx.Config{Addr: mr.Addr()})
	login := newRedisLimiter(client, "login", time.Minute, 3)
	register := newRedisLimiter(client, "register", time.Minute, 3)
	if _, err := login.tryAllow("1.2.3.4"); err != nil {
		t.Fatalf("login tryAllow: %v", err)
	}
	if _, err := register.tryAllow("1.2.3.4"); err != nil {
		t.Fatalf("register tryAllow: %v", err)
	}
	if !mr.Exists("rl:login:1.2.3.4") || !mr.Exists("rl:register:1.2.3.4") {
		t.Fatal("login and register should each have their own key")
	}
}

func TestFallbackLimiterUsesRedisWhenAvailable(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redisx.New(redisx.Config{Addr: mr.Addr()})
	l := newFallbackLimiter(client, "login", time.Minute, 3)
	if !l.Allow("1.2.3.4") {
		t.Fatal("first request should be allowed")
	}
	// 走的是 Redis：key 应已写入
	if !mr.Exists("rl:login:1.2.3.4") {
		t.Fatal("expected redis key rl:login:1.2.3.4 to exist")
	}
}

func TestFallbackLimiterUsesMemoryWhenUnavailable(t *testing.T) {
	// available=false 直接走内存；redis 为 nil 也不应被触碰。
	l := &fallbackLimiter{
		redis:     nil,
		memory:    newRateLimiter(time.Minute, 1),
		available: func() bool { return false },
	}
	if !l.Allow("1.2.3.4") {
		t.Fatal("first request should be allowed")
	}
	if l.Allow("1.2.3.4") {
		t.Fatal("second request should be rejected by memory limiter")
	}
}

func TestFallbackLimiterDegradesOnRedisError(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redisx.New(redisx.Config{Addr: mr.Addr()})
	l := newFallbackLimiter(client, "login", time.Minute, 1)
	// 启动时 available 已固定为 true；关闭 Redis 模拟中途宕机，tryAllow 返回错误
	mr.Close()
	if !l.Allow("1.2.3.4") {
		t.Fatal("first request should fall back to memory and be allowed")
	}
	if l.Allow("1.2.3.4") {
		t.Fatal("second request should be rejected by memory limiter")
	}
}

func TestFallbackLimiterLogsOnRedisError(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redisx.New(redisx.Config{Addr: mr.Addr()})

	var logged []error
	l := &fallbackLimiter{
		redis:     newRedisLimiter(client, "login", time.Minute, 1),
		memory:    newRateLimiter(time.Minute, 1),
		available: client.Available,
		logErr:    func(err error) { logged = append(logged, err) },
	}

	// Redis 正常时不应记录错误
	if !l.Allow("1.2.3.4") {
		t.Fatal("first request should be allowed")
	}
	if len(logged) != 0 {
		t.Fatalf("expected no log on success, got %d", len(logged))
	}

	// 模拟 Redis 中途宕机：应落到内存并记录一次错误
	mr.Close()
	if !l.Allow("2.2.2.2") {
		t.Fatal("should fall back to memory and allow")
	}
	if len(logged) != 1 {
		t.Fatalf("expected 1 logged error, got %d", len(logged))
	}
}

func TestThrottledErrLogger(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var msgs []string
	logf := func(format string, args ...any) { msgs = append(msgs, fmt.Sprintf(format, args...)) }
	l := newThrottledErrLogger(func() time.Time { return now }, time.Minute, logf)

	l(errors.New("a"))
	l(errors.New("b")) // 同一窗口内应被限频
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	now = now.Add(time.Minute)
	l(errors.New("c"))
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after window, got %d", len(msgs))
	}
}
