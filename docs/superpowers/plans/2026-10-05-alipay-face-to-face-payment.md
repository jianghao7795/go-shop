# 支付宝沙箱当面付扫码支付 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把当前模拟支付（`payOrder` 直接 `pending → paid`）替换为支付宝沙箱当面付扫码支付：收银台生成二维码，用户手机支付宝扫码付款，后端轮询确认到账后把订单标记为已支付。

**Architecture:** 新增 `internal/api/payment.go` 封装支付宝操作，通过 `alipayPayer` 接口抽象 `smartwalle/alipay/v3` SDK（便于测试注入 mock）。`Order` 表加 3 个支付字段（`trade_no` / `pay_method` / `paid_at`）。支付确认以 `alipay.trade.query` 轮询为主，`notify_url` 异步通知兜底。后端用 `go-qrcode` 生成 base64 PNG，前端 `<img>` 直显。环境变量未配置时优雅降级（503"支付未配置"）。

**Tech Stack:** Go 1.25 + Gin + GORM(MySQL) + `github.com/smartwalle/alipay/v3@v3.2.31` + `github.com/skip2/go-qrcode`（已有）；测试用 `github.com/glebarez/sqlite`（纯 Go 内存库）；前端 Vue 3 + Vant。

**Spec:** `docs/superpowers/specs/2026-09-30-alipay-face-to-face-payment-design.md`

## Global Constraints

- 环境变量：`ALIPAY_APP_ID` / `ALIPAY_PRIVATE_KEY`（PKCS1）/ `ALIPAY_PUBLIC_KEY` / `ALIPAY_PRODUCTION`（可选，`"true"` 才走生产网关，缺省沙箱）。注意：spec 原写 `ALIPAY_GATEWAY`，但 SDK `alipay.New(appId, privateKey, production bool)` 用 bool 决定网关，故改用 `ALIPAY_PRODUCTION`，意图（默认沙箱、env 驱动）不变。
- 任一必需环境变量缺失 → `newAlipayPayer()` 返回 `nil, nil`，对应 handler 返回 `503 "支付未配置"`，App 其余功能不受影响。
- 支付方式字段固定写 `"alipay"`。
- 金额传给支付宝必须 `fmt.Sprintf("%.2f", amount)`（两位小数、正确处理浮点）。
- 订单状态机不变：`pending` → `paid`（支付宝确认后）→ `shipped` → `completed` → `finished`。
- 幂等：`markPaid` 仅当 `status == pending` 时写入，防轮询/通知并发双写。
- 依赖上限：不引入微信/银行卡支付、不建支付流水表、不做退款/对账。

## Review Focus

以下输入/失败模式是 spec 隐含但普通用例测不到的，最容易咬人，已各自落到对应任务里加测试：

1. 轮询与异步通知**同时**确认同一订单 → 只能落一次库、`trade_no` 不被二次覆盖。→ Task 1 幂等测试。
2. 通知携带非成功状态（`TRADE_CLOSED` / 未知状态）→ 不得把订单标记为已支付。→ Task 5 测试。
3. 金额 `%.2f`：`float64` 金额（如 `0.1+0.2`）必须格式化为 `"0.30"` 而非 `"0.30000000000000004"`。→ Task 3 测试（mock 捕获 `totalAmount`）。
4. 二维码返回值必须是合法 base64 PNG（前端直接 `<img :src>`，坏数据会白屏）。→ Task 3 测试（校验 PNG 魔数）。
5. 存在 `pending` 订单时 `shipNextOrder` 不得发货（支付真实化后未付款订单不可被"自动发货"）。→ Task 6 测试。

---

### Task 1: 订单支付字段 + `markPaid` 幂等落库

**Files:**
- Modify: `internal/model/order.go`（`Order` 结构体加字段）
- Create: `internal/api/payment.go`（先只放 `markPaid`）
- Test: `internal/api/payment_test.go`
- Modify: `go.mod` / `go.sum`（新增测试依赖 `github.com/glebarez/sqlite`）

