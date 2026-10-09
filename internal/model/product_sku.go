package model

import (
	"time"

	"gorm.io/gorm"
)

// ProductSku 是商品的一个规格（独立价格与库存）。
type ProductSku struct {
	ID        int            `json:"id" gorm:"primaryKey"`
	ProductID int            `json:"productId" gorm:"index"`
	Specs     string         `json:"specs" gorm:"size:128"` // 规格描述，如 "红色 / M"
	Price     Price          `json:"price"`
	Stock     int            `json:"stock"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-"`
}

// TableName 使用独立的 shop_product_skus 表。
func (ProductSku) TableName() string { return "shop_product_skus" }
