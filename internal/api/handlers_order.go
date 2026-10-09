package api

import (
	"crypto/rand"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shop/internal/model"
)

// applyCouponDiscount 根据优惠券计算优惠后金额与优惠额（单位：分）；不满足门槛或券为空时优惠额为 0。
func applyCouponDiscount(amount model.Price, coupon *model.Coupon) (payAmount, discount model.Price) {
	if coupon == nil {
		return amount, 0
	}
	if amount < coupon.MinAmount {
		return amount, 0
	}
	discount = coupon.Amount
	if discount > amount {
		discount = amount
	}
	return amount - discount, discount
}

// couponInPeriod 判断优惠券在 now 时刻是否处于有效期 [StartAt, EndAt] 内（含边界）。
func couponInPeriod(c *model.Coupon, now time.Time) bool {
	if c == nil {
		return false
	}
	return !now.Before(c.StartAt) && !now.After(c.EndAt)
}

// repricedItems 以数据库中的商品信息重建订单项，忽略客户端提交的价格/名称等字段。
// 有 SkuID 时用 SKU 的价格与规格；返回错误信息（空串表示合法）。
func repricedItems(products map[int]model.Product, skus map[int]model.ProductSku, req []model.OrderItem) ([]model.OrderItem, string) {
	items := make([]model.OrderItem, 0, len(req))
	for _, it := range req {
		if it.Quantity <= 0 {
			return nil, "商品数量不合法"
		}
		p, ok := products[it.ProductID]
		if !ok || !p.OnShelf {
			return nil, "商品不存在或已下架"
		}
		item := model.OrderItem{
			ProductID: p.ID,
			Name:      p.Name,
			Price:     p.Price,
			Quantity:  it.Quantity,
			Emoji:     p.Emoji,
			Color:     p.Color,
		}
		if it.SkuID > 0 {
			sku, ok := skus[it.SkuID]
			if !ok || sku.ProductID != p.ID {
				return nil, "商品规格不存在"
			}
			item.SkuID = sku.ID
			item.Specs = sku.Specs
			item.Price = sku.Price
		}
		items = append(items, item)
	}
	return items, ""
}

// allowedUserOrderTransition 返回用户端是否允许从 from 状态流转到 to 状态。
// 用户仅可「取消待付款订单」与「确认收货」；支付、发货等由回调/后台驱动。
func allowedUserOrderTransition(from, to string) bool {
	switch from {
	case model.OrderStatusPending:
		return to == model.OrderStatusCancelled
	case model.OrderStatusPaid:
		return to == model.OrderStatusAftersale
	case model.OrderStatusShipped:
		return to == model.OrderStatusCompleted || to == model.OrderStatusAftersale
	default:
		return false
	}
}

// generateOrderNo 生成订单号：时间戳 + 随机数字后缀，不可预测且低碰撞。
func generateOrderNo() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	var sb strings.Builder
	sb.WriteString(time.Now().Format("20060102150405"))
	for _, x := range b {
		sb.WriteByte('0' + x%10)
	}
	return sb.String()
}

