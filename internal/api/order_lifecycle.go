package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smartwalle/alipay/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"shop/internal/logx"
	"shop/internal/model"
)

const paymentWindow = 30 * time.Minute

// migrateOrderLifecycle is transactional and restart-safe. Legacy aftersales have no
// reliable inventory history, so they are refundable but never automatically restocked.
func migrateOrderLifecycle(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Order{}).Where("lifecycle_version = 0 AND paid_at IS NOT NULL AND status IN ?", []string{model.OrderStatusPaid, model.OrderStatusShipped, model.OrderStatusCompleted, model.OrderStatusFinished}).Update("stock_deducted", true).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Order{}).Where("lifecycle_version = 0 AND status = ?", model.OrderStatusPending).Update("payment_started", true).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Order{}).Where("lifecycle_version = 0 AND status = ? AND (aftersale_status IS NULL OR aftersale_status = '')", model.OrderStatusAftersale).Updates(map[string]any{"aftersale_type": "refund_only", "aftersale_status": "requested", "aftersale_reason": "历史售后订单，请核实支付和退货情况"}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Order{}).Where("lifecycle_version = 0").Update("lifecycle_version", 1).Error
	})
}

type alipayCloser interface {
	Close(context.Context, string) error
}
type alipayRefunder interface {
	Refund(context.Context, string, string, string) error
}

func (a *alipaySDK) Close(ctx context.Context, orderNo string) error {
	r, err := a.client.TradeClose(ctx, alipay.TradeClose{OutTradeNo: orderNo})
	if err != nil {
		return err
	}
	if !r.Code.IsSuccess() && r.SubCode != "ACQ.TRADE_NOT_EXIST" {
		return fmt.Errorf("支付宝关单失败: %s", r.SubMsg)
	}
	a.mu.Lock()
	delete(a.qrCache, orderNo)
	a.mu.Unlock()
	return nil
}

func (a *alipaySDK) Refund(ctx context.Context, orderNo, requestNo, amount string) error {
	r, err := a.client.TradeRefund(ctx, alipay.TradeRefund{OutTradeNo: orderNo, OutRequestNo: requestNo, RefundAmount: amount})
	if err != nil {
		return err
	}
	if !r.Code.IsSuccess() {
		return fmt.Errorf("支付宝退款失败: %s", r.SubMsg)
	}
	return nil
}

// changeOrder locks the order while checking and applying a transition.
func changeOrder(db *gorm.DB, id uint, user *string, fn func(*gorm.DB, *model.Order) error) (model.Order, error) {
	var o model.Order
	err := db.Transaction(func(tx *gorm.DB) error {
		q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id)
		if user != nil {
			q = q.Where("user_id = ?", *user)
		}
		if err := q.First(&o).Error; err != nil {
			return err
		}
		if err := fn(tx, &o); err != nil {
			return err
		}
		return tx.Save(&o).Error
	})
	return o, err
}

func lifecycleID(c *gin.Context, db *gorm.DB) (uint, bool) {
	if !dbRequired(c, db) {
		return 0, false
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"message": "无效的订单"})
		return 0, false
	}
	return uint(id), true
}

func orderResult(c *gin.Context, o model.Order, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(404, gin.H{"message": "订单不存在"})
		return
	}
	if err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	c.JSON(200, o)
}

func cancelPendingOrder(ctx context.Context, db *gorm.DB, payer alipayPayer, id uint, user *string) (model.Order, error) {
	o, err := changeOrder(db, id, user, func(tx *gorm.DB, o *model.Order) error {
		if o.Status == model.OrderStatusCancelled {
			return nil
		}
		if o.Status != model.OrderStatusPending {
			return fmt.Errorf("订单状态已变化，无法取消")
		}
		if closer, ok := payer.(alipayCloser); ok {
			// Close even after a precreate timeout: the remote trade may exist although the local flag was not committed.
			if err := closer.Close(ctx, o.OrderNo); err != nil {
				return fmt.Errorf("支付订单尚未关闭，请刷新支付状态后重试")
			}
		} else if o.PaymentStarted {
			return fmt.Errorf("支付关单服务暂不可用，请稍后重试")
		}
		o.Status = model.OrderStatusCancelled
		return nil
	})
	if err != nil && o.Status == model.OrderStatusPending && payer != nil {
		if status, tradeNo, amount, queryErr := payer.Query(ctx, o.OrderNo); queryErr == nil && (status == alipayStatusSuccess || status == alipayStatusFinished) && alipayAmountMatches(amount, o.Amount) {
			if paidErr := markPaid(db, id, tradeNo); paidErr != nil {
				return o, paidErr
			}
			return o, fmt.Errorf("订单已付款，请刷新后申请售后")
		}
	}
	return o, err
}