**Interfaces:**
- Consumes: `model.Order` 现有字段。
- Produces: `func markPaid(db *gorm.DB, orderID uint, tradeNo string) error`（后续 Task 3/4/5 复用）；`model.Order` 新增 `TradeNo string` / `PayMethod string` / `PaidAt *time.Time`。

- [ ] **Step 1: 加测试依赖（纯 Go sqlite，避免 Windows 上 cgo 编译问题）**

Run: `go get github.com/glebarez/sqlite@latest`

说明：`gorm.io/driver/sqlite` 依赖 `mattn/go-sqlite3`（需 cgo），Windows 无 C 编译器会失败；`github.com/glebarez/sqlite` 基于 `modernc.org/sqlite` 纯 Go 实现，仅作测试用。

- [ ] **Step 2: 写失败测试**

在 `internal/api/payment_test.go` 写入（此时 `markPaid` 与测试辅助还不存在，编译失败即"红"）：

```go
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
```

- [ ] **Step 3: 运行测试确认失败**

Run: `go test ./internal/api/ -run TestMarkPaid -v`
Expected: FAIL，报 `undefined: markPaid`。

- [ ] **Step 4: 加字段 + 实现 `markPaid`**

`internal/model/order.go` 的 `Order` 结构体末尾（`DeletedAt` 之后）加三行：

```go
	TradeNo   string     `json:"tradeNo" gorm:"size:64"`
	PayMethod string     `json:"payMethod" gorm:"size:16"`
	PaidAt    *time.Time `json:"paidAt"`
```

新建 `internal/api/payment.go`：

```go
package api

import (
	"time"

	"gorm.io/gorm"

	"shop/internal/model"
)

// markPaid 幂等落库：仅当订单仍是待付款时，标记为已支付并写入支付宝交易号。
func markPaid(db *gorm.DB, orderID uint, tradeNo string) error {
	now := time.Now()
	return db.Model(&model.Order{}).
		Where("id = ? AND status = ?", orderID, model.OrderStatusPending).
		Updates(map[string]interface{}{
			"status":     model.OrderStatusPaid,
			"trade_no":   tradeNo,
			"pay_method": "alipay",
			"paid_at":    &now,
		}).Error
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `go test ./internal/api/ -run TestMarkPaid -v`
Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add internal/model/order.go internal/api/payment.go internal/api/payment_test.go go.mod go.sum
git commit -m "feat: 订单支付字段 + markPaid 幂等落库"
```

---

### Task 2: `alipayPayer` 接口 + SDK 适配器 + 配置加载

**Files:**
- Modify: `internal/api/payment.go`（追加接口、适配器、配置加载）
- Test: `internal/api/payment_test.go`（追加）
- Modify: `go.mod` / `go.sum`（新增 `github.com/smartwalle/alipay/v3@v3.2.31`）

**Interfaces:**
- Consumes: Task 1 的 `markPaid`（本任务不调用，但同文件）。
- Produces:
  - `type alipayPayer interface { PreCreate(ctx, outTradeNo, totalAmount, subject string) (qrCode string, err error); Query(ctx, outTradeNo string) (status, tradeNo string, err error); DecodeNotify(ctx, values url.Values) (status, tradeNo, outTradeNo string, err error) }`
  - `func newAlipayPayer() (alipayPayer, error)` — 未配置返回 `(nil, nil)`；配置了但构建失败返回 `(nil, err)`。
  - 状态常量 `alipayStatusSuccess` / `alipayStatusFinished` / `alipayStatusWaitBuyerPay` / `alipayStatusClosed`。

- [ ] **Step 1: 加 SDK 依赖**

Run: `go get github.com/smartwalle/alipay/v3@v3.2.31`

- [ ] **Step 2: 写失败测试（配置加载）**

追加到 `internal/api/payment_test.go`：

```go
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
	cfg, ok := loadAlipayConfig()
	if !ok {
		t.Fatal("expected configured")
	}
	if cfg.appID != "app-1" || cfg.privateKey != "pk" || cfg.publicKey != "pub" || !cfg.production {
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
```

- [ ] **Step 3: 运行确认失败**

Run: `go test ./internal/api/ -run 'TestLoadAlipayConfig|TestNewAlipayPayer' -v`
Expected: FAIL（`undefined: loadAlipayConfig` 等）。

