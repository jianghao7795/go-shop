package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"shop/internal/model"
)

type lifecyclePayer struct {
	mockPayer
	closeFn  func() error
	refundFn func(string, string, string) error
}

func (p *lifecyclePayer) Close(context.Context, string) error {
	if p.closeFn != nil {
		return p.closeFn()
	}
	return nil
}
func (p *lifecyclePayer) Refund(_ context.Context, order, request, amount string) error {
	if p.refundFn == nil {
		return errors.New("unexpected refund")
	}
	return p.refundFn(order, request, amount)
}

func lifecycleRequest(t *testing.T, h gin.HandlerFunc, id uint, body string) *httptest.ResponseRecorder {
	t.Helper()
	c, w := createOrderCtx(t, body)
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprint(id)}}
	h(c)
	return w
}

func requireStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status=%d want=%d body=%s", w.Code, want, w.Body.String())
	}
}

func paidLifecycleOrder(t *testing.T, db *gorm.DB, name string) (model.Order, model.Product) {
	t.Helper()
	p := model.Product{Name: name, Stock: 5, Price: 1000, OnShelf: true}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	o := model.Order{OrderNo: name, Status: model.OrderStatusPending, Amount: 2000, LifecycleVersion: 1, Items: []model.OrderItem{{ProductID: p.ID, Quantity: 2, Price: 1000}}}
	if err := db.Create(&o).Error; err != nil {
		t.Fatal(err)
	}
	if err := markPaid(db, o.ID, "trade-"+name); err != nil {
		t.Fatal(err)
	}
	db.First(&o, o.ID)
	return o, p
}

func TestRefundRetryRestoresStockAndCouponOnce(t *testing.T) {
	db := newTestDB(t)
	hub := newNotificationHub()
	o, p := paidLifecycleOrder(t, db, "refund-retry")
	uc := model.UserCoupon{UserID: "", Status: model.UserCouponStatusUsed, UsedOrderNo: o.OrderNo}
	db.Create(&uc)
	db.Model(&o).Update("user_coupon_id", uc.ID)
	requireStatus(t, lifecycleRequest(t, applyAftersale(db, hub), o.ID, `{"type":"refund_only","reason":"不再需要"}`), 200)
	var requests []string
	payer := &lifecyclePayer{refundFn: func(order, request, amount string) error {
		if order != o.OrderNo || amount != "20.00" {
			t.Fatalf("wrong refund %s %s", order, amount)
		}
		requests = append(requests, request)
		if len(requests) == 1 {
			return errors.New("network timeout")
		}
		return nil
	}}
	handler := adminAftersale(db, hub, payer)
	requireStatus(t, lifecycleRequest(t, handler, o.ID, `{"action":"refund"}`), 502)
	db.First(&p, p.ID)
	db.First(&o, o.ID)
	if p.Stock != 3 || o.AftersaleStatus != "refunding" {
		t.Fatalf("premature restoration: %+v %+v", o, p)
	}
	requireStatus(t, lifecycleRequest(t, handler, o.ID, `{"action":"reject","note":"不能驳回退款中订单"}`), 400)
	requireStatus(t, lifecycleRequest(t, handler, o.ID, `{"action":"refund"}`), 200)
	requireStatus(t, lifecycleRequest(t, handler, o.ID, `{"action":"refund"}`), 200)
	db.First(&o, o.ID)
	db.First(&p, p.ID)
	db.First(&uc, uc.ID)
	if o.Status != model.OrderStatusRefunded || o.RefundedAt == nil || p.Stock != 5 || p.Sales != 0 || uc.Status != model.UserCouponStatusUnused {
		t.Fatalf("refund state incorrect: %+v %+v %+v", o, p, uc)
	}
	if len(requests) != 2 || requests[0] != requests[1] {
		t.Fatalf("refund not idempotent: %v", requests)
	}
}