func startOrderExpiry(db *gorm.DB, payer alipayPayer, hub *notificationHub) {
	if db == nil {
		return
	}
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	var cursor uint
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		next, err := processPaymentBatch(ctx, db, payer, hub, cursor, time.Now())
		cancel()
		cursor = next
		if err != nil {
			logx.L().Warnf("payment reconciliation: %v", err)
		}
	}
}

func applyAftersale(db *gorm.DB, hub *notificationHub) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := lifecycleID(c, db)
		if !ok {
			return
		}
		var req struct {
			Type   string `json:"type"`
			Reason string `json:"reason"`
		}
		if c.ShouldBindJSON(&req) != nil {
			c.JSON(400, gin.H{"message": "请填写售后申请"})
			return
		}
		req.Reason = strings.TrimSpace(req.Reason)
		if (req.Type != "refund_only" && req.Type != "return_refund") || len([]rune(req.Reason)) < 2 || len([]rune(req.Reason)) > 500 {
			c.JSON(400, gin.H{"message": "请选择售后类型并填写 2-500 字原因"})
			return
		}
		user := currentUser(c)
		o, err := changeOrder(db, id, &user, func(tx *gorm.DB, o *model.Order) error {
			switch o.Status {
			case model.OrderStatusPaid, model.OrderStatusShipped, model.OrderStatusCompleted, model.OrderStatusFinished:
			default:
				return fmt.Errorf("当前订单不能申请售后")
			}
			if o.Status == model.OrderStatusPaid && req.Type == "return_refund" {
				return fmt.Errorf("未发货订单请选择仅退款")
			}
			o.AftersalePreviousStatus, o.Status = o.Status, model.OrderStatusAftersale
			o.AftersaleType, o.AftersaleReason, o.AftersaleStatus = req.Type, req.Reason, "requested"
			o.AftersaleNote, o.ReturnAddress, o.ReturnCarrier, o.ReturnTrackingNo = "", "", "", ""
			return nil
		})
		if err == nil {
			notify(db, hub, user, model.NotificationTypeOrder, "售后申请已提交", "订单 "+o.OrderNo+" 的售后申请等待审核", o.OrderNo)
		}
		orderResult(c, o, err)
	}
}

func submitReturn(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := lifecycleID(c, db)
		if !ok {
			return
		}
		var req struct {
			Carrier    string `json:"carrier"`
			TrackingNo string `json:"trackingNo"`
		}
		if c.ShouldBindJSON(&req) != nil || !validShipment(req.Carrier, req.TrackingNo) {
			c.JSON(400, gin.H{"message": "请填写快递公司和有效运单号"})
			return
		}
		user := currentUser(c)
		o, err := changeOrder(db, id, &user, func(tx *gorm.DB, o *model.Order) error {
			if o.Status != model.OrderStatusAftersale || o.AftersaleStatus != "returning" {
				return fmt.Errorf("当前售后不能填写退货运单")
			}
			o.ReturnCarrier, o.ReturnTrackingNo = strings.TrimSpace(req.Carrier), strings.TrimSpace(req.TrackingNo)
			return nil
		})
		orderResult(c, o, err)
	}
}

