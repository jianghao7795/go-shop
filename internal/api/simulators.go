package api

import (
	"log"
	"time"

	"gorm.io/gorm"

	"shop/internal/model"
)

// startSimulators 启动两个后台模拟器，用于演示服务器主动推送：
// 1) 模拟商家定时发货：把最早的待付款订单改为待收货；
// 2) 模拟营销：优惠券发放 10 分钟后向在线用户推送一次通知。
func startSimulators(db *gorm.DB, hub *notificationHub) {
	if db == nil {
		return
	}
	go func() {
		shipTicker := time.NewTicker(20 * time.Second)
		// 优惠券通知只发一次（time.Timer 触发一次后不再触发），不再定时循环。
		couponTimer := time.NewTimer(10 * time.Minute)
		for {
			select {
			case <-shipTicker.C:
				shipNextOrder(db, hub)
			case <-couponTimer.C:
				if len(coupons) > 0 {
					pushCouponToOnline(db, hub, coupons[0])
				}
			}
		}
	}()
}

func shipNextOrder(db *gorm.DB, hub *notificationHub) {
	var order model.Order
	if db.Where("status = ?", model.OrderStatusPending).Order("id ASC").First(&order).Error != nil {
		return // 没有待付款订单，跳过
	}
	db.Model(&model.Order{}).Where("id = ?", order.ID).Update("status", model.OrderStatusShipped)
	notify(db, hub, order.UserID, model.NotificationTypeOrder, "订单已发货",
		"您的订单 "+order.OrderNo+" 已发货，请注意查收", order.OrderNo)
	log.Printf("simulated merchant shipped order %s", order.OrderNo)
}

func pushCouponToOnline(db *gorm.DB, hub *notificationHub, cp coupon) {
	for _, userID := range hub.onlineUsers() {
		notify(db, hub, userID, model.NotificationTypeCoupon, cp.Title+"已到账",
			cp.Title+"（"+cp.Condition+"）已发放到您的账户", "")
	}
}
