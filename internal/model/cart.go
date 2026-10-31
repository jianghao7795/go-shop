package model

import "time"

// CartItem 是用户购物车中的一项（服务端同步）。
type CartItem struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    string    `json:"userId" gorm:"index;size:64;uniqueIndex:idx_cart_line"`
	ProductID int       `json:"productId" gorm:"index;uniqueIndex:idx_cart_line"`
	SkuID     int       `json:"skuId" gorm:"index;not null;default:0;uniqueIndex:idx_cart_line"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName 使用独立的 shop_cart_items 表。
func (CartItem) TableName() string { return "shop_cart_items" }

// CartState serializes cart writes and checkout across devices.
type CartState struct {
	UserID  string `gorm:"primaryKey;size:64"`
	Version uint64
}

func (CartState) TableName() string { return "shop_cart_states" }

// CheckoutRequest makes a retry after a lost response return the original order.
type CheckoutRequest struct {
	UserID      string `gorm:"primaryKey;size:64"`
	RequestID   string `gorm:"primaryKey;size:64"`
	PayloadHash string `gorm:"size:64"`
	OrderID     uint
	CreatedAt   time.Time
}

func (CheckoutRequest) TableName() string { return "shop_checkout_requests" }
