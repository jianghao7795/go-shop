package model

import (
	"time"

	"gorm.io/gorm"
)

// Coupon 是优惠券（持久化到数据库 shop_coupons）。
type Coupon struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Title     string         `json:"title" gorm:"size:64"`
	Amount    int            `json:"amount"`
	MinAmount int            `json:"minAmount"`
	Condition string         `json:"condition" gorm:"size:64"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-"`
}

// TableName 使用独立的 shop_coupons 表。
func (Coupon) TableName() string { return "shop_coupons" }

// Coupons 是内置优惠券种子数据，也用作数据库不可用时的兜底。
var Coupons = []Coupon{
	{Title: "新人专享券", Amount: 10, MinAmount: 99, Condition: "满 99 元可用"},
	{Title: "全场通用券", Amount: 20, MinAmount: 199, Condition: "满 199 元可用"},
	{Title: "数码品类券", Amount: 50, MinAmount: 499, Condition: "满 499 元可用"},
	{Title: "满减优惠券", Amount: 100, MinAmount: 999, Condition: "满 999 元可用"},
}
