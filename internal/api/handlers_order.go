package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shop/internal/model"
)

// applyCouponDiscount 根据优惠券计算优惠后金额与优惠额；不满足门槛或券为空时优惠额为 0。
func applyCouponDiscount(amount float64, coupon *model.Coupon) (payAmount, discount float64) {
	if coupon == nil {
		return amount, 0
	}
	if amount < float64(coupon.MinAmount) {
		return amount, 0
	}
	discount = float64(coupon.Amount)
	if discount > amount {
		discount = amount
	}
	return amount - discount, discount
}

func createOrder(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		var req struct {
			AddressID uint              `json:"addressId"`
			CouponID  uint              `json:"couponId"`
			Items     []model.OrderItem `json:"items" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "订单商品不能为空"})
			return
		}
		var amount float64
		for _, item := range req.Items {
			amount += item.Price * float64(item.Quantity)
		}
		var coupon model.Coupon
		if req.CouponID > 0 {
			if db.Where("id = ?", req.CouponID).First(&coupon).Error != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": "优惠券不存在"})
				return
			}
			amount, _ = applyCouponDiscount(amount, &coupon)
		}
		var addr model.Address
		if req.AddressID > 0 {
			db.Where("id = ? AND user_id = ?", req.AddressID, currentUser(c)).First(&addr)
		}
		order := model.Order{
			OrderNo:  time.Now().Format("20060102150405") + strconv.FormatInt(time.Now().UnixNano()%1e6, 10),
			UserID:   currentUser(c),
			Status:   model.OrderStatusPending,
			Amount:   amount,
			Receiver: addr.Name,
			Phone:    addr.Phone,
			Region:   addr.Region,
			Detail:   addr.Detail,
			Items:    req.Items,
		}
		if err := db.Create(&order).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "下单失败，请稍后重试"})
			return
		}
		c.JSON(http.StatusOK, order)
	}
}

func listOrders(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		query := db.Where("user_id = ?", currentUser(c))
		if status := c.Query("status"); status != "" {
			query = query.Where("status = ?", status)
		}
		var orders []model.Order
		query.Order("id DESC").Find(&orders)
		c.JSON(http.StatusOK, orders)
	}
}

func updateOrderStatus(db *gorm.DB, hub *notificationHub) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "无效的订单"})
			return
		}
		var req struct {
			Status string `json:"status" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "无效的状态"})
			return
		}
		var order model.Order
		if db.Where("id = ? AND user_id = ?", id, currentUser(c)).First(&order).Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "订单不存在"})
			return
		}
		db.Model(&model.Order{}).Where("id = ? AND user_id = ?", id, currentUser(c)).Update("status", req.Status)

		text := statusText[req.Status]
		if text == "" {
			text = req.Status
		}
		notify(db, hub, currentUser(c), model.NotificationTypeOrder, "订单状态更新",
			"您的订单 "+order.OrderNo+" 已更新为 "+text, order.OrderNo)
		c.JSON(http.StatusOK, gin.H{"message": "已更新"})
	}
}

func getOrder(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "无效的订单"})
			return
		}
		var order model.Order
		if db.Where("id = ? AND user_id = ?", id, currentUser(c)).First(&order).Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "订单不存在"})
			return
		}
		c.JSON(http.StatusOK, order)
	}
}
