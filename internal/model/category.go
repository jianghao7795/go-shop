package model

// Category 是商品分类的元数据（Key 与商品表的 category 字段对齐）。
type Category struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Icon string `json:"icon"`
	Note string `json:"note"`
}

// Categories 是内置的商品分类列表。
var Categories = []Category{
	{Key: "Digital", Name: "数码电器", Icon: "📱", Note: "手机、耳机、智能设备"},
	{Key: "Home", Name: "家居生活", Icon: "🏠", Note: "居家好物，提升生活品质"},
	{Key: "Fashion", Name: "服饰鞋包", Icon: "👕", Note: "精选潮流穿搭"},
	{Key: "Beauty", Name: "美妆护肤", Icon: "💄", Note: "热门品牌，焕亮好气色"},
	{Key: "Food", Name: "食品生鲜", Icon: "🍊", Note: "新鲜直达，安心好味道"},
}