- [ ] **Step 4: 实现接口、适配器、配置加载**

追加到 `internal/api/payment.go`：

```go
import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/smartwalle/alipay/v3"
)

// 支付宝交易状态（与 SDK 的 TradeStatus 字符串一一对应）。
const (
	alipayStatusSuccess      = "TRADE_SUCCESS"
	alipayStatusFinished     = "TRADE_FINISHED"
	alipayStatusWaitBuyerPay = "WAIT_BUYER_PAY"
	alipayStatusClosed       = "TRADE_CLOSED"
)

// alipayPayer 抽象支付宝当面付操作，便于测试注入 mock。
type alipayPayer interface {
	PreCreate(ctx context.Context, outTradeNo, totalAmount, subject string) (qrCode string, err error)
	Query(ctx context.Context, outTradeNo string) (status, tradeNo string, err error)
	DecodeNotify(ctx context.Context, values url.Values) (status, tradeNo, outTradeNo string, err error)
}

// alipaySDK 是 alipayPayer 的 SDK 实现，包装 *alipay.Client。
type alipaySDK struct {
	client *alipay.Client
}

func (a *alipaySDK) PreCreate(ctx context.Context, outTradeNo, totalAmount, subject string) (string, error) {
	p := alipay.TradePreCreate{}
	p.OutTradeNo = outTradeNo
	p.TotalAmount = totalAmount
	p.Subject = subject
	rsp, err := a.client.TradePreCreate(ctx, p)
	if err != nil {
		return "", err
	}
	if !rsp.Code.IsSuccess() {
		return "", fmt.Errorf("支付宝预下单失败: %s", rsp.SubMsg)
	}
	return rsp.QRCode, nil
}

func (a *alipaySDK) Query(ctx context.Context, outTradeNo string) (string, string, error) {
	q := alipay.TradeQuery{OutTradeNo: outTradeNo}
	rsp, err := a.client.TradeQuery(ctx, q)
	if err != nil {
		return "", "", err
	}
	if !rsp.Code.IsSuccess() {
		return "", "", fmt.Errorf("支付宝查询失败: %s", rsp.SubMsg)
	}
	return string(rsp.TradeStatus), rsp.TradeNo, nil
}

func (a *alipaySDK) DecodeNotify(ctx context.Context, values url.Values) (string, string, string, error) {
	noti, err := a.client.DecodeNotification(ctx, values)
	if err != nil {
		return "", "", "", err
	}
	return string(noti.TradeStatus), noti.TradeNo, noti.OutTradeNo, nil
}

// alipayConfig 是从环境变量读到的支付宝配置。
type alipayConfig struct {
	appID      string
	privateKey string
	publicKey  string
	production bool
}

func loadAlipayConfig() (alipayConfig, bool) {
	cfg := alipayConfig{
		appID:      os.Getenv("ALIPAY_APP_ID"),
		privateKey: os.Getenv("ALIPAY_PRIVATE_KEY"),
		publicKey:  os.Getenv("ALIPAY_PUBLIC_KEY"),
		production: os.Getenv("ALIPAY_PRODUCTION") == "true",
	}
	if cfg.appID == "" || cfg.privateKey == "" || cfg.publicKey == "" {
		return alipayConfig{}, false
	}
	return cfg, true
}

// newAlipayPayer 构建支付宝客户端；未配置时返回 (nil, nil)，配置错误返回 (nil, err)。
func newAlipayPayer() (alipayPayer, error) {
	cfg, ok := loadAlipayConfig()
	if !ok {
		return nil, nil
	}
	client, err := alipay.New(cfg.appID, cfg.privateKey, cfg.production)
	if err != nil {
		return nil, err
	}
	if err := client.LoadAliPayPublicKey(cfg.publicKey); err != nil {
		return nil, err
	}
	return &alipaySDK{client: client}, nil
}
```

（`payment.go` 顶部 import 块需同时保留 Task 1 的 `time` / `gorm.io/gorm` / `shop/internal/model`。）

- [ ] **Step 5: 运行确认通过**

