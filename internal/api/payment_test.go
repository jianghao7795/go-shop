package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
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

func TestLoadAlipayConfigUnset(t *testing.T) {
	t.Setenv("ALIPAY_APP_ID", "")
	t.Setenv("ALIPAY_PRIVATE_KEY", "")
	t.Setenv("ALIPAY_PUBLIC_KEY", "")
	if _, ok := loadAlipayConfig(); ok {
		t.Fatal("expected unconfigured")
	}
}

func TestLoadAlipayConfigSet(t *testing.T) {
	t.Setenv("ALIPAY_APP_ID", "app-1")
	t.Setenv("ALIPAY_PRIVATE_KEY", "pk")
	t.Setenv("ALIPAY_PUBLIC_KEY", "pub")
	t.Setenv("ALIPAY_PRODUCTION", "true")
	t.Setenv("ALIPAY_NOTIFY_URL", "https://example.com/notify")
	cfg, ok := loadAlipayConfig()
	if !ok {
		t.Fatal("expected configured")
	}
	if cfg.appID != "app-1" || cfg.privateKey != "pk" || cfg.publicKey != "pub" || !cfg.production || cfg.notifyURL != "https://example.com/notify" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestNewAlipayPayerUnconfigured(t *testing.T) {
	t.Setenv("ALIPAY_APP_ID", "")
	t.Setenv("ALIPAY_PRIVATE_KEY", "")
	t.Setenv("ALIPAY_PUBLIC_KEY", "")
	payer, err := newAlipayPayer()
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if payer != nil {
		t.Fatal("payer should be nil when unconfigured")
	}
}

func TestNewAlipayPayerInvalidKey(t *testing.T) {
	t.Setenv("ALIPAY_APP_ID", "app-1")
	t.Setenv("ALIPAY_PRIVATE_KEY", "not-a-key")
	t.Setenv("ALIPAY_PUBLIC_KEY", "not-a-key")
	if _, err := newAlipayPayer(); err == nil {
		t.Fatal("expected error for invalid key material")
	}
}

type mockPayer struct {
	preCreate func(ctx context.Context, outTradeNo, totalAmount, subject string) (string, error)
	query     func(ctx context.Context, outTradeNo string) (string, string, error)
	decode    func(ctx context.Context, values url.Values) (string, string, string, error)
}

func (m *mockPayer) PreCreate(ctx context.Context, o, a, s string) (string, error) {
	if m.preCreate == nil {
		return "", fmt.Errorf("unexpected PreCreate")
	}
	return m.preCreate(ctx, o, a, s)
}
func (m *mockPayer) Query(ctx context.Context, o string) (string, string, error) {
	if m.query == nil {
		return "", "", fmt.Errorf("unexpected Query")
	}
	return m.query(ctx, o)
}
func (m *mockPayer) DecodeNotify(ctx context.Context, v url.Values) (string, string, string, error) {
	if m.decode == nil {
		return "", "", "", fmt.Errorf("unexpected DecodeNotify")
	}
	return m.decode(ctx, v)
}

func payOrderCtx(t *testing.T, id uint) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/orders/"+strconv.Itoa(int(id))+"/pay", nil)
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(id))}}
	return c, w
}

