package api

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/skip2/go-qrcode"
	"github.com/smartwalle/alipay/v3"
	"gorm.io/gorm"

	"shop/internal/model"
)

// markPaid 幂等落库：仅当订单仍是待付款时，标记为已支付并写入支付宝交易号。
func markPaid(db *gorm.DB, orderID uint, tradeNo string) error {
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
	DecodeNotify(ctx context.Context, values url.Values) (status, tradeNo, outTradeNo string, err error)
}

// alipaySDK 是 alipayPayer 的 SDK 实现，包装 *alipay.Client。
type alipaySDK struct {
	client *alipay.Client
}

func (a *alipaySDK) PreCreate(ctx context.Context, outTradeNo, totalAmount, subject string) (string, error) {
	p := alipay.TradePreCreate{}
	p.OutTradeNo = outTradeNo
	p.TotalAmount = totalAmount
	p.Subject = subject
	rsp, err := a.client.TradePreCreate(ctx, p)
	if err != nil {
		return "", err
	}
	if !rsp.Code.IsSuccess() {
		return "", fmt.Errorf("支付宝预下单失败: %s", rsp.SubMsg)
	}
	return rsp.QRCode, nil
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

func (a *alipaySDK) DecodeNotify(ctx context.Context, values url.Values) (string, string, string, error) {
	noti, err := a.client.DecodeNotification(ctx, values)
	if err != nil {
		return "", "", "", err
	}
	return string(noti.TradeStatus), noti.TradeNo, noti.OutTradeNo, nil
}

// alipayConfig 是从环境变量读到的支付宝配置。
type alipayConfig struct {
	appID      string
	privateKey string
	publicKey  string
	production bool
}

func loadAlipayConfig() (alipayConfig, bool) {
	cfg := alipayConfig{
		appID:      os.Getenv("ALIPAY_APP_ID"),
		privateKey: os.Getenv("ALIPAY_PRIVATE_KEY"),
		publicKey:  os.Getenv("ALIPAY_PUBLIC_KEY"),
		production: os.Getenv("ALIPAY_PRODUCTION") == "true",
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
	client, err := alipay.New(cfg.appID, cfg.privateKey, cfg.production)
	if err != nil {
		return nil, err
	}
	if err := client.LoadAliPayPublicKey(cfg.publicKey); err != nil {
		return nil, err
	}
	return &alipaySDK{client: client}, nil
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