func adminAftersale(db *gorm.DB, hub *notificationHub, payer alipayPayer) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := lifecycleID(c, db)
		if !ok {
			return
		}
		var req struct {
			Action        string `json:"action"`
			Note          string `json:"note"`
			ReturnAddress string `json:"returnAddress"`
			Restock       bool   `json:"restock"`
		}
		if c.ShouldBindJSON(&req) != nil {
			c.JSON(400, gin.H{"message": "无效的操作"})
			return
		}
		req.Note, req.ReturnAddress = strings.TrimSpace(req.Note), strings.TrimSpace(req.ReturnAddress)
		if len([]rune(req.Note)) > 500 || len([]rune(req.ReturnAddress)) > 500 {
			c.JSON(400, gin.H{"message": "备注或地址不能超过 500 字"})
			return
		}
		o, err := changeOrder(db, id, nil, func(tx *gorm.DB, o *model.Order) error {
			if req.Action == "refund" && o.Status == model.OrderStatusRefunded {
				return nil
			}
			if o.Status != model.OrderStatusAftersale {
				return fmt.Errorf("订单不在售后处理中")
			}
			switch req.Action {
			case "reject":
				if o.AftersaleStatus != "requested" || req.Note == "" {
					return fmt.Errorf("仅可驳回待审核申请，且必须填写原因")
				}
				if o.AftersalePreviousStatus == model.OrderStatusPending || o.AftersalePreviousStatus == model.OrderStatusCancelled || o.AftersalePreviousStatus == "" {
					return fmt.Errorf("支付异常订单需要退款，不能驳回")
				}
				o.Status, o.AftersaleStatus = o.AftersalePreviousStatus, "rejected"
			case "approve_return":
				if o.AftersaleStatus != "requested" || o.AftersaleType != "return_refund" || req.ReturnAddress == "" {
					return fmt.Errorf("请为待审核退货申请填写退货地址")
				}
				o.AftersaleStatus, o.ReturnAddress = "returning", req.ReturnAddress
			case "refund":
				if o.AftersaleStatus == "refunding" {
					return nil
				} // retry with the persisted refund decision
				if o.AftersaleStatus != "requested" && o.AftersaleStatus != "returning" {
					return fmt.Errorf("当前售后状态不能退款")
				}
				if o.AftersaleType == "return_refund" && (o.AftersaleStatus != "returning" || o.ReturnTrackingNo == "") {
					return fmt.Errorf("请先审核退货并等待用户提交退货运单")
				}
				if o.Amount > 0 && (o.TradeNo == "" || o.PayMethod != "alipay") {
					return fmt.Errorf("缺少支付宝支付记录，无法原路退款")
				}
				if o.Amount > 0 {
					if _, ok := payer.(alipayRefunder); !ok {
						return fmt.Errorf("退款服务暂未配置")
					}
				}
				o.RefundRestock = o.AftersalePreviousStatus == model.OrderStatusPaid || (o.AftersaleType == "return_refund" && req.Restock)
				o.AftersaleStatus = "refunding"
			default:
				return fmt.Errorf("无效的售后操作")
			}
			o.AftersaleNote = req.Note
			return nil
		})
		if err != nil {
			orderResult(c, o, err)
			return
		}
		if req.Action == "refund" && o.Status != model.OrderStatusRefunded {
			if o.Amount > 0 {
				r, ok := payer.(alipayRefunder)
				if !ok {
					c.JSON(503, gin.H{"message": "退款服务暂未配置"})
					return
				}
				// Stable request number makes retries safe even after a timeout or a local commit failure.
				if err := r.Refund(c.Request.Context(), o.OrderNo, "refund-"+o.OrderNo, o.Amount.YuanString()); err != nil {
					logx.L().Errorf("refund order %s: %v", o.OrderNo, err)
					c.JSON(502, gin.H{"message": "退款结果尚未确认，请重试；重复提交不会重复退款"})
					return
				}
			}
			o, err = finishRefund(db, id)
		}
		if err == nil {
			notify(db, hub, o.UserID, model.NotificationTypeOrder, "售后进度更新", "订单 "+o.OrderNo+" 的售后已更新，请查看订单详情", o.OrderNo)
		}
		orderResult(c, o, err)
	}
}

