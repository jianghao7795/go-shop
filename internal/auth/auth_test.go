package auth

import "testing"

func TestResolveJWTSecret(t *testing.T) {
	if got := resolveJWTSecret("abc"); got != "abc" {
		t.Fatalf("已配置的密钥应原样返回, got %q", got)
	}
	a := resolveJWTSecret("")
	b := resolveJWTSecret("")
	if a == "" || b == "" {
		t.Fatal("未配置时应生成随机密钥")
	}
	if a == "change-this-shop-jwt-secret" {
		t.Fatal("不应回退到硬编码默认密钥")
	}
	if a == b {
		t.Fatal("两次生成的随机密钥应不同")
	}
}
