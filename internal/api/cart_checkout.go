package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"shop/internal/model"
)

type cartCheckoutInput struct {
	RequestID      string            `json:"requestId"`
	Version        *uint64           `json:"version"`
	AddressID      uint              `json:"addressId"`
	UserCouponID   uint              `json:"userCouponId"`
	ExpectedAmount *model.Price      `json:"expectedAmount"`
	Items          []model.OrderItem `json:"items"`
}

var checkoutIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// Creating the order and consuming exactly its cart quantities are atomic.
// The request receipt is committed in the same transaction, so retrying after
// a lost response neither creates another order nor consumes the cart twice.
func checkoutCart(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !dbRequired(c, db) {
			return
		}
		var req cartCheckoutInput
		if c.ShouldBindJSON(&req) != nil || !checkoutIDPattern.MatchString(req.RequestID) || req.Version == nil || req.ExpectedAmount == nil || len(req.Items) == 0 || req.AddressID == 0 {
			writeShopError(c, badCart("请确认商品、收货地址及订单金额"))
			return
		}
		inputs := make([]cartInput, 0, len(req.Items))
		for _, it := range req.Items {
			inputs = append(inputs, cartInput{it.ProductID, it.SkuID, it.Quantity})
		}
		if err := validateCartInput(inputs); err != nil {
			writeShopError(c, err)
			return
		}
		encoded, _ := json.Marshal(req)
		hash := sha256.Sum256(encoded)
		payloadHash := hex.EncodeToString(hash[:])
		user := currentUser(c)
		if err := ensureCartState(db, user); err != nil {
			writeShopError(c, err)
			return
		}
		var order model.Order
		var cart cartView
		err := db.Transaction(func(tx *gorm.DB) error {
			// Lock even for a replay. This serializes identical requests on MySQL
			// and takes SQLite's write lock before any reads.
			if err := tx.Model(&model.CartState{}).Where("user_id = ?", user).UpdateColumn("version", gorm.Expr("version")).Error; err != nil {
				return err
			}
			var receipt model.CheckoutRequest
			err := tx.Where("user_id = ? AND request_id = ?", user, req.RequestID).First(&receipt).Error
			if err == nil {
				if receipt.PayloadHash != payloadHash {
					return &shopError{409, "重复请求的订单内容不一致"}
				}
				if err := tx.Where("id = ? AND user_id = ?", receipt.OrderID, user).First(&order).Error; err != nil {
					return err
				}
				cart, err = readCart(tx, user)
				return err
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := advanceCart(tx, user, *req.Version); err != nil {
				return err
			}
			views, err := cartViews(tx, inputs)
			if err != nil {
				return err
			}
			var amount model.Price
			items := make([]model.OrderItem, 0, len(views))
			for i, v := range views {
				if !v.Available {
					return badCart(v.Name + "：" + v.Reason)
				}
				if req.Items[i].Price != v.Price {
					return &shopError{409, "商品价格已变化，请刷新后确认"}
				}
				var line model.CartItem
				if err := tx.Where("user_id = ? AND product_id = ? AND sku_id = ?", user, v.ProductID, v.SkuID).First(&line).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return cartConflict()
					}
					return err
				}
				if line.Quantity < v.Quantity {
					return cartConflict()
				}
				items = append(items, model.OrderItem{ProductID: v.ProductID, SkuID: v.SkuID, Quantity: v.Quantity, Name: v.Name, Specs: v.Specs, Price: v.Price, Emoji: v.Emoji, Color: v.Color})
				amount += v.Price * model.Price(v.Quantity)
				if line.Quantity == v.Quantity {
					err = tx.Delete(&line).Error
				} else {
					err = tx.Model(&line).Update("quantity", line.Quantity-v.Quantity).Error
				}
				if err != nil {
					return err
				}
			}
			if req.UserCouponID > 0 {
				var uc model.UserCoupon
				if err := tx.Where("id = ? AND user_id = ?", req.UserCouponID, user).First(&uc).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return badCart("优惠券不存在")
					}
					return err
				}
				if uc.Status != model.UserCouponStatusUnused {
					return badCart("优惠券已使用")
				}
				var coupon model.Coupon
				if err := tx.First(&coupon, uc.CouponID).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						return badCart("优惠券不存在")
					}
					return err
				}
				if !couponInPeriod(&coupon, time.Now()) {
					return badCart("优惠券不在有效期")
				}
				if amount < coupon.MinAmount {
					return badCart("未达到优惠券使用门槛")
				}
				amount, _ = applyCouponDiscount(amount, &coupon)
			}
			if amount != *req.ExpectedAmount {
				return &shopError{409, "订单金额已变化，请刷新后确认"}
			}
			var addr model.Address
			if err := tx.Where("id = ? AND user_id = ?", req.AddressID, user).First(&addr).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return badCart("收货地址不存在，请重新选择")
				}
				return err
			}
			order = model.Order{OrderNo: generateOrderNo(), UserID: user, Status: model.OrderStatusPending, LifecycleVersion: 1, Amount: amount, UserCouponID: req.UserCouponID, Receiver: addr.Name, Phone: addr.Phone, Region: addr.Region, Detail: addr.Detail, Items: items}
			if err := tx.Create(&order).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.CheckoutRequest{UserID: user, RequestID: req.RequestID, PayloadHash: payloadHash, OrderID: order.ID}).Error; err != nil {
				return err
			}
			cart, err = readCart(tx, user)
			return err
		})
		if err != nil {
			writeShopError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"order": order, "cart": cart})
	}
}
