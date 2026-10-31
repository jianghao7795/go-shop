package api

import (
	"time"

	"shop/internal/model"

	"gorm.io/gorm"
)

// MigrateCart upgrades only cart persistence and checkout receipts. It can be
// run independently of the application without seeding or changing orders.
func MigrateCart(db *gorm.DB) error {
	if err := migrateCartLines(db); err != nil {
		return err
	}
	return db.AutoMigrate(&model.CartItem{}, &model.CartState{}, &model.CheckoutRequest{})
}

// Run before adding the cart unique index. Duplicate legacy rows retain the
// greatest quantity rather than multiplying a customer's intended purchase.
func migrateCartLines(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.CartItem{}) || db.Migrator().HasIndex(&model.CartItem{}, "idx_cart_line") {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasColumn(&model.CartItem{}, "sku_id") {
			if err := tx.Model(&model.CartItem{}).Where("sku_id IS NULL").UpdateColumn("sku_id", 0).Error; err != nil {
				return err
			}
		}
		var rows []model.CartItem
		if err := tx.Order("id ASC").Find(&rows).Error; err != nil {
			return err
		}
		type key struct {
			user         string
			product, sku int
		}
		seen := map[key]model.CartItem{}
		for _, row := range rows {
			if row.Quantity <= 0 || row.ProductID <= 0 || row.SkuID < 0 {
				if err := tx.Delete(&row).Error; err != nil {
					return err
				}
				continue
			}
			k := key{row.UserID, row.ProductID, row.SkuID}
			if first, ok := seen[k]; ok {
				first.Quantity = min(max(first.Quantity, row.Quantity), maxCartQuantity)
				if err := tx.Model(&first).Update("quantity", first.Quantity).Error; err != nil {
					return err
				}
				if err := tx.Delete(&row).Error; err != nil {
					return err
				}
				seen[k] = first
			} else {
				if row.Quantity > maxCartQuantity {
					row.Quantity = maxCartQuantity
					if err := tx.Model(&row).Update("quantity", row.Quantity).Error; err != nil {
						return err
					}
				}
				seen[k] = row
			}
		}
		return nil
	})
}

// migrationRecord 记录已执行的一次性数据迁移，用于幂等。
type migrationRecord struct {
	Key       string `gorm:"primaryKey;size:64"`
	CreatedAt time.Time
}

// TableName 使用独立的 shop_migrations 表。
func (migrationRecord) TableName() string { return "shop_migrations" }

// migratePricesToCents 把历史金额数据从「元」迁移到「分」（一次性）。
// 必须在 AutoMigrate 改列类型之前执行：此时价格/金额列仍是 float/int 元。
// 商品表无自定义表名，对应 GORM 默认的 products。
func migratePricesToCents(db *gorm.DB) error {
	if err := db.AutoMigrate(&migrationRecord{}); err != nil {
		return err
	}
	const key = "prices-to-cents-v1"
	var n int64
	// `key` 是 MySQL 保留字，必须用反引号，否则 SQL 语法错误、错误被忽略会导致幂等检查失效、重复迁移。
	if err := db.Model(&migrationRecord{}).Where("`key` = ?", key).Count(&n).Error; err != nil {
		return err
	}
	if n > 0 {
		return nil // 已迁移
	}

	mig := db.Migrator()
	// float 元 → 分，用 ROUND 避免 19.9*100 的浮点误差导致截断成 1989。
	if mig.HasTable("products") {
		if err := db.Exec("UPDATE products SET price = ROUND(price * 100), original_price = ROUND(original_price * 100)").Error; err != nil {
			return err
		}
	}
	if mig.HasTable("shop_orders") {
		if err := db.Exec("UPDATE shop_orders SET amount = ROUND(amount * 100)").Error; err != nil {
			return err
		}
	}
	// 优惠券金额原本是 int 元，直接 ×100。
	if mig.HasTable("shop_coupons") {
		if err := db.Exec("UPDATE shop_coupons SET amount = amount * 100, min_amount = min_amount * 100").Error; err != nil {
			return err
		}
	}
	return db.Create(&migrationRecord{Key: key, CreatedAt: time.Now()}).Error
}
