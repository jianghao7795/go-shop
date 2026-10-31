// cart-migrate inspects or upgrades the existing MySQL cart schema without
// starting the app, seeding data, or running unrelated historical migrations.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"shop/internal/api"
	"shop/internal/database"
	"shop/internal/model"
)

var apply = flag.Bool("apply", false, "Apply the cart schema migration; back up existing cart rows first")

type tableStatus struct {
	Name   string `json:"name"`
	Exists bool   `json:"exists"`
	Rows   int64  `json:"rows"`
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := driver.ParseDSN(database.DSN())
	if err != nil {
		return fmt.Errorf("invalid database configuration")
	}
	cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = 5*time.Second, 30*time.Second, 30*time.Second
	db, err := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("connect to MySQL %s/%s: %w", cfg.Addr, cfg.DBName, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	// Advisory locks are connection scoped. Keep inspection and DDL on one
	// connection so the migration lock cannot accidentally be released early.
	sqlDB.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db = db.WithContext(ctx)
	var version string
	if err := db.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
		return err
	}
	backup := ""
	if *apply {
		lockName := fmt.Sprintf("shop-cart-migration-%x", sha256.Sum256([]byte(cfg.DBName)))[:60]
		var acquired int
		if err := db.Raw("SELECT GET_LOCK(?, 10)", lockName).Scan(&acquired).Error; err != nil {
			return err
		}
		if acquired != 1 {
			return fmt.Errorf("another cart migration is running")
		}
		defer db.WithContext(context.Background()).Exec("SELECT RELEASE_LOCK(?)", lockName)
		if db.Migrator().HasTable(&model.CartItem{}) {
			var count int64
			if err := db.Model(&model.CartItem{}).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				// Names are generated locally, never interpolated from external input.
				backup = "shop_cart_backup_" + strconv.FormatInt(time.Now().UnixNano(), 10)
				if err := db.Exec("CREATE TABLE `" + backup + "` LIKE `shop_cart_items`").Error; err != nil {
					return err
				}
				if err := db.Exec("INSERT INTO `" + backup + "` SELECT * FROM `shop_cart_items`").Error; err != nil {
					return err
				}
			}
		}
		if err := api.MigrateCart(db); err != nil {
			return fmt.Errorf("cart migration failed (backup=%q): %w", backup, err)
		}
	}
	tables := []tableStatus{}
	for _, name := range []string{"products", "shop_product_skus", "shop_cart_items", "shop_cart_states", "shop_checkout_requests", "shop_orders"} {
		status := tableStatus{Name: name, Exists: db.Migrator().HasTable(name)}
		if status.Exists {
			if err := db.Table(name).Count(&status.Rows).Error; err != nil {
				return err
			}
		}
		tables = append(tables, status)
	}
	var duplicates, invalid int64
	if db.Migrator().HasTable(&model.CartItem{}) {
		if err := db.Raw("SELECT COUNT(*) FROM (SELECT user_id, product_id, COALESCE(sku_id, 0) FROM shop_cart_items GROUP BY user_id, product_id, COALESCE(sku_id, 0) HAVING COUNT(*) > 1) AS duplicate_lines").Scan(&duplicates).Error; err != nil {
			return err
		}
		if err := db.Table("shop_cart_items").Where("product_id <= 0 OR sku_id < 0 OR quantity <= 0 OR quantity > 999 OR sku_id IS NULL").Count(&invalid).Error; err != nil {
			return err
		}
	}
	result := map[string]any{"target": cfg.Addr + "/" + cfg.DBName, "mysqlVersion": version, "tables": tables,
		"applied": *apply, "backupTable": backup,
		"cartUniqueIndex": db.Migrator().HasIndex(&model.CartItem{}, "idx_cart_line"), "duplicateCartGroups": duplicates, "invalidCartRows": invalid}
	return json.NewEncoder(os.Stdout).Encode(result)
}