Run: `go test ./internal/api/ -run 'TestLoadAlipayConfig|TestNewAlipayPayer' -v`
Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add internal/api/payment.go internal/api/payment_test.go go.mod go.sum
git commit -m "feat: alipayPayer 接口与 SDK 适配器、配置加载"
```

---

### Task 3: 预下单支付接口（替换 `payOrder`）+ 二维码 base64

**Files:**
- Modify: `internal/api/handlers_order.go`（替换 `payOrder` 签名与实现；删除旧模拟逻辑）
- Modify: `internal/api/payment.go`（追加 `qrCodeDataURL`）
- Modify: `internal/api/server.go`（注册/接线 payer）
- Test: `internal/api/payment_test.go`（追加）

**Interfaces:**
- Consumes: `alipayPayer.PreCreate`（Task 2）、`markPaid`（本任务不用）、`currentUser(c)`。
- Produces: `func payOrder(db *gorm.DB, payer alipayPayer) gin.HandlerFunc`、`func qrCodeDataURL(text string) (string, error)`。

- [ ] **Step 1: 写失败测试**

追加到 `internal/api/payment_test.go`（`mockPayer` 定义放本任务；导入 `context` / `encoding/json` / `net/http` / `net/http/httptest` / `net/url` / `strconv` / `strings` / `bytes` / `github.com/gin-gonic/gin`）：

```go
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
		t.Fatalf("missing prefix")
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
```

（`min` 是 Go 1.21+ 内置函数，无需自定义。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/api/ -run 'TestPayOrder|TestQrCodeDataURL' -v`
Expected: FAIL（`undefined: qrCodeDataURL`，`payOrder` 签名不匹配）。

- [ ] **Step 3: 实现 `qrCodeDataURL`**

追加到 `internal/api/payment.go`（import 加 `encoding/base64` 与 `github.com/skip2/go-qrcode`）：

```go
// qrCodeDataURL 把文本生成 base64 编码的 PNG data URL，供前端 <img :src> 直显。
func qrCodeDataURL(text string) (string, error) {
	qr, err := qrcode.New(text, qrcode.Medium)
	if err != nil {
		return "", err
	}
	png, err := qr.PNG(256)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}
```

- [ ] **Step 4: 替换 `payOrder`**

在 `internal/api/handlers_order.go` 删除旧 `payOrder`（第 150-177 行整段），替换为：

```go
// payOrder 发起支付宝当面付预下单，返回收款二维码 data URL。
func payOrder(db *gorm.DB, payer alipayPayer) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		if payer == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "支付未配置"})
			return
		}
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "无效的订单"})
			return
		}
		var order model.Order
		if db.Where("id = ? AND user_id = ?", id, currentUser(c)).First(&order).Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "订单不存在"})
			return
		}
		if order.Status != model.OrderStatusPending {
			c.JSON(http.StatusBadRequest, gin.H{"message": "订单状态不可支付"})
			return
		}
		qrCode, err := payer.PreCreate(c.Request.Context(), order.OrderNo,
			fmt.Sprintf("%.2f", order.Amount), "订单 "+order.OrderNo)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
			return
		}
		qrImage, err := qrCodeDataURL(qrCode)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "二维码生成失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"qrImage": qrImage, "orderNo": order.OrderNo, "amount": order.Amount})
	}
}
```

（`handlers_order.go` import 块需新增 `"fmt"`。）

- [ ] **Step 5: 接线 `server.go`**

在 `internal/api/server.go` 的 `hub := newNotificationHub()` 之后新增：

```go
	payer, err := newAlipayPayer()
	if err != nil {
		log.Printf("alipay init failed (payment disabled): %v", err)
		payer = nil
	}
```

并把 `protected.POST("/orders/:id/pay", payOrder(db))` 改为：

```go
	protected.POST("/orders/:id/pay", payOrder(db, payer))
```

- [ ] **Step 6: 运行确认通过**

Run: `go test ./internal/api/ -run 'TestPayOrder|TestQrCodeDataURL' -v`
Expected: PASS。

- [ ] **Step 7: Commit**

