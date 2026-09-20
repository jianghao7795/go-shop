package model

import (
	"time"

	"gorm.io/gorm"
)

// 订单状态。
const (
	OrderStatusPending   = "pending"   // 待付款
	OrderStatusShipped   = "shipped"   // 待收货
	OrderStatusCompleted = "completed" // 待评价
	OrderStatusAftersale = "aftersale" // 售后
	OrderStatusFinished  = "finished"  // 已完成
)

// OrderItem 是订单中的单个商品项。
type OrderItem struct {
	ProductID int     `json:"productId"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
	Emoji     string  `json:"emoji"`
	Color     string  `json:"color"`
}

// Order 是用户的订单。
type Order struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	OrderNo   string         `json:"orderNo"`
	UserID    string         `json:"userId" gorm:"index;size:64"`
	Status    string         `json:"status" gorm:"size:20"`
	Amount    float64        `json:"amount"`
	Receiver  string         `json:"receiver" gorm:"size:64"`
	Phone     string         `json:"phone" gorm:"size:20"`
	Region    string         `json:"region" gorm:"size:128"`
	Detail    string         `json:"detail" gorm:"size:255"`
	Items     []OrderItem    `json:"items" gorm:"type:text;serializer:json"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-"`
}

// TableName 使用独立的 shop_orders 表。
func (Order) TableName() string { return "shop_orders" }
