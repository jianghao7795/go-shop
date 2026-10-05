package api

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
	"github.com/smartwalle/alipay/v3"
	"gorm.io/gorm"

	"shop/internal/model"
)

// markPaid 幂等落库：仅当订单仍是待付款时，标记为已支付并写入支付宝交易号。
func markPaid(db *gorm.DB, orderID uint, tradeNo string) error {
	if tradeNo == "" {
		return fmt.Errorf("支付宝交易号为空")
	}
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
}

func (a *alipaySDK) PreCreate(ctx context.Context, outTradeNo, totalAmount, subject string) (string, error) {
	p := buildPrecreate(outTradeNo, totalAmount, subject, a.notifyURL)
	rsp, err := a.client.TradePreCreate(ctx, p)
	if err != nil {
		return "", err
	}
	if !rsp.Code.IsSuccess() {
		return "", fmt.Errorf("支付宝预下单失败: %s", rsp.SubMsg)
	}
	return rsp.QRCode, nil
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
	return &alipaySDK{client: client, notifyURL: cfg.notifyURL}, nil
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
				if totalAmount != "" && totalAmount != fmt.Sprintf("%.2f", order.Amount) {
					log.Printf("alipay notify amount mismatch: order %s notify %s want %.2f", order.OrderNo, totalAmount, order.Amount)
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