```bash
git add internal/api/handlers_order.go internal/api/payment.go internal/api/server.go internal/api/payment_test.go
git commit -m "feat: 支付宝预下单支付接口 + 二维码 base64"
```

---

### Task 4: 支付状态查询接口（轮询闭环）

**Files:**
- Modify: `internal/api/handlers_order.go`（新增 `payOrderStatus`）
- Modify: `internal/api/server.go`（注册 `GET /orders/:id/pay/status`）
- Test: `internal/api/payment_test.go`（追加）

**Interfaces:**
- Consumes: `alipayPayer.Query`、`markPaid`、`currentUser`。
- Produces: `func payOrderStatus(db *gorm.DB, payer alipayPayer) gin.HandlerFunc`，返回 `{"status": "paid"|"pending"|"closed"}`。

- [ ] **Step 1: 写失败测试**

追加到 `internal/api/payment_test.go`：

```go
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
	// 已 paid 时应直接返回，不调用 query（query 为 nil，调用即 panic/报错）。
	payer := &mockPayer{}
	c, w := payStatusCtx(t, order.ID)
	payOrderStatus(db, payer)(c)
	if got := payStatusOf(t, w); got != "paid" {
		t.Fatalf("status = %q, want paid", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/api/ -run TestPayOrderStatus -v`
Expected: FAIL（`undefined: payOrderStatus`）。

- [ ] **Step 3: 实现 `payOrderStatus`**

追加到 `internal/api/handlers_order.go`：

```go
// payOrderStatus 查询支付宝交易状态；已支付则幂等落库，供前端轮询。
func payOrderStatus(db *gorm.DB, payer alipayPayer) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "无效的订单"})
			return
		}
		var order model.Order
		if db.Where("id = ? AND user_id = ?", id, currentUser(c)).First(&order).Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "订单不存在"})
			return
		}
		if order.Status == model.OrderStatusPaid {
			c.JSON(http.StatusOK, gin.H{"status": "paid"})
			return
		}
		if order.Status != model.OrderStatusPending {
			c.JSON(http.StatusOK, gin.H{"status": order.Status})
			return
		}
		if payer == nil {
			c.JSON(http.StatusOK, gin.H{"status": "pending"})
			return
		}
		status, tradeNo, err := payer.Query(c.Request.Context(), order.OrderNo)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
			return
		}
		switch status {
		case alipayStatusSuccess, alipayStatusFinished:
			_ = markPaid(db, order.ID, tradeNo)
			c.JSON(http.StatusOK, gin.H{"status": "paid"})
		case alipayStatusClosed:
			c.JSON(http.StatusOK, gin.H{"status": "closed"})
		default: // WAIT_BUYER_PAY 等
			c.JSON(http.StatusOK, gin.H{"status": "pending"})
		}
	}
}
```

- [ ] **Step 4: 注册路由**

在 `internal/api/server.go` 的 `protected.POST("/orders/:id/pay", payOrder(db, payer))` 之后加：

```go
	protected.GET("/orders/:id/pay/status", payOrderStatus(db, payer))
```

- [ ] **Step 5: 运行确认通过**

