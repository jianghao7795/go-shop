package api

import (
	"time"

	"gorm.io/gorm"

	"shop/internal/model"
)

// markPaid 幂等落库：仅当订单仍是待付款时，标记为已支付并写入支付宝交易号。
func markPaid(db *gorm.DB, orderID uint, tradeNo string) error {
	now := time.Now()
	return db.Model(&model.Order{}).
		Where("id = ? AND status = ?", orderID, model.OrderStatusPending).
		Updates(map[string]any{
			"status":     model.OrderStatusPaid,
			"trade_no":   tradeNo,
			"pay_method": "alipay",
			"paid_at":    &now,
		}).Error
}
