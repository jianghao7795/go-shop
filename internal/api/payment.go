package api

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
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
	if err := db.Where("id = ?", orderID).First(&order).Error; err != nil {
		return err
	}
	now := time.Now()
	// 原子认领：只有把 pending 改成 paid 的那次调用才继续，保证并发幂等。
	res := db.Model(&model.Order{}).
		Where("id = ? AND status = ?", orderID, model.OrderStatusPending).
		Updates(map[string]any{
			"status":     model.OrderStatusPaid,
			"trade_no":   tradeNo,
			"pay_method": "alipay",
			"paid_at":    &now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return nil // 幂等：别的调用已处理
	}
	// 认领成功，扣库存 + 核销券；任一失败则回补已扣并转售后。
	var deducted []model.OrderItem
	fail := false
	for _, it := range order.Items {
		if it.ProductID == 0 || it.Quantity <= 0 {
			continue
		}
		var r *gorm.DB
		if it.SkuID > 0 {
			r = db.Model(&model.ProductSku{}).Where("id = ? AND stock >= ?", it.SkuID, it.Quantity).
				Update("stock", gorm.Expr("stock - ?", it.Quantity))
		} else {
			r = db.Model(&model.Product{}).Where("id = ? AND stock >= ?", it.ProductID, it.Quantity).
				Update("stock", gorm.Expr("stock - ?", it.Quantity))
		}
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 0 {
			fail = true
			break
		}
		deducted = append(deducted, it)
	}
	if !fail && order.UserCouponID > 0 {
		r := db.Model(&model.UserCoupon{}).
			Where("id = ? AND user_id = ? AND status = ?", order.UserCouponID, order.UserID, model.UserCouponStatusUnused).
			Updates(map[string]any{
				"status":        model.UserCouponStatusUsed,
				"used_order_no": order.OrderNo,
				"used_at":       &now,
			})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 0 {
			fail = true
		}
	}
	if fail {
		for _, d := range deducted {
			if d.SkuID > 0 {
				db.Model(&model.ProductSku{}).Where("id = ?", d.SkuID).
					Update("stock", gorm.Expr("stock + ?", d.Quantity))
			} else {
				db.Model(&model.Product{}).Where("id = ?", d.ProductID).
					Update("stock", gorm.Expr("stock + ?", d.Quantity))
			}
		}
		logx.L().Warnf("order %s paid but stock/coupon unavailable", order.OrderNo)
		return db.Model(&model.Order{}).Where("id = ?", orderID).
			Update("status", model.OrderStatusAftersale).Error
	}
	// 全部成功：累加销量。
	for _, it := range order.Items {
		if it.ProductID == 0 || it.Quantity <= 0 {
			continue
		}
		if err := db.Model(&model.Product{}).Where("id = ?", it.ProductID).
			Update("sales", gorm.Expr("sales + ?", it.Quantity)).Error; err != nil {
			return err
		}
	}
	return nil
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
	Query(ctx context.Context, outTradeNo string) (status, tradeNo string, err error)
	DecodeNotify(ctx context.Context, values url.Values) (status, tradeNo, outTradeNo, totalAmount string, err error)
}

// alipaySDK 是 alipayPayer 的 SDK 实现，包装 *alipay.Client。
type alipaySDK struct {
	client    *alipay.Client
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

func (a *alipaySDK) Query(ctx context.Context, outTradeNo string) (string, string, error) {
	q := alipay.TradeQuery{OutTradeNo: outTradeNo}
	rsp, err := a.client.TradeQuery(ctx, q)
	if err != nil {
		return "", "", err
	}
	if !rsp.Code.IsSuccess() {
		return "", "", fmt.Errorf("支付宝查询失败: %s", rsp.SubMsg)
	}
	return string(rsp.TradeStatus), rsp.TradeNo, nil
}

func (a *alipaySDK) DecodeNotify(ctx context.Context, values url.Values) (string, string, string, string, error) {
	noti, err := a.client.DecodeNotification(ctx, values)
	if err != nil {
		return "", "", "", "", err
	}
	return string(noti.TradeStatus), noti.TradeNo, noti.OutTradeNo, noti.TotalAmount, nil
}

// alipayConfig 是从环境变量读到的支付宝配置。
type alipayConfig struct {
	appID      string
	privateKey string
	publicKey  string
	production bool
	notifyURL  string
}

func loadAlipayConfig() (alipayConfig, bool) {
	cfg := alipayConfig{
		appID:      os.Getenv("ALIPAY_APP_ID"),
		privateKey: os.Getenv("ALIPAY_PRIVATE_KEY"),
		publicKey:  os.Getenv("ALIPAY_PUBLIC_KEY"),
		production: os.Getenv("ALIPAY_PRODUCTION") == "true",
		notifyURL:  os.Getenv("ALIPAY_NOTIFY_URL"),
	}
	if cfg.appID == "" || cfg.privateKey == "" || cfg.publicKey == "" {
		return alipayConfig{}, false
	}
	return cfg, true
}

// newAlipayPayer 构建支付宝客户端；未配置时返回 (nil, nil)，配置错误返回 (nil, err)。
func newAlipayPayer() (alipayPayer, error) {
	cfg, ok := loadAlipayConfig()
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
	return &alipaySDK{client: client, notifyURL: cfg.notifyURL, qrCache: make(map[string]string)}, nil
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
		_ = c.Request.ParseForm()
		status, tradeNo, outTradeNo, totalAmount, err := payer.DecodeNotify(c.Request.Context(), c.Request.Form)
		if err != nil {
			c.String(http.StatusBadRequest, "failure")
			return
		}
		if status == alipayStatusSuccess || status == alipayStatusFinished {
			var order model.Order
			if db.Where("order_no = ?", outTradeNo).First(&order).Error == nil {
				if totalAmount != "" && totalAmount != order.Amount.YuanString() {
					logx.L().Errorf("alipay notify amount mismatch: order %s notify %s want %s", order.OrderNo, totalAmount, order.Amount.YuanString())
					c.String(http.StatusOK, "success")
					return
				}
				if err := markPaid(db, order.ID, tradeNo); err != nil {
					c.String(http.StatusInternalServerError, "failure")
					return
				}
			}
		}
		c.String(http.StatusOK, "success")
	}
}
