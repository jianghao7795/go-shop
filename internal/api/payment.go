package api

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
	"github.com/smartwalle/alipay/v3"
	"gorm.io/gorm"

	"shop/internal/logx"
	"shop/internal/model"
)

// markPaid 幂等落库：仅当订单仍是待付款时，认领为已支付并扣减库存。
// 支付时才扣库存以杜绝超卖；任一商品库存不足则整单标记售后（退款），不扣库存。
func markPaid(db *gorm.DB, orderID uint, tradeNo string) error {
	if tradeNo == "" {
		return fmt.Errorf("支付宝交易号为空")
	}
	var order model.Order
	err := db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		paid := map[string]any{"status": model.OrderStatusPaid, "trade_no": tradeNo, "pay_method": "alipay", "paid_at": &now}
		// Claim with a write before reading so concurrent notifications serialize on the order.
		r := tx.Model(&model.Order{}).Where("id = ? AND status = ?", orderID, model.OrderStatusPending).Updates(paid)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 0 {
			// A late payment on a cancelled order must remain visible and refundable.
			paid["status"] = model.OrderStatusAftersale
			paid["aftersale_type"] = "refund_only"
			paid["aftersale_status"] = "requested"
			paid["aftersale_reason"] = "订单取消后收到支付款项，请原路退款"
			paid["aftersale_previous_status"] = model.OrderStatusCancelled
			return tx.Model(&model.Order{}).Where("id = ? AND status = ?", orderID, model.OrderStatusCancelled).Updates(paid).Error
		}
		if err := tx.First(&order, orderID).Error; err != nil {
			return err
		}
		if err := tx.SavePoint("inventory").Error; err != nil {
			return err
		}
		unavailable := false
		for _, it := range order.Items {
			if it.ProductID <= 0 || it.Quantity <= 0 {
				continue
			}
			q := tx.Model(&model.Product{}).Where("id = ? AND stock >= ?", it.ProductID, it.Quantity)
			if it.SkuID > 0 {
				q = tx.Model(&model.ProductSku{}).Where("id = ? AND stock >= ?", it.SkuID, it.Quantity)
			}
			r := q.Update("stock", gorm.Expr("stock - ?", it.Quantity))
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected == 0 {
				unavailable = true
				break
			}
		}
		if !unavailable && order.UserCouponID > 0 {
			r := tx.Model(&model.UserCoupon{}).Where("id = ? AND user_id = ? AND status = ?", order.UserCouponID, order.UserID, model.UserCouponStatusUnused).
				Updates(map[string]any{"status": model.UserCouponStatusUsed, "used_order_no": order.OrderNo, "used_at": &now})
			if r.Error != nil {
				return r.Error
			}
			unavailable = r.RowsAffected == 0
		}
		if unavailable {
			if err := tx.RollbackTo("inventory").Error; err != nil {
				return err
			}
			return tx.Model(&order).Updates(map[string]any{"status": model.OrderStatusAftersale, "aftersale_type": "refund_only", "aftersale_status": "requested", "aftersale_previous_status": model.OrderStatusPending, "aftersale_reason": "支付成功，但库存或优惠券不可用，请原路退款"}).Error
		}
		for _, it := range order.Items {
			if it.ProductID > 0 && it.Quantity > 0 {
				if err := tx.Model(&model.Product{}).Where("id = ?", it.ProductID).Update("sales", gorm.Expr("sales + ?", it.Quantity)).Error; err != nil {
					return err
				}
			}
		}
		return tx.Model(&order).Update("stock_deducted", true).Error
	})
	if err == nil {
		for _, it := range order.Items {
			invalidateProductCache(it.ProductID)
		}
	}
	return err
}

// 支付宝交易状态（与 SDK 的 TradeStatus 字符串一一对应）。
const (
	alipayStatusSuccess      = "TRADE_SUCCESS"
	alipayStatusFinished     = "TRADE_FINISHED"
	alipayStatusWaitBuyerPay = "WAIT_BUYER_PAY"
	alipayStatusClosed       = "TRADE_CLOSED"
)

// alipayPayer 抽象支付宝当面付操作，便于测试注入 mock。
type alipayPayer interface {
	PreCreate(ctx context.Context, outTradeNo, totalAmount, subject string) (qrCode string, err error)
	Query(ctx context.Context, outTradeNo string) (status, tradeNo, totalAmount string, err error)
	DecodeNotify(ctx context.Context, values url.Values) (status, tradeNo, outTradeNo, totalAmount string, err error)
}

// alipaySDK 是 alipayPayer 的 SDK 实现，包装 *alipay.Client。
type alipaySDK struct {
	client    *alipay.Client
	appID     string
	notifyURL string

	mu      sync.Mutex
	qrCache map[string]string // outTradeNo -> 二维码串，避免同一订单重复预下单
}

func (a *alipaySDK) PreCreate(ctx context.Context, outTradeNo, totalAmount, subject string) (string, error) {
	if qr, ok := a.cachedQR(outTradeNo); ok {
		return qr, nil
	}
	p := buildPrecreate(outTradeNo, totalAmount, subject, a.notifyURL)
	rsp, err := a.client.TradePreCreate(ctx, p)
	if err != nil {
		return "", err
	}
	if !rsp.Code.IsSuccess() {
		return "", fmt.Errorf("支付宝预下单失败: %s", rsp.SubMsg)
	}
	a.cacheQR(outTradeNo, rsp.QRCode)
	return rsp.QRCode, nil
}

