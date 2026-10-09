package model

import (
	"time"

	"gorm.io/gorm"
)

// 订单状态。
const (
	OrderStatusPending   = "pending"   // 待付款
	OrderStatusPaid      = "paid"      // 待发货
	OrderStatusShipped   = "shipped"   // 待收货
	OrderStatusCompleted = "completed" // 待评价
	OrderStatusAftersale = "aftersale" // 售后
	OrderStatusFinished  = "finished"  // 已完成
	OrderStatusCancelled = "cancelled" // 已取消
)

// OrderItem 是订单中的单个商品项。
type OrderItem struct {
	ProductID int    `json:"productId"`
	SkuID     int    `json:"skuId"` // 0 = 无 SKU，用商品默认价格/库存
	Specs     string `json:"specs"`
	Name      string `json:"name"`
	Price     Price  `json:"price"`
	Quantity  int    `json:"quantity"`
	Emoji     string `json:"emoji"`
	Color     string `json:"color"`
}

// Order 是用户的订单。
type Order struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	OrderNo      string         `json:"orderNo" gorm:"size:64;uniqueIndex"`
	UserID       string         `json:"userId" gorm:"index;size:64"`
	Status       string         `json:"status" gorm:"size:20"`
	Amount       Price          `json:"amount"`
	UserCouponID uint           `json:"userCouponId"`
	Receiver     string         `json:"receiver" gorm:"size:64"`
	Phone        string         `json:"phone" gorm:"size:20"`
	Region       string         `json:"region" gorm:"size:128"`
	Detail       string         `json:"detail" gorm:"size:255"`
	Items        []OrderItem    `json:"items" gorm:"type:text;serializer:json"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `json:"-"`
	TradeNo      string         `json:"tradeNo" gorm:"size:64"`
	PayMethod    string         `json:"payMethod" gorm:"size:16"`
	PaidAt       *time.Time     `json:"paidAt"`
}

// TableName 使用独立的 shop_orders 表。
func (Order) TableName() string { return "shop_orders" }
