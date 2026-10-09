package api

import (
	"testing"

	"shop/internal/model"
)

func TestShipNextOrderSkipsPending(t *testing.T) {
	db := newTestDB(t)
	pending := model.Order{OrderNo: "P1", UserID: "u1", Status: model.OrderStatusPending}
	db.Create(&pending)

	shipNextOrder(db, newNotificationHub())

	var got model.Order
	db.First(&got, pending.ID)
	if got.Status != model.OrderStatusPending {
		t.Fatalf("pending 订单被发货: status = %q", got.Status)
	}
}

func TestShipNextOrderShipsPaid(t *testing.T) {
	db := newTestDB(t)
	paid := model.Order{OrderNo: "P2", UserID: "u1", Status: model.OrderStatusPaid}
	db.Create(&paid)

	shipNextOrder(db, newNotificationHub())

	var got model.Order
	db.First(&got, paid.ID)
	if got.Status != model.OrderStatusShipped {
		t.Fatalf("paid 订单未发货: status = %q", got.Status)
	}
}

func TestSimulatorsEnabled(t *testing.T) {
	t.Setenv("ENABLE_SIMULATORS", "true")
	if !simulatorsEnabled() {
		t.Fatal("ENABLE_SIMULATORS=true 应启用")
	}
	t.Setenv("ENABLE_SIMULATORS", "")
	if simulatorsEnabled() {
		t.Fatal("未设置应默认关闭")
	}
	t.Setenv("ENABLE_SIMULATORS", "false")
	if simulatorsEnabled() {
		t.Fatal("ENABLE_SIMULATORS=false 应关闭")
	}
}
