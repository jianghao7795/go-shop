package model

import "time"

// CartItem 是用户购物车中的一项（服务端同步）。
type CartItem struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    string    `json:"userId" gorm:"index;size:64"`
	ProductID int       `json:"productId" gorm:"index"`
	SkuID     int       `json:"skuId" gorm:"index"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName 使用独立的 shop_cart_items 表。
func (CartItem) TableName() string { return "shop_cart_items" }
