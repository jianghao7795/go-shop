package api

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"shop/internal/model"
)

// newTestDB 建内存库并迁移订单/通知表。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Order{}, &model.Notification{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestMarkPaid(t *testing.T) {
	db := newTestDB(t)
	order := model.Order{OrderNo: "O1", UserID: "u1", Status: model.OrderStatusPending, Amount: 19.9}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}

	if err := markPaid(db, order.ID, "trade-123"); err != nil {
		t.Fatalf("markPaid: %v", err)
	}

	var got model.Order
	db.First(&got, order.ID)
	if got.Status != model.OrderStatusPaid {
		t.Fatalf("status = %q, want paid", got.Status)
	}
	if got.TradeNo != "trade-123" {
		t.Fatalf("tradeNo = %q, want trade-123", got.TradeNo)
	}
	if got.PayMethod != "alipay" {
		t.Fatalf("payMethod = %q, want alipay", got.PayMethod)
	}
	if got.PaidAt == nil {
		t.Fatal("paidAt is nil")
	}
}

func TestMarkPaidIdempotent(t *testing.T) {
	db := newTestDB(t)
	order := model.Order{OrderNo: "O2", UserID: "u1", Status: model.OrderStatusPending}
	db.Create(&order)

	if err := markPaid(db, order.ID, "trade-1"); err != nil {
		t.Fatalf("first markPaid: %v", err)
	}
	// 第二次调用（模拟轮询与通知并发）不应覆盖 trade_no。
	if err := markPaid(db, order.ID, "trade-2"); err != nil {
		t.Fatalf("second markPaid: %v", err)
	}

	var got model.Order
	db.First(&got, order.ID)
	if got.TradeNo != "trade-1" {
		t.Fatalf("tradeNo = %q, want trade-1 (unchanged)", got.TradeNo)
	}
	if got.Status != model.OrderStatusPaid {
		t.Fatalf("status = %q, want paid", got.Status)
	}
}
