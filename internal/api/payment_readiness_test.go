package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"shop/internal/model"
)

func TestEmptyMerchantConfigKeepsPaymentDisabled(t *testing.T) {
	for _, key := range []string{"ALIPAY_APP_ID", "ALIPAY_PRIVATE_KEY", "ALIPAY_PUBLIC_KEY"} {
		t.Setenv(key, "  ")
	}
	t.Setenv("ALIPAY_PRODUCTION", "true")
	payer, err := newAlipayPayer()
	if err != nil || payer != nil {
		t.Fatalf("empty credentials should disable payment: %v", err)
	}
	db := newTestDB(t)
	o := model.Order{OrderNo: "disabled", Status: model.OrderStatusPending, Amount: 1000}
	db.Create(&o)
	c, w := payOrderCtx(t, o.ID)
	payOrder(db, payer, "支付未配置")(c)
	requireStatus(t, w, 503)
	db.First(&o, o.ID)
	if o.Status != model.OrderStatusPending || o.PaymentStarted || o.PaidAt != nil || o.TradeNo != "" {
		t.Fatalf("disabled payment modified order: %+v", o)
	}
}

func TestPaymentResponseUsesOrderDeadline(t *testing.T) {
	db := newTestDB(t)
	created := time.Now().Add(-10 * time.Minute).Truncate(time.Second)
	o := model.Order{OrderNo: "deadline", Status: model.OrderStatusPending, Amount: 1234, CreatedAt: created}
	db.Create(&o)
	payer := &mockPayer{preCreate: func(_ context.Context, no, amount, subject string) (string, error) {
		if no != o.OrderNo || amount != "12.34" {
			t.Fatal("wrong payment data")
		}
		return "https://qr.alipay.com/test", nil
	}}
	c, w := payOrderCtx(t, o.ID)
	payOrder(db, payer, "")(c)
	requireStatus(t, w, 200)
	var result struct {
		ExpiresAt  time.Time `json:"expiresAt"`
		ServerTime time.Time `json:"serverTime"`
		QRImage    string    `json:"qrImage"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.ExpiresAt.Equal(created.Add(paymentWindow)) || result.ServerTime.IsZero() || result.QRImage == "" {
		t.Fatalf("missing deadline metadata: %+v", result)
	}
	db.First(&o, o.ID)
	if !o.PaymentStarted {
		t.Fatal("payment attempt not persisted")
	}
}

func TestTimedOutPrecreateReconcilesWithoutCashier(t *testing.T) {
	db := newTestDB(t)
	hub := newNotificationHub()
	p := model.Product{Name: "reconcile", Stock: 3}
	db.Create(&p)
	o := model.Order{OrderNo: "timeout", Status: model.OrderStatusPending, Amount: 1000, Items: []model.OrderItem{{ProductID: p.ID, Quantity: 1}}}
	db.Create(&o)
	queries := 0
	payer := &mockPayer{
		preCreate: func(context.Context, string, string, string) (string, error) {
			return "", errors.New("gateway response lost")
		},
		query: func(context.Context, string) (string, string, string, error) {
			queries++
			return alipayStatusSuccess, "trade-timeout", "10.00", nil
		},
	}
	c, w := payOrderCtx(t, o.ID)
	payOrder(db, payer, "")(c)
	requireStatus(t, w, 502)
	db.First(&o, o.ID)
	if !o.PaymentStarted || o.Status != model.OrderStatusPending {
		t.Fatal("lost timed-out attempt")
	}
	for i := 0; i < 2; i++ {
		if _, err := processPaymentBatch(context.Background(), db, payer, hub, 0, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	db.First(&o, o.ID)
	db.First(&p, p.ID)
	if o.Status != model.OrderStatusPaid || p.Stock != 2 || queries != 1 {
		t.Fatalf("reconciliation failed or repeated: %+v stock=%d queries=%d", o, p.Stock, queries)
	}
}

func TestPaymentBatchClosesExpiredUnpaidOrdersWithoutCredentials(t *testing.T) {
	db := newTestDB(t)
	now := time.Now()
	expired := model.Order{OrderNo: "expired", Status: model.OrderStatusPending, CreatedAt: now.Add(-paymentWindow - time.Minute)}
	db.Create(&expired)
	fresh := model.Order{OrderNo: "fresh", Status: model.OrderStatusPending, CreatedAt: now}
	db.Create(&fresh)
	if _, err := processPaymentBatch(context.Background(), db, nil, newNotificationHub(), 0, now); err != nil {
		t.Fatal(err)
	}
	db.First(&expired, expired.ID)
	db.First(&fresh, fresh.ID)
	if expired.Status != model.OrderStatusCancelled || fresh.Status != model.OrderStatusPending {
		t.Fatalf("wrong expiration result: %s %s", expired.Status, fresh.Status)
	}
}

func TestPaymentBatchCursorDoesNotStarveLaterOrders(t *testing.T) {
	db := newTestDB(t)
	hub := newNotificationHub()
	orders := make([]model.Order, 101)
	for i := range orders {
		orders[i] = model.Order{OrderNo: fmt.Sprintf("cursor-%d", i), Status: model.OrderStatusPending, PaymentStarted: true, Amount: 100}
	}
	if err := db.Create(&orders).Error; err != nil {
		t.Fatal(err)
	}
	queries := 0
	payer := &mockPayer{query: func(_ context.Context, no string) (string, string, string, error) {
		queries++
		if no == "cursor-100" {
			return alipayStatusSuccess, "last-trade", "1.00", nil
		}
		return "", "", "", errors.New("temporary query failure")
	}}
	cursor, err := processPaymentBatch(context.Background(), db, payer, hub, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if cursor != orders[99].ID || queries != 100 {
		t.Fatalf("wrong first batch: cursor=%d calls=%d", cursor, queries)
	}
	cursor, err = processPaymentBatch(context.Background(), db, payer, hub, cursor, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var last model.Order
	db.First(&last, orders[100].ID)
	if cursor != 0 || last.Status != model.OrderStatusPaid || queries != 101 {
		t.Fatal("old failed payments starved later payment")
	}
}

func TestQueryRejectsMissingOrMismatchedAmount(t *testing.T) {
	for _, amount := range []string{"", "9.90", "19.901", "NaN", "1e2"} {
		t.Run(amount, func(t *testing.T) {
			db := newTestDB(t)
			o := model.Order{OrderNo: "mismatch", Status: model.OrderStatusPending, Amount: 1990}
			db.Create(&o)
			payer := &mockPayer{query: func(context.Context, string) (string, string, string, error) {
				return alipayStatusSuccess, "trade", amount, nil
			}}
			c, w := payStatusCtx(t, o.ID)
			payOrderStatus(db, payer)(c)
			requireStatus(t, w, 502)
			db.First(&o, o.ID)
			if o.Status != model.OrderStatusPending {
				t.Fatal("invalid payment amount settled order")
			}
		})
	}
}

func TestNotifyRequiresAmountAndRetriesDatabaseReadFailure(t *testing.T) {
	db := newTestDB(t)
	o := model.Order{OrderNo: "callback", Status: model.OrderStatusPending, Amount: 1990}
	db.Create(&o)
	amount := ""
	payer := &mockPayer{decode: func(context.Context, url.Values) (string, string, string, string, error) {
		return alipayStatusSuccess, "trade", o.OrderNo, amount, nil
	}}
	c, w := notifyCtx(t)
	handleAlipayNotify(db, payer)(c)
	requireStatus(t, w, 400)
	db.First(&o, o.ID)
	if o.Status != model.OrderStatusPending {
		t.Fatal("empty amount accepted")
	}
	amount = "19.9"
	c, w = notifyCtx(t)
	handleAlipayNotify(db, payer)(c)
	requireStatus(t, w, 200)
	db.First(&o, o.ID)
	if o.Status != model.OrderStatusPaid {
		t.Fatal("equivalent decimal amount rejected")
	}
	if err := db.Migrator().DropTable(&model.Order{}); err != nil {
		t.Fatal(err)
	}
	c, w = notifyCtx(t)
	handleAlipayNotify(db, payer)(c)
	requireStatus(t, w, 500)
	if w.Body.String() == "success" {
		t.Fatal("database read failure incorrectly acknowledged")
	}
}

func TestStockFailureReturnsAftersalePaymentStatus(t *testing.T) {
	db := newTestDB(t)
	p := model.Product{Name: "sold out", Stock: 1}
	db.Create(&p)
	o := model.Order{OrderNo: "stock-failure", Status: model.OrderStatusPending, Amount: 1000, Items: []model.OrderItem{{ProductID: p.ID, Quantity: 2}}}
	db.Create(&o)
	payer := &mockPayer{query: func(context.Context, string) (string, string, string, error) {
		return alipayStatusSuccess, "trade-stock", "10.00", nil
	}}
	c, w := payStatusCtx(t, o.ID)
	payOrderStatus(db, payer)(c)
	requireStatus(t, w, 200)
	if payStatusOf(t, w) != "aftersale" {
		t.Fatalf("stock failure presented as ordinary payment success: %s", w.Body.String())
	}
	db.First(&p, p.ID)
	if p.Stock != 1 {
		t.Fatal("stock deducted on failed fulfillment")
	}
}