func TestShipmentReturnRefundWorkflow(t *testing.T) {
	db := newTestDB(t)
	hub := newNotificationHub()
	o, p := paidLifecycleOrder(t, db, "return-refund")
	requireStatus(t, lifecycleRequest(t, adminShipOrder(db, hub), o.ID, `{"carrier":"顺丰","trackingNo":""}`), 400)
	requireStatus(t, lifecycleRequest(t, adminShipOrder(db, hub), o.ID, `{"carrier":"顺丰","trackingNo":"SF123456"}`), 200)
	requireStatus(t, lifecycleRequest(t, adminShipOrder(db, hub), o.ID, `{"carrier":"顺丰","trackingNo":"SF9999"}`), 400)
	requireStatus(t, lifecycleRequest(t, adminAddLogistics(db), o.ID, `{"text":"快递已揽收"}`), 200)
	db.First(&o, o.ID)
	if o.TrackingNo != "SF123456" || len(o.Logistics) != 2 {
		t.Fatalf("shipment not persisted: %+v", o)
	}
	requireStatus(t, lifecycleRequest(t, applyAftersale(db, hub), o.ID, `{"type":"return_refund","reason":"尺寸不合适"}`), 200)
	payer := &lifecyclePayer{refundFn: func(string, string, string) error { return nil }}
	h := adminAftersale(db, hub, payer)
	requireStatus(t, lifecycleRequest(t, h, o.ID, `{"action":"refund"}`), 400)
	requireStatus(t, lifecycleRequest(t, h, o.ID, `{"action":"approve_return","returnAddress":"上海市测试路1号，收件人王先生"}`), 200)
	requireStatus(t, lifecycleRequest(t, h, o.ID, `{"action":"refund"}`), 400)
	requireStatus(t, lifecycleRequest(t, submitReturn(db), o.ID, `{"carrier":"中通","trackingNo":"ZT888888"}`), 200)
	requireStatus(t, lifecycleRequest(t, h, o.ID, `{"action":"refund","restock":true}`), 200)
	db.First(&p, p.ID)
	db.First(&o, o.ID)
	if p.Stock != 5 || o.Status != model.OrderStatusRefunded {
		t.Fatalf("return not completed: %+v %+v", p, o)
	}
}

func TestRefundWithoutReturnDoesNotRestockShippedGoods(t *testing.T) {
	db := newTestDB(t)
	hub := newNotificationHub()
	o, p := paidLifecycleOrder(t, db, "no-return")
	requireStatus(t, lifecycleRequest(t, adminShipOrder(db, hub), o.ID, `{"carrier":"顺丰","trackingNo":"SF56789"}`), 200)
	requireStatus(t, lifecycleRequest(t, applyAftersale(db, hub), o.ID, `{"type":"refund_only","reason":"商品损坏无需退回"}`), 200)
	payer := &lifecyclePayer{refundFn: func(string, string, string) error { return nil }}
	requireStatus(t, lifecycleRequest(t, adminAftersale(db, hub, payer), o.ID, `{"action":"refund","restock":true}`), 200)
	db.First(&p, p.ID)
	if p.Stock != 3 || p.Sales != 0 {
		t.Fatalf("unreturned goods were restocked: %+v", p)
	}
}

func TestCancelAndLatePayment(t *testing.T) {
	db := newTestDB(t)
	p := model.Product{Name: "late", Stock: 5}
	db.Create(&p)
	o := model.Order{OrderNo: "late", Status: model.OrderStatusPending, PaymentStarted: true, Amount: 1000, Items: []model.OrderItem{{ProductID: p.ID, Quantity: 1}}}
	db.Create(&o)
	payer := &lifecyclePayer{closeFn: func() error { return errors.New("close failed") }}
	if _, err := cancelPendingOrder(context.Background(), db, payer, o.ID, nil); err == nil {
		t.Fatal("failed close accepted")
	}
	db.First(&o, o.ID)
	if o.Status != model.OrderStatusPending {
		t.Fatal("failed close cancelled order")
	}
	payer.closeFn = nil
	if _, err := cancelPendingOrder(context.Background(), db, payer, o.ID, nil); err != nil {
		t.Fatal(err)
	}
	c, w := payStatusCtx(t, o.ID)
	payOrderStatus(db, payer)(c)
	if payStatusOf(t, w) != "closed" {
		t.Fatal("cancelled order reported as paid")
	}
	if err := markPaid(db, o.ID, "late-trade"); err != nil {
		t.Fatal(err)
	}
	db.First(&o, o.ID)
	db.First(&p, p.ID)
	if o.Status != model.OrderStatusAftersale || o.TradeNo != "late-trade" || o.AftersaleStatus != "requested" || p.Stock != 5 {
		t.Fatalf("late payment lost: %+v", o)
	}
}

func TestMarkPaidRollsBackDatabaseFailure(t *testing.T) {
	db := newTestDB(t)
	p := model.Product{Name: "rollback", Stock: 5}
	db.Create(&p)
	o := model.Order{OrderNo: "rollback", Status: model.OrderStatusPending, Items: []model.OrderItem{{ProductID: p.ID, Quantity: 2}}}
	db.Create(&o)
	if err := db.Exec(`CREATE TRIGGER fail_sales BEFORE UPDATE OF sales ON products BEGIN SELECT RAISE(ABORT, 'test sales failure'); END;`).Error; err != nil {
		t.Fatal(err)
	}
	if err := markPaid(db, o.ID, "trade"); err == nil {
		t.Fatal("expected database failure")
	}
	db.First(&o, o.ID)
	db.First(&p, p.ID)
	if o.Status != model.OrderStatusPending || p.Stock != 5 {
		t.Fatalf("partial payment committed: %+v %+v", o, p)
	}
}

