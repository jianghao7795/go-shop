package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shop/internal/logx"
	"shop/internal/model"
)

var errPaymentQuery = errors.New("payment query failed")

func orderPaymentStatus(order model.Order) string {
	switch order.Status {
	case model.OrderStatusPending:
		return "pending"
	case model.OrderStatusPaid, model.OrderStatusShipped, model.OrderStatusCompleted, model.OrderStatusFinished:
		return "paid"
	case model.OrderStatusAftersale:
		return "aftersale"
	case model.OrderStatusCancelled:
		return "closed"
	case model.OrderStatusRefunded:
		return "refunded"
	default:
		return "unknown"
	}
}

func paymentStatusResponse(order model.Order) gin.H {
	return gin.H{"status": orderPaymentStatus(order), "orderStatus": order.Status, "expiresAt": order.CreatedAt.Add(paymentWindow), "serverTime": time.Now()}
}

// syncOrderPayment is used by the cashier and the background reconciliation task.
// Only the gateway's verified result can settle a pending order.
func syncOrderPayment(ctx context.Context, db *gorm.DB, payer alipayPayer, order *model.Order) error {
	if order.Status != model.OrderStatusPending || payer == nil {
		return nil
	}
	status, tradeNo, amount, err := payer.Query(ctx, order.OrderNo)
	if err != nil {
		return fmt.Errorf("%w: %v", errPaymentQuery, err)
	}
	ctxDB := db.WithContext(ctx)
	switch status {
	case alipayStatusSuccess, alipayStatusFinished:
		if !alipayAmountMatches(amount, order.Amount) {
			return fmt.Errorf("%w: amount mismatch for %s", errPaymentQuery, order.OrderNo)
		}
		if err := markPaid(ctxDB, order.ID, tradeNo); err != nil {
			return err
		}
	case alipayStatusClosed:
		if err := ctxDB.Model(&model.Order{}).Where("id = ? AND status = ?", order.ID, model.OrderStatusPending).Update("status", model.OrderStatusCancelled).Error; err != nil {
			return err
		}
	}
	// A callback or cancellation may have changed the order during the query.
	return ctxDB.First(order, order.ID).Error
}

// processPaymentBatch uses a cursor so a slow/failing old payment cannot prevent
// subsequent orders from being checked. The caller retains the cursor between runs.
func processPaymentBatch(ctx context.Context, db *gorm.DB, payer alipayPayer, hub *notificationHub, afterID uint, now time.Time) (uint, error) {
	var orders []model.Order
	err := db.WithContext(ctx).Where("id > ? AND status = ? AND (payment_started = ? OR created_at <= ?)", afterID, model.OrderStatusPending, true, now.Add(-paymentWindow)).Order("id ASC").Limit(100).Find(&orders).Error
	if err != nil {
		return afterID, err
	}
	if len(orders) == 0 {
		return 0, nil
	}
	for _, order := range orders {
		if err := ctx.Err(); err != nil {
			return afterID, err
		}
		previousStatus := order.Status
		if payer != nil && order.PaymentStarted {
			if err := syncOrderPayment(ctx, db, payer, &order); err != nil {
				logx.L().Warnf("reconcile payment %s: %v", order.OrderNo, err)
			}
		}
		if ctx.Err() != nil {
			return order.ID, ctx.Err()
		}
		if order.Status == model.OrderStatusPending && !now.Before(order.CreatedAt.Add(paymentWindow)) {
			updated, err := cancelPendingOrder(ctx, db, payer, order.ID, nil)
			if err != nil {
				logx.L().Warnf("expire order %s: %v", order.OrderNo, err)
			} else {
				order = updated
			}
		}
		if previousStatus != order.Status {
			notify(db, hub, order.UserID, model.NotificationTypeOrder, "订单支付状态更新", "订单 "+order.OrderNo+"："+statusText[order.Status], order.OrderNo)
		}
		afterID = order.ID
	}
	if len(orders) < 100 {
		return 0, nil
	}
	return afterID, nil
}
