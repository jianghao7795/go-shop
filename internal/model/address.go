package model

import (
	"time"

	"gorm.io/gorm"
)

// Address 是用户的收货地址。
type Address struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	UserID    string         `json:"userId" gorm:"index;size:64"`
	Name      string         `json:"name"`
	Phone     string         `json:"phone"`
	Region    string         `json:"region"`
	Detail    string         `json:"detail"`
	Tag       string         `json:"tag" gorm:"size:20"`
	IsDefault bool           `json:"isDefault"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-"`
}

// TableName 使用独立的 shop_addresses 表。
func (Address) TableName() string { return "shop_addresses" }