Run: `go test ./internal/api/ -run TestPayOrderStatus -v`
Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add internal/api/handlers_order.go internal/api/server.go internal/api/payment_test.go
git commit -m "feat: 支付宝支付状态查询接口（轮询确认到账）"
```

---

### Task 5: 异步通知处理（验签 + 幂等落库）

**Files:**
- Modify: `internal/api/payment.go`（新增 `handleAlipayNotify`）
- Modify: `internal/api/server.go`（注册公开路由 `POST /api/pay/alipay/notify`，不走 JWT）
- Test: `internal/api/payment_test.go`（追加）

**Interfaces:**
- Consumes: `alipayPayer.DecodeNotify`、`markPaid`。
- Produces: `func handleAlipayNotify(db *gorm.DB, payer alipayPayer) gin.HandlerFunc`。

- [ ] **Step 1: 写失败测试**

追加到 `internal/api/payment_test.go`：

```go
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
		t.Fatalf("非成功状态也应 ack 以停止重试")
	}
	var got model.Order
	db.First(&got, order.ID)
	if got.Status != model.OrderStatusPending {
		t.Fatalf("TRADE_CLOSED 不应标记支付: %+v", got)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/api/ -run TestNotify -v`
Expected: FAIL（`undefined: handleAlipayNotify`）。

- [ ] **Step 3: 实现 `handleAlipayNotify`**

追加到 `internal/api/payment.go`：

```go
// handleAlipayNotify 处理支付宝异步通知：验签通过且交易成功时幂等落库。
func handleAlipayNotify(db *gorm.DB, payer alipayPayer) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil || payer == nil {
			c.String(http.StatusInternalServerError, "failure")
			return
		}
		_ = c.Request.ParseForm()
		status, tradeNo, outTradeNo, err := payer.DecodeNotify(c.Request.Context(), c.Request.Form)
		if err != nil {
			c.String(http.StatusBadRequest, "failure")
			return
		}
		if status == alipayStatusSuccess || status == alipayStatusFinished {
			var order model.Order
			if db.Where("order_no = ?", outTradeNo).First(&order).Error == nil {
				_ = markPaid(db, order.ID, tradeNo)
			}
		}
		c.String(http.StatusOK, "success")
	}
}
```

（`payment.go` import 需加 `net/http` 与 `github.com/gin-gonic/gin`。）

- [ ] **Step 4: 注册公开路由**

在 `internal/api/server.go` 的 `router.POST("/api/register", registerHandler(db))` 之后（公开区，不走 JWT）加：

```go
	router.POST("/api/pay/alipay/notify", handleAlipayNotify(db, payer))
```

- [ ] **Step 5: 运行确认通过**

Run: `go test ./internal/api/ -run TestNotify -v`
Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add internal/api/payment.go internal/api/server.go internal/api/payment_test.go
git commit -m "feat: 支付宝异步通知处理（验签+幂等落库）"
```

---

### Task 6: 修 `shipNextOrder`（只发 `paid`，不碰 `pending`）

**Files:**
- Modify: `internal/api/simulators.go`（`shipNextOrder` 的查询条件）
- Test: `internal/api/simulators_test.go`（新建）

**Interfaces:**
- Consumes: `newTestDB`（Task 1 的测试辅助）、`model.OrderStatusPaid` / `OrderStatusShipped` / `OrderStatusPending`。

- [ ] **Step 1: 写失败测试**

新建 `internal/api/simulators_test.go`：

```go
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
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/api/ -run TestShipNextOrder -v`
Expected: FAIL（`TestShipNextOrderSkipsPending`：当前实现把 pending 发货了）。

- [ ] **Step 3: 修 `shipNextOrder`**

`internal/api/simulators.go` 第 38 行把查询条件从 `status = pending` 改为 `status = paid`：

```go
	if db.Where("status = ?", model.OrderStatusPaid).Order("id ASC").First(&order).Error != nil {
		return // 没有待发货订单，跳过
	}
```

（同步更新上方注释：`把最早的待付款订单改为待收货` → `把最早的已付款订单改为待收货`。）

- [ ] **Step 4: 运行确认通过**

Run: `go test ./internal/api/ -run TestShipNextOrder -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/api/simulators.go internal/api/simulators_test.go
git commit -m "fix: 自动发货只处理已付款订单，不再发货未付款订单"
```

---

### Task 7: 前端重写收银台 `PayView.vue`

**Files:**
- Modify: `frontend/src/views/PayView.vue`（整文件重写）

**Interfaces:**
- Consumes: `http`（`frontend/src/lib/http.ts`）、`POST /api/orders/:id/pay` → `{ qrImage, orderNo, amount }`、`GET /api/orders/:id/pay/status` → `{ status }`。

说明：前端无自动化测试框架（`package.json` 无 vitest/jest），本任务验证方式为 `npm run build` 编译通过 + 手动联调。

- [ ] **Step 1: 重写 `PayView.vue`**

整文件替换为：