func TestOrderLifecycleMigrationIsRepeatable(t *testing.T) {
	db := newTestDB(t)
	now := time.Now()
	old := model.Order{OrderNo: "legacy", Status: model.OrderStatusPaid, PaidAt: &now}
	db.Create(&old)
	newOrder := model.Order{OrderNo: "new", Status: model.OrderStatusPending, LifecycleVersion: 1}
	db.Create(&newOrder)
	for i := 0; i < 2; i++ {
		if err := migrateOrderLifecycle(db); err != nil {
			t.Fatal(err)
		}
	}
	db.First(&old, old.ID)
	db.First(&newOrder, newOrder.ID)
	if !old.StockDeducted || old.LifecycleVersion != 1 || newOrder.PaymentStarted {
		t.Fatalf("bad migration: %+v %+v", old, newOrder)
	}
}

func TestOrderAndSupportOwnership(t *testing.T) {
	db := newTestDB(t)
	hub := newNotificationHub()
	o := model.Order{OrderNo: "other", UserID: "another-user", Status: model.OrderStatusPaid}
	db.Create(&o)
	requireStatus(t, lifecycleRequest(t, applyAftersale(db, hub), o.ID, `{"type":"refund_only","reason":"申请退款"}`), 404)
	ticket := model.SupportTicket{UserID: "another-user", Subject: "私人咨询", Status: "open"}
	db.Create(&ticket)
	requireStatus(t, lifecycleRequest(t, supportTicket(db, false), ticket.ID, `{}`), 404)
	requireStatus(t, lifecycleRequest(t, replySupport(db, hub, false), ticket.ID, `{"content":"不能越权回复"}`), 404)
	var count int64
	db.Model(&model.SupportMessage{}).Count(&count)
	if count != 0 {
		t.Fatal("unauthorized message stored")
	}
}

func TestSupportConversationAndNotification(t *testing.T) {
	db := newTestDB(t)
	hub := newNotificationHub()
	w := lifecycleRequest(t, createSupport(db), 0, `{"subject":"询问发货","content":"订单何时发货？"}`)
	requireStatus(t, w, 200)
	var ticket model.SupportTicket
	if err := json.Unmarshal(w.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}
	requireStatus(t, lifecycleRequest(t, replySupport(db, hub, true), ticket.ID, `{"content":"今日安排发货"}`), 200)
	w = lifecycleRequest(t, supportTicket(db, false), ticket.ID, `{}`)
	requireStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "今日安排发货") {
		t.Fatal("reply not readable")
	}
	var count int64
	db.Model(&model.Notification{}).Where("type = ?", model.NotificationTypeSupport).Count(&count)
	if count != 1 {
		t.Fatal("missing reply notification")
	}
	requireStatus(t, lifecycleRequest(t, replySupport(db, hub, false), ticket.ID, `{"close":true}`), 200)
	requireStatus(t, lifecycleRequest(t, replySupport(db, hub, true), ticket.ID, `{"content":"不能回复已关闭咨询"}`), 400)
}

func TestAdminCannotForgePaidState(t *testing.T) {
	db := newTestDB(t)
	o := model.Order{OrderNo: "pending", Status: model.OrderStatusPending}
	db.Create(&o)
	requireStatus(t, lifecycleRequest(t, adminUpdateOrderStatus(db, newNotificationHub()), o.ID, `{"status":"paid"}`), 400)
	db.First(&o, o.ID)
	if o.Status != model.OrderStatusPending {
		t.Fatal("payment forged")
	}
}

func TestCreateOrderRequiresSkuAndAggregatesQuantity(t *testing.T) {
	db := newTestDB(t)
	p := model.Product{Name: "sku", Stock: 20, Price: 1000, OnShelf: true}
	db.Create(&p)
	sku := model.ProductSku{ProductID: p.ID, Specs: "红色", Price: 1500, Stock: 3}
	db.Create(&sku)
	w := lifecycleRequest(t, createOrder(db), 0, fmt.Sprintf(`{"items":[{"productId":%d,"quantity":1}]}`, p.ID))
	requireStatus(t, w, 400)
	w = lifecycleRequest(t, createOrder(db), 0, fmt.Sprintf(`{"items":[{"productId":%d,"skuId":%d,"quantity":2},{"productId":%d,"skuId":%d,"quantity":2}]}`, p.ID, sku.ID, p.ID, sku.ID))
	requireStatus(t, w, 400)
	w = lifecycleRequest(t, createOrder(db), 0, fmt.Sprintf(`{"items":[{"productId":%d,"skuId":%d,"quantity":2,"price":1}]}`, p.ID, sku.ID))
	requireStatus(t, w, 200)
	var o model.Order
	if err := json.Unmarshal(w.Body.Bytes(), &o); err != nil {
		t.Fatal(err)
	}
	if o.Amount != 3000 || o.Items[0].Specs != "红色" {
		t.Fatalf("wrong SKU pricing: %+v", o)
	}
}