func finishRefund(db *gorm.DB, id uint) (model.Order, error) {
	o, err := changeOrder(db, id, nil, func(tx *gorm.DB, o *model.Order) error {
		if o.Status == model.OrderStatusRefunded {
			return nil
		}
		if o.Status != model.OrderStatusAftersale || o.AftersaleStatus != "refunding" {
			return fmt.Errorf("退款状态已变化")
		}
		if o.StockDeducted {
			for _, it := range o.Items {
				if it.ProductID <= 0 || it.Quantity <= 0 {
					continue
				}
				if o.RefundRestock {
					q := tx.Unscoped().Model(&model.Product{}).Where("id = ?", it.ProductID)
					if it.SkuID > 0 {
						q = tx.Unscoped().Model(&model.ProductSku{}).Where("id = ?", it.SkuID)
					}
					if err := q.Update("stock", gorm.Expr("stock + ?", it.Quantity)).Error; err != nil {
						return err
					}
				}
				if err := tx.Unscoped().Model(&model.Product{}).Where("id = ?", it.ProductID).Update("sales", gorm.Expr("CASE WHEN sales >= ? THEN sales - ? ELSE 0 END", it.Quantity, it.Quantity)).Error; err != nil {
					return err
				}
			}
		}
		if o.UserCouponID > 0 {
			if err := tx.Model(&model.UserCoupon{}).Where("id = ? AND used_order_no = ?", o.UserCouponID, o.OrderNo).Updates(map[string]any{"status": model.UserCouponStatusUnused, "used_order_no": "", "used_at": nil}).Error; err != nil {
				return err
			}
		}
		now := time.Now()
		o.Status, o.AftersaleStatus, o.RefundedAt, o.StockDeducted = model.OrderStatusRefunded, "refunded", &now, false
		return nil
	})
	if err == nil {
		for _, it := range o.Items {
			invalidateProductCache(it.ProductID)
		}
	}
	return o, err
}

func validShipment(carrier, trackingNo string) bool {
	carrier, trackingNo = strings.TrimSpace(carrier), strings.TrimSpace(trackingNo)
	if carrier == "" || len([]rune(carrier)) > 64 || trackingNo == "" || len(trackingNo) > 128 {
		return false
	}
	for _, ch := range trackingNo {
		if !((ch >= '0' && ch <= '9') || (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '-') {
			return false
		}
	}
	return true
}

func adminShipOrder(db *gorm.DB, hub *notificationHub) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := lifecycleID(c, db)
		if !ok {
			return
		}
		var req struct {
			Carrier    string `json:"carrier"`
			TrackingNo string `json:"trackingNo"`
		}
		if c.ShouldBindJSON(&req) != nil || !validShipment(req.Carrier, req.TrackingNo) {
			c.JSON(400, gin.H{"message": "请填写快递公司和有效运单号"})
			return
		}
		o, err := changeOrder(db, id, nil, func(tx *gorm.DB, o *model.Order) error {
			if o.Status != model.OrderStatusPaid {
				return fmt.Errorf("仅待发货订单可以发货")
			}
			now := time.Now()
			o.Carrier, o.TrackingNo, o.ShippedAt, o.Status = strings.TrimSpace(req.Carrier), strings.TrimSpace(req.TrackingNo), &now, model.OrderStatusShipped
			o.Logistics = append(o.Logistics, model.LogisticsEvent{Time: now, Text: "商家已发货"})
			return nil
		})
		if err == nil {
			notify(db, hub, o.UserID, model.NotificationTypeOrder, "订单已发货", o.Carrier+" "+o.TrackingNo, o.OrderNo)
		}
		orderResult(c, o, err)
	}
}

func adminAddLogistics(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := lifecycleID(c, db)
		if !ok {
			return
		}
		var req struct {
			Text string `json:"text"`
		}
		if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Text) == "" || len([]rune(req.Text)) > 500 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请填写 1-500 字物流进度"})
			return
		}
		o, err := changeOrder(db, id, nil, func(tx *gorm.DB, o *model.Order) error {
			if o.ShippedAt == nil || o.Status == model.OrderStatusRefunded {
				return fmt.Errorf("当前订单不能更新物流")
			}
			o.Logistics = append(o.Logistics, model.LogisticsEvent{Time: time.Now(), Text: strings.TrimSpace(req.Text)})
			return nil
		})
		orderResult(c, o, err)
	}
}