```vue
<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { showToast } from "vant";
import http from "../lib/http";

interface Order { id: number; orderNo: string; status: string; amount: number; }

const route = useRoute();
const router = useRouter();
const order = ref<Order | null>(null);
const qrImage = ref("");
const loading = ref(true);
const errorMsg = ref("");
let timer: number | undefined;

async function load() {
  try {
    const res = await http.get("/api/orders/" + route.params.id);
    order.value = res.data;
    if (order.value.status === "paid") {
      showToast("订单已支付");
      router.replace("/orders");
      return;
    }
    if (order.value.status !== "pending") {
      errorMsg.value = "订单状态不可支付";
      loading.value = false;
      return;
    }
    await createPayment();
  } catch {
    order.value = null;
    loading.value = false;
  }
}

async function createPayment() {
  try {
    const res = await http.post("/api/orders/" + route.params.id + "/pay");
    qrImage.value = res.data.qrImage;
    loading.value = false;
    startPolling();
  } catch (err) {
    const e = err as any;
    if (e?.response?.status === 503) errorMsg.value = "支付服务暂未配置";
    else errorMsg.value = e?.response?.data?.message || "支付下单失败";
    loading.value = false;
  }
}

function startPolling() {
  stopPolling();
  timer = window.setInterval(poll, 2000);
}

async function poll() {
  if (!order.value) return;
  try {
    const res = await http.get("/api/orders/" + order.value.id + "/pay/status");
    const status = res.data.status;
    if (status === "paid") {
      stopPolling();
      showToast("支付成功");
      router.replace("/orders");
    } else if (status === "closed") {
      stopPolling();
      errorMsg.value = "支付已关闭，请重新下单";
    }
  } catch { /* 网络抖动忽略，继续轮询 */ }
}

function stopPolling() {
  if (timer !== undefined) {
    clearInterval(timer);
    timer = undefined;
  }
}

onMounted(load);
onUnmounted(stopPolling);
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="收银台" left-arrow @click-left="$router.back()" />
    <van-empty v-if="!order" description="订单不存在" />
    <van-empty v-else-if="errorMsg" :description="errorMsg" />
    <template v-else>
      <div class="pay-amount">
        <div class="pay-label">应付金额</div>
        <div class="pay-money">¥{{ order.amount.toFixed(2) }}</div>
        <div class="pay-order">订单号 {{ order.orderNo }}</div>
      </div>
      <div class="pay-qr">
        <van-loading v-if="loading" size="40px" vertical>生成收款码中…</van-loading>
        <img v-else-if="qrImage" :src="qrImage" alt="支付宝收款码" />
      </div>
      <div class="pay-tip">请使用支付宝扫码付款</div>
      <div class="pay-refresh">
        <van-button block round type="primary" @click="poll">已付款？点此刷新</van-button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.pay-amount {
  margin: 16px;
  padding: 24px 16px;
  background: #fff;
  border-radius: 10px;
  text-align: center;
}
.pay-label { font-size: 13px; color: #969ba5; }
.pay-money { margin: 8px 0; font-size: 32px; font-weight: 700; color: #ff4d67; }
.pay-order { font-size: 12px; color: #969ba5; }
.pay-qr {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 220px;
  margin: 16px;
  background: #fff;
  border-radius: 10px;
}
.pay-qr img { width: 200px; height: 200px; }
.pay-tip { text-align: center; font-size: 13px; color: #969ba5; }
.pay-refresh { margin: 24px 16px; }
</style>
```

- [ ] **Step 2: 编译验证**

Run: `cd frontend && npm run build`
Expected: 构建成功，无 TS 类型错误。

- [ ] **Step 3: Commit**

```bash
git add frontend/src/views/PayView.vue
git commit -m "feat: 收银台改为支付宝当面付扫码 + 轮询确认"
```

---

## 收尾验证（全部完成后）

```bash
go test ./...                 # 全量后端测试通过
cd frontend && npm run build  # 前端编译通过
```

联调实测需支付宝沙箱凭据（`ALIPAY_APP_ID` / `ALIPAY_PRIVATE_KEY` / `ALIPAY_PUBLIC_KEY`），见 spec 的「沙箱实测步骤」。未配置时，下单收银台返回 503"支付服务暂未配置"，其余商城功能不受影响。
