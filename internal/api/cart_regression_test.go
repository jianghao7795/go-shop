package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"shop/internal/model"
)

func cartRequest(t *testing.T, handler gin.HandlerFunc, user, method string, body any) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	c, w := cartCtx(t, method, "/api/cart", string(encoded))
	c.Set("JWT_PAYLOAD", jwt.MapClaims{"user": user})
	handler(c)
	return w
}

func savedCart(t *testing.T, db *gorm.DB, user string) cartView {
	t.Helper()
	w := cartRequest(t, getCart(db), user, "GET", nil)
	requireStatus(t, w, 200)
	var view cartView
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	return view
}

func saveCart(t *testing.T, db *gorm.DB, user string, version uint64, rows ...cartInput) *httptest.ResponseRecorder {
	t.Helper()
	if rows == nil {
		rows = []cartInput{}
	}
	return cartRequest(t, putCart(db), user, "PUT", map[string]any{"version": version, "items": rows})
}

func TestCartVersionAndOwnership(t *testing.T) {
	db := newTestDB(t)
	p := model.Product{Name: "A", Price: 1000, Stock: 10}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	requireStatus(t, saveCart(t, db, "alice", 0, cartInput{p.ID, 0, 3}), 200)
	requireStatus(t, saveCart(t, db, "alice", 0, cartInput{p.ID, 0, 1}), 409)
	view := savedCart(t, db, "alice")
	if view.Version != 1 || len(view.Items) != 1 || view.Items[0].Quantity != 3 {
		t.Fatalf("stale write changed cart: %+v", view)
	}
	if view := savedCart(t, db, "bob"); view.Version != 0 || len(view.Items) != 0 {
		t.Fatalf("other user cart leaked: %+v", view)
	}
	requireStatus(t, saveCart(t, db, "bob", 0, cartInput{p.ID, 0, 1}), 200)
	requireStatus(t, saveCart(t, db, "alice", 1), 200)
	if len(savedCart(t, db, "bob").Items) != 1 {
		t.Fatal("clearing alice changed bob")
	}
}

func TestCartRejectsInvalidWritesWithoutDeletingExistingItems(t *testing.T) {
	db := newTestDB(t)
	p := model.Product{Name: "A", Price: 1000, Stock: 5}
	other := model.Product{Name: "B", Price: 2000, Stock: 5}
	db.Create(&p)
	db.Create(&other)
	sku := model.ProductSku{ProductID: other.ID, Specs: "M", Price: 2000, Stock: 3}
	db.Create(&sku)
	requireStatus(t, saveCart(t, db, "alice", 0, cartInput{p.ID, 0, 2}), 200)
	cases := map[string][]cartInput{
		"zero": {{p.ID, 0, 0}}, "negative": {{p.ID, 0, -1}}, "excessive": {{p.ID, 0, 1000}},
		"stock": {{p.ID, 0, 6}}, "missing product": {{99999, 0, 1}}, "negative sku": {{p.ID, -1, 1}},
		"foreign sku": {{p.ID, sku.ID, 1}}, "missing sku": {{other.ID, 0, 1}},
		"duplicate": {{p.ID, 0, 1}, {p.ID, 0, 1}},
	}
	for name, rows := range cases {
		t.Run(name, func(t *testing.T) {
			requireStatus(t, saveCart(t, db, "alice", 1, rows...), 400)
			view := savedCart(t, db, "alice")
			if view.Version != 1 || len(view.Items) != 1 || view.Items[0].Quantity != 2 {
				t.Fatalf("invalid write changed cart: %+v", view)
			}
		})
	}
	requireStatus(t, cartRequest(t, putCart(db), "alice", "PUT", map[string]any{"items": []cartInput{}}), 400)
}

