package model

import (
	"time"

	"gorm.io/gorm"
)

// 用户券状态。
const (
	UserCouponStatusUnused = "unused" // 未使用
	UserCouponStatusUsed   = "used"   // 已使用
)

// UserCoupon 是用户领取的优惠券（含核销状态）。
type UserCoupon struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	UserID      string         `json:"userId" gorm:"index;size:64"`
	CouponID    uint           `json:"couponId" gorm:"index"`
	Status      string         `json:"status" gorm:"size:16"`
	UsedOrderNo string         `json:"usedOrderNo" gorm:"size:64"`
	UsedAt      *time.Time     `json:"usedAt"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `json:"-"`
}

// TableName 使用独立的 shop_user_coupons 表。
func (UserCoupon) TableName() string { return "shop_user_coupons" }
