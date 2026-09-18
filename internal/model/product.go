package model

// Product 是商品目录的持久化模型。
type Product struct {
	ID            int     `json:"id" gorm:"primaryKey"`
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Price         float64 `json:"price"`
	OriginalPrice float64 `json:"originalPrice"`
	Sales         int     `json:"sales"`
	Emoji         string  `json:"emoji"`
	Color         string  `json:"color"`
	Category      string  `json:"category"`
}

// FallbackProducts 是数据库不可用或为空时兜底的商品目录。
var FallbackProducts = []Product{
	{ID: 1, Name: "Wireless Headphones", Description: "Noise canceling and long battery life", Price: 199, OriginalPrice: 299, Sales: 2341, Emoji: "headphones", Color: "#e8efff", Category: "Digital"},
	{ID: 2, Name: "Cloud Thermos", Description: "Lightweight 316L stainless steel", Price: 89, OriginalPrice: 129, Sales: 1820, Emoji: "thermos", Color: "#fff0e6", Category: "Home"},
	{ID: 3, Name: "Knit Cardigan", Description: "Soft and versatile for spring", Price: 159, OriginalPrice: 239, Sales: 968, Emoji: "cardigan", Color: "#f7eaff", Category: "Fashion"},
	{ID: 4, Name: "Brightening Serum", Description: "Hydrating and luminous skin care", Price: 229, OriginalPrice: 329, Sales: 1536, Emoji: "serum", Color: "#fff5d9", Category: "Beauty"},
	{ID: 5, Name: "Nut Gift Box", Description: "Selected nuts for everyday nutrition", Price: 69, OriginalPrice: 99, Sales: 3204, Emoji: "nuts", Color: "#e7f7ec", Category: "Food"},
	{ID: 6, Name: "Desk Night Light", Description: "Three brightness levels", Price: 79, OriginalPrice: 119, Sales: 741, Emoji: "lamp", Color: "#e7f4ff", Category: "Home"},
}