// cachedQR 返回已缓存的二维码串（若有）；并发安全。
func (a *alipaySDK) cachedQR(outTradeNo string) (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.qrCache == nil {
		return "", false
	}
	qr, ok := a.qrCache[outTradeNo]
	return qr, ok
}

// cacheQR 缓存二维码串；并发安全。
func (a *alipaySDK) cacheQR(outTradeNo, qr string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.qrCache == nil {
		a.qrCache = make(map[string]string)
	}
	a.qrCache[outTradeNo] = qr
}

// buildPrecreate 组装当面付预下单参数（含 notify_url，供生产环境异步通知兜底）。
func buildPrecreate(outTradeNo, totalAmount, subject, notifyURL string) alipay.TradePreCreate {
	p := alipay.TradePreCreate{}
	p.OutTradeNo = outTradeNo
	p.TotalAmount = totalAmount
	p.Subject = subject
	p.NotifyURL = notifyURL
	return p
}

func (a *alipaySDK) Query(ctx context.Context, outTradeNo string) (string, string, string, error) {
	q := alipay.TradeQuery{OutTradeNo: outTradeNo}
	rsp, err := a.client.TradeQuery(ctx, q)
	if err != nil {
		return "", "", "", err
	}
	if !rsp.Code.IsSuccess() {
		if rsp.SubCode == "ACQ.TRADE_NOT_EXIST" {
			return alipayStatusWaitBuyerPay, "", "", nil
		}
		return "", "", "", fmt.Errorf("支付宝查询失败: %s", rsp.SubMsg)
	}
	if rsp.OutTradeNo != outTradeNo {
		return "", "", "", fmt.Errorf("支付宝查询订单号不匹配")
	}
	return string(rsp.TradeStatus), rsp.TradeNo, rsp.TotalAmount, nil
}

func (a *alipaySDK) DecodeNotify(ctx context.Context, values url.Values) (string, string, string, string, error) {
	noti, err := a.client.DecodeNotification(ctx, values)
	if err != nil {
		return "", "", "", "", err
	}
	if noti.AppId != a.appID {
		return "", "", "", "", fmt.Errorf("支付宝回调应用不匹配")
	}
	return string(noti.TradeStatus), noti.TradeNo, noti.OutTradeNo, noti.TotalAmount, nil
}

// newAlipayPayer 构建支付宝客户端；未配置时返回 (nil, nil)，配置错误返回 (nil, err)。
func newAlipayPayer() (alipayPayer, error) {
	cfg, ok, err := loadAlipayConfig()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	client, err := alipay.New(cfg.appID, cfg.privateKey, cfg.production, alipay.WithHTTPClient(alipayHTTPClient()))
	if err != nil {
		return nil, err
	}
	if err := client.LoadAliPayPublicKey(cfg.publicKey); err != nil {
		return nil, err
	}
	return &alipaySDK{client: client, appID: cfg.appID, notifyURL: cfg.notifyURL, qrCache: make(map[string]string)}, nil
}

var alipayAmountPattern = regexp.MustCompile(`^[0-9]{1,9}(\.[0-9]{1,2})?$`)

// Compare integer cents, allowing equivalent gateway formats such as 19.9 and 19.90.
func alipayAmountMatches(amount string, expected model.Price) bool {
	if !alipayAmountPattern.MatchString(amount) {
		return false
	}
	parts := strings.SplitN(amount, ".", 2)
	yuan, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	var cents int64
	if len(parts) == 2 {
		fraction := parts[1]
		if len(fraction) == 1 {
			fraction += "0"
		}
		cents, _ = strconv.ParseInt(fraction, 10, 64)
	}
	return model.Price(yuan*100+cents) == expected
}

// alipayHTTPClient 返回带超时的 HTTP 客户端，避免支付接口无限阻塞。
func alipayHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// qrCodeDataURL 把文本生成 base64 编码的 PNG data URL，供前端 <img :src> 直显。
func qrCodeDataURL(text string) (string, error) {
	qr, err := qrcode.New(text, qrcode.Medium)
	if err != nil {
		return "", err
	}
	png, err := qr.PNG(256)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

// handleAlipayNotify 处理支付宝异步通知：验签通过且交易成功时幂等落库。
func handleAlipayNotify(db *gorm.DB, payer alipayPayer) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil || payer == nil {
			c.String(http.StatusInternalServerError, "failure")
			return
		}
		if err := c.Request.ParseForm(); err != nil {
			c.String(http.StatusBadRequest, "failure")
			return
		}
		status, tradeNo, outTradeNo, totalAmount, err := payer.DecodeNotify(c.Request.Context(), c.Request.Form)
		if err != nil {
			c.String(http.StatusBadRequest, "failure")
			return
		}
		if status == alipayStatusSuccess || status == alipayStatusFinished {
			var order model.Order
			err := db.Where("order_no = ?", outTradeNo).First(&order).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.String(http.StatusNotFound, "failure")
				return
			}
			if err != nil {
				c.String(http.StatusInternalServerError, "failure")
				return
			}
			if !alipayAmountMatches(totalAmount, order.Amount) {
				logx.L().Errorf("alipay notify amount mismatch: order %s notify %s want %s", order.OrderNo, totalAmount, order.Amount.YuanString())
				c.String(http.StatusBadRequest, "failure")
				return
			}
			if err := markPaid(db, order.ID, tradeNo); err != nil {
				c.String(http.StatusInternalServerError, "failure")
				return
			}
		}
		c.String(http.StatusOK, "success")
	}
}
