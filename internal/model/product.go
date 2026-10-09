package model

import (
	"time"

	"gorm.io/gorm"
)

// Product 是商品目录的持久化模型。
type Product struct {
	ID            int            `json:"id" gorm:"primaryKey"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Detail        string         `json:"detail" gorm:"type:text"`
	Price         Price          `json:"price"`
	OriginalPrice Price          `json:"originalPrice"`
	Sales         int            `json:"sales"`
	Rating        float64        `json:"rating"`
	Stock         int            `json:"stock" gorm:"default:100"`
	Emoji         string         `json:"emoji"`
	Color         string         `json:"color"`
	Category      string         `json:"category"`
	OnShelf       bool           `json:"onShelf" gorm:"default:true"`
	Featured      bool           `json:"featured" gorm:"default:false"`
	Skus          []ProductSku   `json:"skus" gorm:"-"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `json:"-"`
}

// FallbackProducts 是数据库不可用或为空时兜底的商品目录。
var FallbackProducts = []Product{
	{ID: 1, Name: "Wireless Headphones", Description: "Noise canceling and long battery life", Price: 19900, OriginalPrice: 29900, Sales: 2341, Stock: 100, Emoji: "headphones", Color: "#e8efff", Category: "Digital"},
	{ID: 2, Name: "Cloud Thermos", Description: "Lightweight 316L stainless steel", Price: 8900, OriginalPrice: 12900, Sales: 1820, Stock: 100, Emoji: "thermos", Color: "#fff0e6", Category: "Home"},
	{ID: 3, Name: "Knit Cardigan", Description: "Soft and versatile for spring", Price: 15900, OriginalPrice: 23900, Sales: 968, Stock: 100, Emoji: "cardigan", Color: "#f7eaff", Category: "Fashion"},
	{ID: 4, Name: "Brightening Serum", Description: "Hydrating and luminous skin care", Price: 22900, OriginalPrice: 32900, Sales: 1536, Stock: 100, Emoji: "serum", Color: "#fff5d9", Category: "Beauty"},
	{ID: 5, Name: "Nut Gift Box", Description: "Selected nuts for everyday nutrition", Price: 6900, OriginalPrice: 9900, Sales: 3204, Stock: 100, Emoji: "nuts", Color: "#e7f7ec", Category: "Food"},
	{ID: 6, Name: "Desk Night Light", Description: "Three brightness levels", Price: 7900, OriginalPrice: 11900, Sales: 741, Stock: 100, Emoji: "lamp", Color: "#e7f4ff", Category: "Home"},
}
