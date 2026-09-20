package api

import (
	"testing"

	"shop/internal/model"
)

func TestApplyCouponDiscount(t *testing.T) {
	coupon := &model.Coupon{MinAmount: 99, Amount: 10}

	if pay, discount := applyCouponDiscount(150, coupon); pay != 140 || discount != 10 {
		t.Fatalf("eligible: pay=%v discount=%v", pay, discount)
	}
	if pay, discount := applyCouponDiscount(50, coupon); pay != 50 || discount != 0 {
		t.Fatalf("below threshold: pay=%v discount=%v", pay, discount)
	}
	if pay, discount := applyCouponDiscount(100, nil); pay != 100 || discount != 0 {
		t.Fatalf("nil coupon: pay=%v discount=%v", pay, discount)
	}
	if pay, discount := applyCouponDiscount(5, &model.Coupon{MinAmount: 0, Amount: 10}); pay != 0 || discount != 5 {
		t.Fatalf("cap: pay=%v discount=%v", pay, discount)
	}
}
