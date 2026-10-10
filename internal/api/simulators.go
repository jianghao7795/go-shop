package api

import (
	"os"
	"time"

	"gorm.io/gorm"

	"shop/internal/logx"
	"shop/internal/model"
)

// simulatorsEnabled 返回是否启用后台模拟器（自动发货/营销推送）。默认关闭，仅演示环境开启。
func simulatorsEnabled() bool {
	return os.Getenv("ENABLE_SIMULATORS") == "true"
}

// startSimulators 启动两个后台模拟器，用于演示服务器主动推送：
// 1) 模拟商家定时发货：把最早的已付款订单改为待收货；
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
				if len(model.Coupons) > 0 {
					pushCouponToOnline(db, hub, model.Coupons[0])
				}
			}
		}
	}()
}

func shipNextOrder(db *gorm.DB, hub *notificationHub) {
	var order model.Order
	if db.Where("status = ?", model.OrderStatusPaid).Order("id ASC").First(&order).Error != nil {
		return // 没有待发货订单，跳过
	}
	db.Model(&model.Order{}).Where("id = ?", order.ID).Update("status", model.OrderStatusShipped)
	notify(db, hub, order.UserID, model.NotificationTypeOrder, "订单已发货",
		"您的订单 "+order.OrderNo+" 已发货，请注意查收", order.OrderNo)
	logx.L().Infof("simulated merchant shipped order %s", order.OrderNo)
}

func pushCouponToOnline(db *gorm.DB, hub *notificationHub, cp model.Coupon) {
	for _, userID := range hub.onlineUsers() {
		notify(db, hub, userID, model.NotificationTypeCoupon, cp.Title+"已到账",
			cp.Title+"（"+cp.Condition+"）已发放到您的账户", "")
	}
}