func TestCartRefreshReportsPricesStockAndDeletedSpecifications(t *testing.T) {
	db := newTestDB(t)
	p := model.Product{Name: "A", Price: 1000, Stock: 10}
	db.Create(&p)
	sku := model.ProductSku{ProductID: p.ID, Specs: "Red", Price: 1200, Stock: 5}
	db.Create(&sku)
	requireStatus(t, saveCart(t, db, "alice", 0, cartInput{p.ID, sku.ID, 3}), 200)
	db.Model(&sku).Updates(map[string]any{"price": 1500, "stock": 2})
	v := savedCart(t, db, "alice").Items[0]
	if v.Price != 1500 || v.Stock != 2 || v.Available || v.Reason == "" {
		t.Fatalf("stale metadata: %+v", v)
	}
	// Keep invalid rows visible while allowing unrelated edits and removal.
	requireStatus(t, saveCart(t, db, "alice", 1, cartInput{p.ID, sku.ID, 3}), 200)
	db.Delete(&sku)
	v = savedCart(t, db, "alice").Items[0]
	if v.Available || v.Specs != "Red" || v.Reason != "商品规格已失效" {
		t.Fatalf("missing invalid SKU status: %+v", v)
	}
	db.Delete(&p)
	v = savedCart(t, db, "alice").Items[0]
	if v.Name != "A" || v.Reason != "商品已删除" {
		t.Fatalf("missing deleted product metadata: %+v", v)
	}
	requireStatus(t, saveCart(t, db, "alice", 2), 200)
}

func checkoutFixture(t *testing.T) (*gorm.DB, cartCheckoutInput, model.Product) {
	t.Helper()
	db := newTestDB(t)
	p := model.Product{Name: "A", Price: 1000, Stock: 10}
	db.Create(&p)
	addr := model.Address{UserID: "alice", Name: "Alice", Phone: "13800000000", Region: "上海", Detail: "1号"}
	db.Create(&addr)
	requireStatus(t, saveCart(t, db, "alice", 0, cartInput{p.ID, 0, 4}), 200)
	version := uint64(1)
	amount := model.Price(2000)
	return db, cartCheckoutInput{RequestID: "checkout-test", Version: &version, AddressID: addr.ID, ExpectedAmount: &amount, Items: []model.OrderItem{{ProductID: p.ID, Quantity: 2, Price: p.Price}}}, p
}

