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
