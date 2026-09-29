package model

import (
	"time"

	"gorm.io/gorm"
)

// Review 是商品评价（评分 1-5 星 + 文字）。
type Review struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	UserID    string         `json:"userId" gorm:"index;size:64"`
	ProductID int            `json:"productId" gorm:"index"`
	OrderID   uint           `json:"orderId" gorm:"index"`
	Rating    int            `json:"rating"`
	Content   string         `json:"content" gorm:"size:500"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-"`
}

// TableName 使用独立的 shop_reviews 表。
func (Review) TableName() string { return "shop_reviews" }
