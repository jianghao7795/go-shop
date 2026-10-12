package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimiter(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rl := newRateLimiterWithClock(time.Minute, 3, func() time.Time { return now })
	ip := "1.2.3.4"

	for i := 0; i < 3; i++ {
		if !rl.Allow(ip) {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if rl.Allow(ip) {
		t.Fatal("4th request within window should be rejected")
	}
	// 窗口过后重置
	now = now.Add(time.Minute)
	if !rl.Allow(ip) {
		t.Fatal("after window should be allowed again")
	}
	// 不同 IP 互不影响
	if !rl.Allow("5.6.7.8") {
		t.Fatal("different IP should be allowed")
	}
}

func TestRateLimitMiddlewareRejectsOverLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rl := newRateLimiter(time.Minute, 1)
	router := gin.New()
	router.POST("/x", rateLimitMiddleware(rl), func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("POST", "/x", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("first request: got %d, want 200", w.Code)
	}
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, httptest.NewRequest("POST", "/x", nil))
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: got %d, want 429", w2.Code)
	}
}
