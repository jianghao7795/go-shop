package api

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shop/internal/logx"
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
	discount = min(coupon.Amount, amount)
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
		if it.SkuID <= 0 && len(p.Skus) > 0 {
			return nil, "请选择商品规格"
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
		for _, it := range req.Items {
			ids = append(ids, it.ProductID)
		}
		var products []model.Product
		db.Where("id IN ?", ids).Find(&products)
		prodMap := make(map[int]model.Product, len(products))
		for _, p := range products {
			prodMap[p.ID] = p
		}
		var skus []model.ProductSku
		if err := db.Where("product_id IN ?", ids).Find(&skus).Error; err != nil {
			c.JSON(500, gin.H{"message": "读取商品规格失败"})
			return
		}
		skuMap := make(map[int]model.ProductSku, len(skus))
		for _, s := range skus {
			skuMap[s.ID] = s
			p := prodMap[s.ProductID]
			p.Skus = append(p.Skus, s)
			prodMap[s.ProductID] = p
		}
		items, msg := repricedItems(prodMap, skuMap, req.Items)
		if msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": msg})
			return
		}
		// 下单时软校验库存（支付时才真正扣减）。
		quantities := map[[2]int]int{}
		for _, it := range items {
			key := [2]int{it.ProductID, it.SkuID}
			quantities[key] += it.Quantity
			stock := prodMap[it.ProductID].Stock
			if it.SkuID > 0 {
				stock = skuMap[it.SkuID].Stock
			}
			if stock < quantities[key] {
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
			LifecycleVersion: 1,
			OrderNo:          generateOrderNo(),
			UserID:           currentUser(c),
			Status:           model.OrderStatusPending,
			Amount:           amount,
			UserCouponID:     userCouponID,
			Receiver:         addr.Name,
			Phone:            addr.Phone,
			Region:           addr.Region,
			Detail:           addr.Detail,
			Items:            items,
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
		query := db.Model(&model.Order{}).Where("user_id = ?", currentUser(c))
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

func updateOrderStatus(db *gorm.DB, hub *notificationHub, payers ...alipayPayer) gin.HandlerFunc {
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
		if req.Status == model.OrderStatusAftersale {
			c.JSON(400, gin.H{"message": "请填写售后类型和原因后提交申请"})
			return
		}
		if req.Status == model.OrderStatusCancelled {
			var payer alipayPayer
			if len(payers) > 0 {
				payer = payers[0]
			}
			user := currentUser(c)
			if _, err := cancelPendingOrder(c.Request.Context(), db, payer, uint(id), &user); err != nil {
				c.JSON(409, gin.H{"message": err.Error()})
				return
			}
		} else {
			r := db.Model(&model.Order{}).Where("id = ? AND user_id = ? AND status = ?", id, currentUser(c), order.Status).Update("status", req.Status)
			if r.Error != nil || r.RowsAffected == 0 {
				c.JSON(409, gin.H{"message": "订单状态已变化，请刷新后重试"})
				return
			}
		}

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
		if time.Since(order.CreatedAt) >= paymentWindow {
			c.JSON(400, gin.H{"message": "订单已超过付款期限，请重新下单"})
			return
		}
		// Persist the attempt before contacting the gateway. A remote precreate may
		// succeed even if its response times out; reconciliation must still check it.
		attempt := db.Model(&model.Order{}).Where("id = ? AND status = ?", order.ID, model.OrderStatusPending).Update("payment_started", true)
		if attempt.Error != nil {
			c.JSON(500, gin.H{"message": "支付请求保存失败，请重试"})
			return
		}
		if attempt.RowsAffected == 0 {
			c.JSON(409, gin.H{"message": "订单状态已变化，请刷新"})
			return
		}
		var qrCode string
		_, err = changeOrder(db, order.ID, nil, func(tx *gorm.DB, o *model.Order) error {
			if o.Status != model.OrderStatusPending {
				return fmt.Errorf("订单状态不可支付")
			}
			if time.Since(o.CreatedAt) >= paymentWindow {
				return fmt.Errorf("订单已超过付款期限")
			}
			var err error
			qrCode, err = payer.PreCreate(c.Request.Context(), o.OrderNo, o.Amount.YuanString(), "订单 "+o.OrderNo)
			if err == nil {
				o.PaymentStarted = true
			}
			return err
		})
		if err != nil {
			logx.L().Errorf("alipay precreate failed: order %s: %v", order.OrderNo, err)
			c.JSON(http.StatusBadGateway, gin.H{"message": "支付下单失败，请稍后重试"})
			return
		}
		qrImage, err := qrCodeDataURL(qrCode)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "二维码生成失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"qrImage": qrImage, "orderNo": order.OrderNo, "amount": order.Amount, "expiresAt": order.CreatedAt.Add(paymentWindow), "serverTime": time.Now()})
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
		if err := syncOrderPayment(c.Request.Context(), db, payer, &order); err != nil {
			logx.L().Errorf("alipay sync failed: order %s: %v", order.OrderNo, err)
			if errors.Is(err, errPaymentQuery) {
				c.JSON(http.StatusBadGateway, gin.H{"message": "支付状态查询失败，请稍后重试"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "支付确认失败，请稍后重试"})
			}
			return
		}
		c.JSON(http.StatusOK, paymentStatusResponse(order))
	}
}
