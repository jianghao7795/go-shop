package api

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"shop/internal/database"
	"shop/internal/model"
)

// Explicitly opt in to real MySQL. The configured application schema is never
// modified: every test uses a newly created schema and drops only that schema.
func mysqlCartTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if os.Getenv("SHOP_MYSQL_INTEGRATION") != "1" {
		t.Skip("set SHOP_MYSQL_INTEGRATION=1 to test against configured MySQL")
	}
	cfg, err := driver.ParseDSN(database.DSN())
	if err != nil {
		t.Fatal("invalid MySQL configuration")
	}
	applicationSchema := cfg.DBName
	cfg.DBName = ""
	cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = 5*time.Second, 30*time.Second, 30*time.Second
	options := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(mysql.Open(cfg.FormatDSN()), options)
	if err != nil {
		t.Fatal(err)
	}
	adminSQL, err := admin.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { adminSQL.Close() })
	schema := fmt.Sprintf("shop_cart_verify_%d", time.Now().UnixNano())
	if schema == applicationSchema || !strings.HasPrefix(schema, "shop_cart_verify_") {
		t.Fatal("unsafe test schema name")
	}
	if err := admin.Exec("CREATE DATABASE `" + schema + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP DATABASE `" + schema + "`").Error; err != nil {
			t.Errorf("clean up test schema %s: %v", schema, err)
		}
	})
	cfg.DBName = schema
	db, err := gorm.Open(mysql.Open(cfg.FormatDSN()), options)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(5)
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&model.Product{}, &model.ProductSku{}, &model.Order{}, &model.Address{}, &model.UserCoupon{}, &model.Coupon{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestMySQLCartMigrationAndConcurrentCheckout(t *testing.T) {
	db := mysqlCartTestDB(t)
	// Reproduce the old schema, including nullable SKU IDs and duplicate rows.
	if err := db.Exec(`CREATE TABLE shop_cart_items (
		id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
		user_id VARCHAR(64), product_id BIGINT, sku_id BIGINT NULL, quantity BIGINT,
		created_at DATETIME(3), updated_at DATETIME(3)
	) ENGINE=InnoDB`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO shop_cart_items (user_id, product_id, sku_id, quantity) VALUES ('legacy', 1, NULL, 2), ('legacy', 1, 0, 4)").Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := MigrateCart(db); err != nil {
			t.Fatalf("migration run %d: %v", i+1, err)
		}
	}
	var legacy []model.CartItem
	if err := db.Where("user_id = ?", "legacy").Find(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if len(legacy) != 1 || legacy[0].SkuID != 0 || legacy[0].Quantity != 4 {
		t.Fatalf("legacy migration = %+v", legacy)
	}
	if err := db.Create(&model.CartItem{UserID: "legacy", ProductID: 1, Quantity: 1}).Error; err == nil {
		t.Fatal("unique cart index not enforced by MySQL")
	}

	p := model.Product{Name: "MySQL cart verification", Price: 1000, Stock: 10}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	addr := model.Address{UserID: "alice", Name: "Test receiver", Phone: "13800000000", Region: "Test region", Detail: "Test address"}
	if err := db.Create(&addr).Error; err != nil {
		t.Fatal(err)
	}
	requireStatus(t, saveCart(t, db, "alice", 0, cartInput{p.ID, 0, 4}), 200)
	requireStatus(t, saveCart(t, db, "alice", 0, cartInput{p.ID, 0, 1}), 409)
	if len(savedCart(t, db, "bob").Items) != 0 {
		t.Fatal("cart ownership leaked")
	}

	version := uint64(1)
	amount := model.Price(2000)
	req := cartCheckoutInput{RequestID: "mysql-concurrent-retry", Version: &version, AddressID: addr.ID, ExpectedAmount: &amount,
		Items: []model.OrderItem{{ProductID: p.ID, Quantity: 2, Price: p.Price}}}
	encoded, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	firstContext, firstWriter := cartCtx(t, "POST", "/api/cart/checkout", string(encoded))
	secondContext, secondWriter := cartCtx(t, "POST", "/api/cart/checkout", string(encoded))
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() { <-start; checkoutCart(db)(firstContext) })
	wg.Go(func() { <-start; checkoutCart(db)(secondContext) })
	close(start)
	wg.Wait()
	requireStatus(t, firstWriter, 200)
	requireStatus(t, secondWriter, 200)
	var first, second struct{ Order model.Order }
	if err := json.Unmarshal(firstWriter.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(secondWriter.Body.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if first.Order.ID == 0 || first.Order.ID != second.Order.ID {
		t.Fatal("concurrent retries created different orders")
	}
	var orderCount int64
	if err := db.Model(&model.Order{}).Count(&orderCount).Error; err != nil {
		t.Fatal(err)
	}
	if orderCount != 1 {
		t.Fatalf("orders = %d", orderCount)
	}
	view := savedCart(t, db, "alice")
	if view.Version != 2 || len(view.Items) != 1 || view.Items[0].Quantity != 2 {
		t.Fatalf("checkout cart = %+v", view)
	}

	// Invalid address after cart deductions must roll the entire transaction back.
	req.RequestID = "mysql-rollback"
	*req.Version = 2
	req.AddressID = addr.ID + 999
	requireStatus(t, cartRequest(t, checkoutCart(db), "alice", "POST", req), 400)
	view = savedCart(t, db, "alice")
	if view.Version != 2 || len(view.Items) != 1 || view.Items[0].Quantity != 2 {
		t.Fatalf("failed checkout changed cart = %+v", view)
	}
	if err := db.Model(&model.Order{}).Count(&orderCount).Error; err != nil {
		t.Fatal(err)
	}
	if orderCount != 1 {
		t.Fatalf("failed checkout left an order: %d", orderCount)
	}
	t.Log("MySQL legacy migration, unique index, version conflicts, account isolation, concurrent retry and rollback verified in an isolated schema")
}
