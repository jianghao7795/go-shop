package model

import "time"

// 通知类型。
const (
	NotificationTypeOrder  = "order"  // 订单变动
	NotificationTypeCoupon = "coupon" // 优惠券
)

// Notification 是推送给用户的站内通知。
type Notification struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    string    `json:"userId" gorm:"index;size:64"`
	Type      string    `json:"type" gorm:"size:20"`
	Title     string    `json:"title" gorm:"size:128"`
	Content   string    `json:"content" gorm:"size:255"`
	OrderNo   string    `json:"orderNo" gorm:"size:64"`
	Read      bool      `json:"read" gorm:"default:false"`
	CreatedAt time.Time `json:"createdAt"`
}

// TableName 使用独立的 shop_notifications 表。
func (Notification) TableName() string { return "shop_notifications" }
