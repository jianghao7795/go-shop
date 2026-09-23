package api

import (
	"strings"
	"testing"
)

func TestValidateProfile(t *testing.T) {
	tests := []struct {
		name string
		p    profileUpdate
		ok   bool
	}{
		{"全空", profileUpdate{}, true},
		{"合法手机号", profileUpdate{Mobile: "13800138000"}, true},
		{"合法邮箱", profileUpdate{Email: "a@b.com"}, true},
		{"合法头像", profileUpdate{Avatar: "https://example.com/a.png"}, true},
		{"合法昵称", profileUpdate{Nickname: "张三"}, true},

		{"手机号少一位", profileUpdate{Mobile: "1380013800"}, false},
		{"手机号以0开头", profileUpdate{Mobile: "03800138000"}, false},
		{"手机号以2开头", profileUpdate{Mobile: "23800138000"}, false},
		{"手机号含字母", profileUpdate{Mobile: "1380013800a"}, false},

		{"邮箱缺@", profileUpdate{Email: "ab.com"}, false},
		{"邮箱缺域名点", profileUpdate{Email: "a@b"}, false},

		{"头像无协议", profileUpdate{Avatar: "example.com/a.png"}, false},
		{"头像非http协议", profileUpdate{Avatar: "ftp://example.com/a.png"}, false},

		{"昵称超长", profileUpdate{Nickname: strings.Repeat("a", 65)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateProfile(tt.p)
			if (got == "") != tt.ok {
				t.Fatalf("validateProfile(%+v) = %q, want ok=%v", tt.p, got, tt.ok)
			}
		})
	}
}
