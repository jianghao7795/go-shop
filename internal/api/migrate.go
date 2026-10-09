package api

import (
	"time"

	"gorm.io/gorm"
)

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
	db.Model(&migrationRecord{}).Where("key = ?", key).Count(&n)
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