func TestCartCheckoutRetryConsumesOnlySubmittedQuantityOnce(t *testing.T) {
	db, req, p := checkoutFixture(t)
	var firstID uint
	for i := 0; i < 2; i++ {
		w := cartRequest(t, checkoutCart(db), "alice", "POST", req)
		requireStatus(t, w, 200)
		var response struct {
			Order model.Order
			Cart  cartView
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Order.Amount != 2000 || response.Order.Receiver != "Alice" || response.Cart.Version != 2 || len(response.Cart.Items) != 1 || response.Cart.Items[0].Quantity != 2 {
			t.Fatalf("bad checkout: %s", w.Body.String())
		}
		if i == 0 {
			firstID = response.Order.ID
			// A retry must return the receipt even after price/stock changes.
			db.Model(&p).Updates(map[string]any{"price": 2000, "stock": 1})
		} else if response.Order.ID != firstID {
			t.Fatal("retry created another order")
		}
	}
	var count int64
	db.Model(&model.Order{}).Count(&count)
	if count != 1 {
		t.Fatalf("orders=%d", count)
	}
	req.Items[0].Quantity = 1
	requireStatus(t, cartRequest(t, checkoutCart(db), "alice", "POST", req), 409)
	// A delayed full-cart write cannot put purchased quantities back.
	requireStatus(t, saveCart(t, db, "alice", 1, cartInput{p.ID, 0, 4}), 409)
}

func TestCartCheckoutFailuresRollbackCartAndVersion(t *testing.T) {
	for _, name := range []string{"price", "amount", "stock", "address", "version", "insert failure", "receipt failure", "coupon"} {
		t.Run(name, func(t *testing.T) {
			db, req, p := checkoutFixture(t)
			status := 400
			switch name {
			case "price":
				db.Model(&p).Update("price", 1500)
				status = 409
			case "amount":
				*req.ExpectedAmount = 1
				status = 409
			case "stock":
				db.Model(&p).Update("stock", 1)
			case "address":
				db.Model(&model.Address{}).Where("id = ?", req.AddressID).Update("user_id", "bob")
			case "version":
				*req.Version = 0
				status = 409
			case "insert failure":
				if err := db.Exec("CREATE TRIGGER fail_checkout BEFORE INSERT ON shop_orders BEGIN SELECT RAISE(ABORT, 'injected failure'); END").Error; err != nil {
					t.Fatal(err)
				}
				status = 500
			case "receipt failure":
				if err := db.Exec("CREATE TRIGGER fail_receipt BEFORE INSERT ON shop_checkout_requests BEGIN SELECT RAISE(ABORT, 'injected failure'); END").Error; err != nil {
					t.Fatal(err)
				}
				status = 500
			case "coupon":
				coupon := model.Coupon{Title: "Threshold", Amount: 500, MinAmount: 10000, StartAt: time.Now().Add(-time.Hour), EndAt: time.Now().Add(time.Hour)}
				db.Create(&coupon)
				uc := model.UserCoupon{UserID: "alice", CouponID: coupon.ID, Status: model.UserCouponStatusUnused}
				db.Create(&uc)
				req.UserCouponID = uc.ID
			}
			requireStatus(t, cartRequest(t, checkoutCart(db), "alice", "POST", req), status)
			view := savedCart(t, db, "alice")
			if view.Version != 1 || len(view.Items) != 1 || view.Items[0].Quantity != 4 {
				t.Fatalf("failed checkout consumed cart: %+v", view)
			}
			var orders, receipts int64
			db.Model(&model.Order{}).Count(&orders)
			db.Model(&model.CheckoutRequest{}).Count(&receipts)
			if orders != 0 || receipts != 0 {
				t.Fatalf("partial checkout persisted: orders=%d receipts=%d", orders, receipts)
			}
		})
	}
}

func TestCartMigrationDeduplicatesBeforeUniqueIndex(t *testing.T) {
	db := newTestDB(t)
	if err := db.Migrator().DropIndex(&model.CartItem{}, "idx_cart_line"); err != nil {
		t.Fatal(err)
	}
	for _, quantity := range []int{2, 4} {
		if err := db.Create(&model.CartItem{UserID: "alice", ProductID: 1, Quantity: quantity}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := migrateCartLines(db); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.AutoMigrate(&model.CartItem{}); err != nil {
		t.Fatal(err)
	}
	var rows []model.CartItem
	db.Find(&rows)
	if len(rows) != 1 || rows[0].Quantity != 4 {
		t.Fatal(fmt.Sprintf("migration changed intended quantities: %+v", rows))
	}
	if err := db.Create(&model.CartItem{UserID: "alice", ProductID: 1, Quantity: 1}).Error; err == nil {
		t.Fatal("duplicate row accepted")
	}
}

func TestCartDatabaseFailureIsNotAnEmptySuccess(t *testing.T) {
	db := newTestDB(t)
	if err := db.Migrator().DropTable(&model.CartItem{}); err != nil {
		t.Fatal(err)
	}
	requireStatus(t, cartRequest(t, getCart(db), "alice", "GET", nil), 500)
}

func TestCartCheckoutConcurrentRetriesCreateOneOrder(t *testing.T) {
	db, req, _ := checkoutFixture(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	// SQLite's in-memory database belongs to one connection. Real requests still
	// race at the handler boundary and both must resolve to the committed receipt.
	sqlDB.SetMaxOpenConns(1)
	encoded, _ := json.Marshal(req)
	contexts := make([]*gin.Context, 2)
	writers := make([]*httptest.ResponseRecorder, 2)
	for i := range contexts {
		contexts[i], writers[i] = cartCtx(t, "POST", "/api/cart/checkout", string(encoded))
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, c := range contexts {
		wg.Go(func() { <-start; checkoutCart(db)(c) })
	}
	close(start)
	wg.Wait()
	for _, w := range writers {
		requireStatus(t, w, 200)
	}
	var count int64
	db.Model(&model.Order{}).Count(&count)
	if count != 1 {
		t.Fatalf("concurrent retry created %d orders", count)
	}
}