func TestPayOrderPrecreateSuccess(t *testing.T) {
	db := newTestDB(t)
	order := model.Order{OrderNo: "O100", UserID: "", Status: model.OrderStatusPending, Amount: 0.1 + 0.2}
	db.Create(&order)

	var gotTotal string
	payer := &mockPayer{
		preCreate: func(ctx context.Context, outTradeNo, totalAmount, subject string) (string, error) {
			gotTotal = totalAmount
			return "https://qr.alipay.com/xxx", nil
		},
	}
	c, w := payOrderCtx(t, order.ID)
	payOrder(db, payer)(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if gotTotal != "0.30" {
		t.Fatalf("totalAmount = %q, want 0.30 (浮点已格式化)", gotTotal)
	}
	var body struct {
		QrImage string `json:"qrImage"`
		OrderNo string `json:"orderNo"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.HasPrefix(body.QrImage, "data:image/png;base64,") {
		t.Fatalf("qrImage 无 data URL 前缀: %q", body.QrImage[:min(len(body.QrImage), 30)])
	}
	if body.OrderNo != "O100" {
		t.Fatalf("orderNo = %q", body.OrderNo)
	}
}

func TestQrCodeDataURLIsPNG(t *testing.T) {
	s, err := qrCodeDataURL("hello")
	if err != nil {
		t.Fatalf("qrCodeDataURL: %v", err)
	}
	prefix := "data:image/png;base64,"
	if !strings.HasPrefix(s, prefix) {
		t.Fatal("missing prefix")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, prefix))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(raw) < 8 || !bytes.Equal(raw[0:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		t.Fatal("不是合法 PNG")
	}
}

func TestPayOrderUnconfigured(t *testing.T) {
	db := newTestDB(t)
	c, w := payOrderCtx(t, 1)
	payOrder(db, nil)(c) // payer == nil → 503
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}

func TestPayOrderNotPending(t *testing.T) {
	db := newTestDB(t)
	order := model.Order{OrderNo: "O101", UserID: "", Status: model.OrderStatusPaid}
	db.Create(&order)
	payer := &mockPayer{preCreate: func(ctx context.Context, o, a, s string) (string, error) { return "x", nil }}
	c, w := payOrderCtx(t, order.ID)
	payOrder(db, payer)(c)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func payStatusCtx(t *testing.T, id uint) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/orders/"+strconv.Itoa(int(id))+"/pay/status", nil)
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(id))}}
	return c, w
}

func payStatusOf(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return body.Status
}

func TestPayOrderStatusQuerySuccess(t *testing.T) {
	db := newTestDB(t)
	order := model.Order{OrderNo: "O200", UserID: "", Status: model.OrderStatusPending}
	db.Create(&order)
	payer := &mockPayer{query: func(ctx context.Context, o string) (string, string, error) {
		return "TRADE_SUCCESS", "trade-200", nil
	}}
	c, w := payStatusCtx(t, order.ID)
	payOrderStatus(db, payer)(c)

	if got := payStatusOf(t, w); got != "paid" {
		t.Fatalf("status = %q, want paid", got)
	}
	var reloaded model.Order
	db.First(&reloaded, order.ID)
	if reloaded.Status != model.OrderStatusPaid || reloaded.TradeNo != "trade-200" {
		t.Fatalf("order not marked paid: %+v", reloaded)
	}
}

func TestPayOrderStatusWaitBuyer(t *testing.T) {
	db := newTestDB(t)
	order := model.Order{OrderNo: "O201", UserID: "", Status: model.OrderStatusPending}
	db.Create(&order)
	payer := &mockPayer{query: func(ctx context.Context, o string) (string, string, error) {
		return "WAIT_BUYER_PAY", "", nil
	}}
	c, w := payStatusCtx(t, order.ID)
	payOrderStatus(db, payer)(c)
	if got := payStatusOf(t, w); got != "pending" {
		t.Fatalf("status = %q, want pending", got)
	}
}

func TestPayOrderStatusClosed(t *testing.T) {
	db := newTestDB(t)
	order := model.Order{OrderNo: "O202", UserID: "", Status: model.OrderStatusPending}
	db.Create(&order)
	payer := &mockPayer{query: func(ctx context.Context, o string) (string, string, error) {
		return "TRADE_CLOSED", "", nil
	}}
	c, w := payStatusCtx(t, order.ID)
	payOrderStatus(db, payer)(c)
	if got := payStatusOf(t, w); got != "closed" {
		t.Fatalf("status = %q, want closed", got)
	}
}

func TestPayOrderStatusAlreadyPaid(t *testing.T) {
	db := newTestDB(t)
	order := model.Order{OrderNo: "O203", UserID: "", Status: model.OrderStatusPaid}
	db.Create(&order)
	// 已 paid 时应直接返回，不调用 query（query 为 nil，调用即报错）。
	payer := &mockPayer{}
	c, w := payStatusCtx(t, order.ID)
	payOrderStatus(db, payer)(c)
	if got := payStatusOf(t, w); got != "paid" {
		t.Fatalf("status = %q, want paid", got)
	}
}

func notifyCtx(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/pay/alipay/notify", nil)
	c.Request.Form = url.Values{}
	return c, w
}

func TestNotifyMarksPaid(t *testing.T) {
	db := newTestDB(t)
	order := model.Order{OrderNo: "O300", UserID: "u1", Status: model.OrderStatusPending}
	db.Create(&order)
	payer := &mockPayer{decode: func(ctx context.Context, v url.Values) (string, string, string, error) {
		return "TRADE_SUCCESS", "trade-300", "O300", nil
	}}
	c, w := notifyCtx(t)
	handleAlipayNotify(db, payer)(c)

	if w.Code != http.StatusOK || w.Body.String() != "success" {
		t.Fatalf("resp = %d %q, want 200 success", w.Code, w.Body.String())
	}
	var got model.Order
	db.First(&got, order.ID)
	if got.Status != model.OrderStatusPaid || got.TradeNo != "trade-300" {
		t.Fatalf("order not paid: %+v", got)
	}
}

func TestNotifyInvalidSignDoesNotMarkPaid(t *testing.T) {
	db := newTestDB(t)
	order := model.Order{OrderNo: "O301", UserID: "u1", Status: model.OrderStatusPending}
	db.Create(&order)
	payer := &mockPayer{decode: func(ctx context.Context, v url.Values) (string, string, string, error) {
		return "", "", "", fmt.Errorf("sign verify failed")
	}}
	c, w := notifyCtx(t)
	handleAlipayNotify(db, payer)(c)

	if w.Code == http.StatusOK && w.Body.String() == "success" {
		t.Fatal("验签失败不应 ack success")
	}
	var got model.Order
	db.First(&got, order.ID)
	if got.Status != model.OrderStatusPending {
		t.Fatalf("验签失败仍被标记支付: %+v", got)
	}
}

func TestNotifyNonSuccessStatus(t *testing.T) {
	db := newTestDB(t)
	order := model.Order{OrderNo: "O302", UserID: "u1", Status: model.OrderStatusPending}
	db.Create(&order)
	payer := &mockPayer{decode: func(ctx context.Context, v url.Values) (string, string, string, error) {
		return "TRADE_CLOSED", "", "O302", nil
	}}
	c, w := notifyCtx(t)
	handleAlipayNotify(db, payer)(c)

	if w.Body.String() != "success" {
		t.Fatal("非成功状态也应 ack 以停止重试")
	}
	var got model.Order
	db.First(&got, order.ID)
	if got.Status != model.OrderStatusPending {
		t.Fatalf("TRADE_CLOSED 不应标记支付: %+v", got)
	}
}

// makeReadOnly 把测试库设为只读，使后续写操作（markPaid 的 UPDATE）失败，用于验证错误传播。
func makeReadOnly(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec("PRAGMA query_only = ON").Error; err != nil {
		t.Fatalf("set read-only: %v", err)
	}
}

func TestPayOrderStatusMarkPaidError(t *testing.T) {
	db := newTestDB(t)
	order := model.Order{OrderNo: "O204", UserID: "", Status: model.OrderStatusPending}
	db.Create(&order)
	makeReadOnly(t, db)
	payer := &mockPayer{query: func(ctx context.Context, o string) (string, string, error) {
		return "TRADE_SUCCESS", "trade-x", nil
	}}
	c, w := payStatusCtx(t, order.ID)
	payOrderStatus(db, payer)(c)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 when markPaid fails (不能谎报 paid)", w.Code)
	}
}

func TestNotifyMarkPaidError(t *testing.T) {
	db := newTestDB(t)
	order := model.Order{OrderNo: "O303", UserID: "u1", Status: model.OrderStatusPending}
	db.Create(&order)
	makeReadOnly(t, db)
	payer := &mockPayer{decode: func(ctx context.Context, v url.Values) (string, string, string, error) {
		return "TRADE_SUCCESS", "trade-303", "O303", nil
	}}
	c, w := notifyCtx(t)
	handleAlipayNotify(db, payer)(c)

	if w.Body.String() == "success" {
		t.Fatal("markPaid 失败不应 ack success（否则支付宝不再重试）")
	}
}

func TestBuildPrecreateSetsNotifyURL(t *testing.T) {
	p := buildPrecreate("O1", "19.90", "subject", "https://example.com/notify")
	if p.OutTradeNo != "O1" || p.TotalAmount != "19.90" || p.Subject != "subject" || p.NotifyURL != "https://example.com/notify" {
		t.Fatalf("param = %+v", p)
	}
}

func TestPayOrderStatusNonPending(t *testing.T) {
	db := newTestDB(t)
	order := model.Order{OrderNo: "O205", UserID: "", Status: model.OrderStatusShipped}
	db.Create(&order)
	payer := &mockPayer{}
	c, w := payStatusCtx(t, order.ID)
	payOrderStatus(db, payer)(c)

	if got := payStatusOf(t, w); got != "paid" {
		t.Fatalf("status = %q, want paid（非待付款视为已完成支付流程）", got)
	}
}
