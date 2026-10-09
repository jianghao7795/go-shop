package api

import "testing"

func TestCorsAllowed(t *testing.T) {
	if !corsAllowed("", "") {
		t.Fatal("empty origin should be allowed")
	}
	if !corsAllowed("http://localhost:9245", "") {
		t.Fatal("localhost origin should be allowed")
	}
	if !corsAllowed("http://127.0.0.1:8080", "") {
		t.Fatal("127.0.0.1 origin should be allowed")
	}
	if !corsAllowed("http://wails.localhost:9245", "") {
		t.Fatal("wails.localhost 子域应被允许")
	}
	if corsAllowed("https://evil.com", "") {
		t.Fatal("unknown origin should be denied")
	}
	if !corsAllowed("https://app.example.com", "https://app.example.com") {
		t.Fatal("whitelisted origin should be allowed")
	}
}
