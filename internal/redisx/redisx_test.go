package redisx

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestIncrWithTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	client := New(Config{Addr: mr.Addr()})
	if !client.Available() {
		t.Fatal("client should be available against miniredis")
	}
	ctx := context.Background()

	n, err := client.IncrWithTTL(ctx, "k", time.Minute)
	if err != nil || n != 1 {
		t.Fatalf("first incr: got %d, %v; want 1, nil", n, err)
	}
	if n, _ = client.IncrWithTTL(ctx, "k", time.Minute); n != 2 {
		t.Fatalf("second incr: got %d; want 2", n)
	}
	if ttl := mr.TTL("k"); ttl <= 0 || ttl > time.Minute {
		t.Fatalf("ttl: got %v; want >0 and <=1m", ttl)
	}
}

func TestIncrWithTTLShortTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	client := New(Config{Addr: mr.Addr()})
	// ttl 不足 1s 时保底 1s，避免 EXPIRE 0 立即删除导致窗口失效
	if _, err := client.IncrWithTTL(context.Background(), "k", time.Millisecond); err != nil {
		t.Fatalf("short ttl incr: %v", err)
	}
	if ttl := mr.TTL("k"); ttl <= 0 {
		t.Fatalf("short ttl should still set expiry, got %v", ttl)
	}
}

func TestNewUnavailable(t *testing.T) {
	// 端口 1 无监听，连接立即被拒绝
	client := New(Config{Addr: "127.0.0.1:1"})
	if client.Available() {
		t.Fatal("client should be unavailable against closed port")
	}
	if _, err := client.IncrWithTTL(context.Background(), "k", time.Minute); err != ErrUnavailable {
		t.Fatalf("IncrWithTTL when unavailable: got %v; want ErrUnavailable", err)
	}
}
