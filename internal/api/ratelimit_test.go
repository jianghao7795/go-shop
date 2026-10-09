package api

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rl := newRateLimiterWithClock(time.Minute, 3, func() time.Time { return now })
	ip := "1.2.3.4"

	for i := 0; i < 3; i++ {
		if !rl.allow(ip) {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if rl.allow(ip) {
		t.Fatal("4th request within window should be rejected")
	}
	// 窗口过后重置
	now = now.Add(time.Minute)
	if !rl.allow(ip) {
		t.Fatal("after window should be allowed again")
	}
	// 不同 IP 互不影响
	if !rl.allow("5.6.7.8") {
		t.Fatal("different IP should be allowed")
	}
}
