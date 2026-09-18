package api

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"shop/internal/auth"
	"shop/internal/model"
)

// TestSSEPushEndToEnd 走真实的 JWT 中间件 + SSE handler，验证服务器能主动把通知推给订阅者。
func TestSSEPushEndToEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authMW, err := auth.New(nil)
	if err != nil {
		t.Fatalf("auth.New: %v", err)
	}
	token, _, err := authMW.TokenGenerator("admin")
	if err != nil {
		t.Fatalf("TokenGenerator: %v", err)
	}

	hub := newNotificationHub()
	r := gin.New()
	r.GET("/api/notifications/stream", tokenFromQuery(), authMW.MiddlewareFunc(), streamNotifications(hub))

	srv := httptest.NewServer(r)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/api/notifications/stream?token="+token, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer resp.Body.Close()

	// 连接建立后再广播，模拟服务器主动推送。
	time.Sleep(100 * time.Millisecond)
	notify(nil, hub, "admin", model.NotificationTypeCoupon, "测试推送", "服务器主动推送的内容", "")

	reader := bufio.NewReader(resp.Body)
	deadline := time.Now().Add(2 * time.Second)
	var gotData string
	for time.Now().Before(deadline) {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			gotData = strings.TrimPrefix(line, "data:")
			break
		}
	}
	if gotData == "" {
		t.Fatal("did not receive SSE notification data")
	}
	if !strings.Contains(gotData, "测试推送") {
		t.Fatalf("unexpected data %q", gotData)
	}
}
