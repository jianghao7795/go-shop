package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"shop/internal/model"
)

func TestApplyCouponDiscount(t *testing.T) {
	coupon := &model.Coupon{MinAmount: 9900, Amount: 1000}

	if pay, discount := applyCouponDiscount(15000, coupon); pay != 14000 || discount != 1000 {
		t.Fatalf("eligible: pay=%v discount=%v", pay, discount)
	}
	if pay, discount := applyCouponDiscount(5000, coupon); pay != 5000 || discount != 0 {
		t.Fatalf("below threshold: pay=%v discount=%v", pay, discount)
	}
	if pay, discount := applyCouponDiscount(10000, nil); pay != 10000 || discount != 0 {
		t.Fatalf("nil coupon: pay=%v discount=%v", pay, discount)
	}
	if pay, discount := applyCouponDiscount(500, &model.Coupon{MinAmount: 0, Amount: 1000}); pay != 0 || discount != 500 {
		t.Fatalf("cap: pay=%v discount=%v", pay, discount)
	}
}

func TestCouponInPeriod(t *testing.T) {
	now := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)
	if !couponInPeriod(&model.Coupon{StartAt: start, EndAt: end}, now) {
		t.Fatalf("inside period should be valid")
	}
	if couponInPeriod(&model.Coupon{StartAt: now.Add(time.Minute), EndAt: end}, now) {
		t.Fatalf("not yet started should be invalid")
	}
	if couponInPeriod(&model.Coupon{StartAt: start, EndAt: now.Add(-time.Minute)}, now) {
		t.Fatalf("expired should be invalid")
	}
	if !couponInPeriod(&model.Coupon{StartAt: now, EndAt: end}, now) {
		t.Fatalf("start boundary should be valid")
	}
	if !couponInPeriod(&model.Coupon{StartAt: start, EndAt: now}, now) {
		t.Fatalf("end boundary should be valid")
	}
	if couponInPeriod(nil, now) {
		t.Fatalf("nil coupon should be invalid")
	}
}

func TestAllowedUserOrderTransition(t *testing.T) {
	allowed := []struct{ from, to string }{
		{model.OrderStatusShipped, model.OrderStatusCompleted},
		{model.OrderStatusPending, model.OrderStatusCancelled},
		{model.OrderStatusPaid, model.OrderStatusAftersale},
		{model.OrderStatusShipped, model.OrderStatusAftersale},
	}
	for _, tt := range allowed {
		if !allowedUserOrderTransition(tt.from, tt.to) {
			t.Fatalf("%s->%s should be allowed", tt.from, tt.to)
		}
	}
	rejected := []struct{ from, to string }{
		{model.OrderStatusPending, model.OrderStatusPaid},
		{model.OrderStatusPending, model.OrderStatusFinished},
		{model.OrderStatusCompleted, model.OrderStatusFinished},
		{model.OrderStatusShipped, model.OrderStatusPaid},
		{model.OrderStatusPaid, model.OrderStatusCancelled},
		{model.OrderStatusCompleted, model.OrderStatusCancelled},
		{"", model.OrderStatusCompleted},
	}
	for _, tt := range rejected {
		if allowedUserOrderTransition(tt.from, tt.to) {
			t.Fatalf("%s->%s should be rejected", tt.from, tt.to)
		}
	}
}

func TestRepricedItems(t *testing.T) {
	products := map[int]model.Product{
		1: {ID: 1, Name: "A", Price: 1000, Emoji: "a", Color: "#fff", OnShelf: true},
		2: {ID: 2, Name: "B", Price: 2000, Emoji: "b", Color: "#000", OnShelf: true},
		3: {ID: 3, Name: "C", Price: 3000, OnShelf: false},
	}
	req := []model.OrderItem{
		{ProductID: 1, Name: "hacked", Price: 1, Quantity: 2, Emoji: "x", Color: "y"},
		{ProductID: 2, Quantity: 1},
	}
	items, msg := repricedItems(products, nil, req)
	if msg != "" {
		t.Fatalf("expected ok, got %q", msg)
	}
	if len(items) != 2 {
		t.Fatalf("len=%d, want 2", len(items))
	}
	if items[0].Price != 1000 || items[0].Name != "A" || items[0].Emoji != "a" || items[0].Color != "#fff" || items[0].Quantity != 2 {
		t.Fatalf("item0 not repriced: %+v", items[0])
	}
	if items[1].Price != 2000 || items[1].Name != "B" {
		t.Fatalf("item1 not repriced: %+v", items[1])
	}
	if _, msg := repricedItems(products, nil, []model.OrderItem{{ProductID: 99, Quantity: 1}}); msg == "" {
		t.Fatalf("missing product should error")
	}
	if _, msg := repricedItems(products, nil, []model.OrderItem{{ProductID: 3, Quantity: 1}}); msg == "" {
		t.Fatalf("off-shelf product should error")
	}
	if _, msg := repricedItems(products, nil, []model.OrderItem{{ProductID: 1, Quantity: 0}}); msg == "" {
		t.Fatalf("zero quantity should error")
	}
}

func TestRepricedItemsWithSku(t *testing.T) {
	products := map[int]model.Product{1: {ID: 1, Name: "A", Price: 1000, OnShelf: true}}
	skus := map[int]model.ProductSku{10: {ID: 10, ProductID: 1, Specs: "红色", Price: 1500}}
	items, msg := repricedItems(products, skus, []model.OrderItem{{ProductID: 1, SkuID: 10, Quantity: 1}})
	if msg != "" {
		t.Fatalf("expected ok, got %q", msg)
	}
	if items[0].Price != 1500 || items[0].Specs != "红色" || items[0].SkuID != 10 {
		t.Fatalf("sku not applied: %+v", items[0])
	}
	if _, msg := repricedItems(products, skus, []model.OrderItem{{ProductID: 1, SkuID: 99, Quantity: 1}}); msg == "" {
		t.Fatalf("missing sku should error")
	}
	if _, msg := repricedItems(products, skus, []model.OrderItem{{ProductID: 2, SkuID: 10, Quantity: 1}}); msg == "" {
		t.Fatalf("sku belonging to other product should error")
	}
}

func createOrderCtx(t *testing.T, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/orders", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, w
}

func TestCreateOrderInsufficientStock(t *testing.T) {
	db := newTestDB(t)
	p := model.Product{Name: "A", Price: 1000, Stock: 1, OnShelf: true}
	db.Create(&p)
	c, w := createOrderCtx(t, fmt.Sprintf(`{"items":[{"productId":%d,"quantity":2}]}`, p.ID))
	createOrder(db)(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", w.Code, w.Body.String())
	}
	var n int64
	db.Model(&model.Order{}).Count(&n)
	if n != 0 {
		t.Fatalf("order created despite insufficient stock: count=%d", n)
	}
}

func TestCreateOrderSufficientStock(t *testing.T) {
	db := newTestDB(t)
	p := model.Product{Name: "A", Price: 1000, Stock: 5, OnShelf: true}
	db.Create(&p)
	c, w := createOrderCtx(t, fmt.Sprintf(`{"items":[{"productId":%d,"quantity":2}]}`, p.ID))
	createOrder(db)(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
}

func TestGenerateOrderNo(t *testing.T) {
	a := generateOrderNo()
	b := generateOrderNo()
	if a == "" {
		t.Fatal("orderNo empty")
	}
	if a == b {
		t.Fatal("two orderNos should differ")
	}
	for _, ch := range a {
		if ch < '0' || ch > '9' {
			t.Fatalf("orderNo should be digits only: %q", a)
		}
	}
}
