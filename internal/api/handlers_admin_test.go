package api

import "testing"

func TestValidateProduct(t *testing.T) {
	tests := []struct {
		name string
		p    productPayload
		ok   bool
	}{
		{"合法", productPayload{Name: "商品A", Price: 10, Category: "Home"}, true},
		{"缺名称", productPayload{Price: 10, Category: "Home"}, false},
		{"价格为负", productPayload{Name: "A", Price: -1, Category: "Home"}, false},
		{"缺分类", productPayload{Name: "A", Price: 10}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateProduct(tt.p); (got == "") != tt.ok {
				t.Fatalf("validateProduct(%+v)=%q, ok=%v", tt.p, got, tt.ok)
			}
		})
	}
}

func TestValidateCategory(t *testing.T) {
	tests := []struct {
		name string
		p    categoryPayload
		ok   bool
	}{
		{"合法", categoryPayload{Key: "Home", Name: "家居生活"}, true},
		{"缺 key", categoryPayload{Name: "家居生活"}, false},
		{"缺名称", categoryPayload{Key: "Home"}, false},
		{"空白 key", categoryPayload{Key: "  ", Name: "家居生活"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateCategory(tt.p); (got == "") != tt.ok {
				t.Fatalf("validateCategory(%+v)=%q, ok=%v", tt.p, got, tt.ok)
			}
		})
	}
}

func TestValidateCoupon(t *testing.T) {
	tests := []struct {
		name string
		p    couponPayload
		ok   bool
	}{
		{"合法", couponPayload{Title: "新人专享券", Amount: 10, MinAmount: 99}, true},
		{"缺标题", couponPayload{Amount: 10, MinAmount: 99}, false},
		{"金额为 0", couponPayload{Title: "A", Amount: 0, MinAmount: 99}, false},
		{"金额为负", couponPayload{Title: "A", Amount: -1, MinAmount: 99}, false},
		{"门槛为负", couponPayload{Title: "A", Amount: 10, MinAmount: -1}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateCoupon(tt.p); (got == "") != tt.ok {
				t.Fatalf("validateCoupon(%+v)=%q, ok=%v", tt.p, got, tt.ok)
			}
		})
	}
}