func createOrder(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !dbRequired(c, db) {
			return
		}
		var req struct {
			AddressID    uint              `json:"addressId"`
			UserCouponID uint              `json:"userCouponId"`
			Items        []model.OrderItem `json:"items" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "订单商品不能为空"})
			return
		}
		// 服务端重新计价：按 productId/skuId 查库，忽略客户端提交的价格/名称等字段。
		ids := make([]int, 0, len(req.Items))
		skuIDs := make([]int, 0)
		for _, it := range req.Items {
			ids = append(ids, it.ProductID)
			if it.SkuID > 0 {
				skuIDs = append(skuIDs, it.SkuID)
			}
		}
		var products []model.Product
		db.Where("id IN ?", ids).Find(&products)
		prodMap := make(map[int]model.Product, len(products))
		for _, p := range products {
			prodMap[p.ID] = p
		}
		var skus []model.ProductSku
		if len(skuIDs) > 0 {
			db.Where("id IN ?", skuIDs).Find(&skus)
		}
		skuMap := make(map[int]model.ProductSku, len(skus))
		for _, s := range skus {
			skuMap[s.ID] = s
		}
		items, msg := repricedItems(prodMap, skuMap, req.Items)
		if msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": msg})
			return
		}
		// 下单时软校验库存（支付时才真正扣减）。
		for _, it := range items {
			stock := prodMap[it.ProductID].Stock
			if it.SkuID > 0 {
				stock = skuMap[it.SkuID].Stock
			}
			if stock < it.Quantity {
				c.JSON(http.StatusBadRequest, gin.H{"message": "「" + prodMap[it.ProductID].Name + "」库存不足"})
				return
			}
		}
		var amount model.Price
		for _, item := range items {
			amount += item.Price * model.Price(item.Quantity)
		}
		var userCouponID uint
		var coupon model.Coupon
		if req.UserCouponID > 0 {
			var uc model.UserCoupon
			if db.Where("id = ? AND user_id = ?", req.UserCouponID, currentUser(c)).First(&uc).Error != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": "优惠券不存在"})
				return
			}
			if uc.Status != model.UserCouponStatusUnused {
				c.JSON(http.StatusBadRequest, gin.H{"message": "优惠券已使用"})
				return
			}
			if db.Where("id = ?", uc.CouponID).First(&coupon).Error != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": "优惠券不存在"})
				return
			}
			if !couponInPeriod(&coupon, time.Now()) {
				c.JSON(http.StatusBadRequest, gin.H{"message": "优惠券不在有效期"})
				return
			}
			userCouponID = uc.ID
			amount, _ = applyCouponDiscount(amount, &coupon)
		}
		var addr model.Address
		if req.AddressID > 0 {
			db.Where("id = ? AND user_id = ?", req.AddressID, currentUser(c)).First(&addr)
		}
		order := model.Order{
			OrderNo:      generateOrderNo(),
			UserID:       currentUser(c),
			Status:       model.OrderStatusPending,
			Amount:       amount,
			UserCouponID: userCouponID,
			Receiver:     addr.Name,
			Phone:        addr.Phone,
			Region:       addr.Region,
			Detail:       addr.Detail,
			Items:        items,
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
		if !dbRequired(c, db) {
			return
		}
		page, pageSize := parsePagination(c)
		query := db.Where("user_id = ?", currentUser(c))
		if status := c.Query("status"); status != "" {
			query = query.Where("status = ?", status)
		}
		var total int64
		query.Count(&total)
		orders := make([]model.Order, 0)
		query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&orders)
		c.JSON(http.StatusOK, gin.H{"items": orders, "total": total, "page": page, "pageSize": pageSize})
	}
}

func updateOrderStatus(db *gorm.DB, hub *notificationHub) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !dbRequired(c, db) {
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
		if !allowedUserOrderTransition(order.Status, req.Status) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "非法操作"})
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
		if !dbRequired(c, db) {
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

// payOrder 发起支付宝当面付预下单，返回收款二维码 data URL。
// payDisabled 描述支付不可用的原因（未配置 / 配置错误），供 payer == nil 时区分返回。
func payOrder(db *gorm.DB, payer alipayPayer, payDisabled string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !dbRequired(c, db) {
			return
		}
		if payer == nil {
			msg := payDisabled
			if msg == "" {
				msg = "支付未配置"
			}
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": msg})
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
		if order.Status != model.OrderStatusPending {
			c.JSON(http.StatusBadRequest, gin.H{"message": "订单状态不可支付"})
			return
		}
		qrCode, err := payer.PreCreate(c.Request.Context(), order.OrderNo,
			order.Amount.YuanString(), "订单 "+order.OrderNo)
		if err != nil {
			log.Printf("alipay precreate failed: order %s: %v", order.OrderNo, err)
			c.JSON(http.StatusBadGateway, gin.H{"message": "支付下单失败，请稍后重试"})
			return
		}
		qrImage, err := qrCodeDataURL(qrCode)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "二维码生成失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"qrImage": qrImage, "orderNo": order.OrderNo, "amount": order.Amount})
	}
}

// payOrderStatus 查询支付宝交易状态；已支付则幂等落库，供前端轮询。
func payOrderStatus(db *gorm.DB, payer alipayPayer) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !dbRequired(c, db) {
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
		if order.Status != model.OrderStatusPending {
			// 非待付款（已支付/已发货/售后/已完成等）视为支付流程已结束。
			c.JSON(http.StatusOK, gin.H{"status": "paid"})
			return
		}
		if payer == nil {
			c.JSON(http.StatusOK, gin.H{"status": "pending"})
			return
		}
		status, tradeNo, err := payer.Query(c.Request.Context(), order.OrderNo)
		if err != nil {
			log.Printf("alipay query failed: order %s: %v", order.OrderNo, err)
			c.JSON(http.StatusBadGateway, gin.H{"message": "支付状态查询失败，请稍后重试"})
			return
		}
		switch status {
		case alipayStatusSuccess, alipayStatusFinished:
			if err := markPaid(db, order.ID, tradeNo); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "支付确认失败"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "paid"})
		case alipayStatusClosed:
			c.JSON(http.StatusOK, gin.H{"status": "closed"})
		default: // WAIT_BUYER_PAY 等
			c.JSON(http.StatusOK, gin.H{"status": "pending"})
		}
	}
}
