package model

import (
	"time"

	"gorm.io/gorm"
)

// Category 是商品分类（持久化到数据库 shop_categories）。
type Category struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Key       string         `json:"key" gorm:"size:32;uniqueIndex"`
	Name      string         `json:"name" gorm:"size:64"`
	Icon      string         `json:"icon" gorm:"size:8"`
	Note      string         `json:"note" gorm:"size:128"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-"`
}

// TableName 使用独立的 shop_categories 表。
func (Category) TableName() string { return "shop_categories" }

// Categories 是内置分类种子数据，也用作数据库不可用时的兜底。
var Categories = []Category{
	{Key: "Digital", Name: "数码电器", Icon: "📱", Note: "手机、耳机、智能设备"},
	{Key: "Home", Name: "家居生活", Icon: "🏠", Note: "居家好物，提升生活品质"},
	{Key: "Fashion", Name: "服饰鞋包", Icon: "👕", Note: "精选潮流穿搭"},
	{Key: "Beauty", Name: "美妆护肤", Icon: "💄", Note: "热门品牌，焕亮好气色"},
	{Key: "Food", Name: "食品生鲜", Icon: "🍊", Note: "新鲜直达，安心好味道"},
}
