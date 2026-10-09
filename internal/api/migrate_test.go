package api

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func scalarInt(t *testing.T, db *gorm.DB, sql string) int64 {
	t.Helper()
	var got int64
	if err := db.Raw(sql).Row().Scan(&got); err != nil {
		t.Fatalf("query %q: %v", sql, err)
	}
	return got
}

func TestMigratePricesToCents(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, stmt := range []string{
		"CREATE TABLE products (id integer primary key autoincrement, price real, original_price real)",
		"CREATE TABLE shop_orders (id integer primary key autoincrement, amount real)",
		"CREATE TABLE shop_coupons (id integer primary key autoincrement, amount int, min_amount int)",
		"INSERT INTO products (price, original_price) VALUES (199, 299)",
		"INSERT INTO products (price, original_price) VALUES (19.9, 29.9)",
		"INSERT INTO shop_orders (amount) VALUES (199)",
		"INSERT INTO shop_coupons (amount, min_amount) VALUES (10, 99)",
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("setup %q: %v", stmt, err)
		}
	}

	if err := migratePricesToCents(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	check := func(sql string, want int64) {
		t.Helper()
		if got := scalarInt(t, db, sql); got != want {
			t.Fatalf("%s = %d, want %d", sql, got, want)
		}
	}
	check("SELECT price FROM products WHERE id = 1", 19900)
	check("SELECT original_price FROM products WHERE id = 1", 29900)
	check("SELECT price FROM products WHERE id = 2", 1990) // 19.9 元 → 1990 分
	check("SELECT amount FROM shop_orders WHERE id = 1", 19900)
	check("SELECT amount FROM shop_coupons WHERE id = 1", 1000)
	check("SELECT min_amount FROM shop_coupons WHERE id = 1", 9900)

	// 幂等：再跑一次不应翻倍
	if err := migratePricesToCents(db); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	check("SELECT price FROM products WHERE id = 1", 19900)
}
