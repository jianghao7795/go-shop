package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shop/internal/model"
)

// listCoupons 返回券模板列表（领券中心展示用）：数据库可用时读 shop_coupons，否则回退到内置种子数据。
func listCoupons(db *gorm.DB, databaseReady bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !databaseReady || db == nil {
			c.JSON(http.StatusOK, model.Coupons)
			return
		}
		var list []model.Coupon
		if err := db.Order("id ASC").Find(&list).Error; err != nil || len(list) == 0 {
			c.JSON(http.StatusOK, model.Coupons)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

// myCoupon 是「我的券」返回结构：在用户券基础上附带券模板信息。
type myCoupon struct {
	model.UserCoupon
	Title     string      `json:"title"`
	Amount    model.Price `json:"amount"`
	MinAmount model.Price `json:"minAmount"`
	Condition string      `json:"condition"`
	StartAt   time.Time   `json:"startAt"`
	EndAt     time.Time   `json:"endAt"`
}

// listMyCoupons 返回当前用户已领取的券（附带券模板信息）。
func listMyCoupons(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !dbRequired(c, db) {
			return
		}
		var ucs []model.UserCoupon
		db.Where("user_id = ?", currentUser(c)).Order("id DESC").Find(&ucs)
		coupons := map[uint]model.Coupon{}
		if len(ucs) > 0 {
			ids := make([]uint, 0, len(ucs))
			for _, uc := range ucs {
				ids = append(ids, uc.CouponID)
			}
			var cs []model.Coupon
			db.Where("id IN ?", ids).Find(&cs)
			for _, c := range cs {
				coupons[c.ID] = c
			}
		}
		items := make([]myCoupon, 0, len(ucs))
		for _, uc := range ucs {
			c := coupons[uc.CouponID]
			items = append(items, myCoupon{
				UserCoupon: uc,
				Title:      c.Title,
				Amount:     c.Amount,
				MinAmount:  c.MinAmount,
				Condition:  c.Condition,
				StartAt:    c.StartAt,
				EndAt:      c.EndAt,
			})
		}
		c.JSON(http.StatusOK, items)
	}
}

// claimCoupon 领取优惠券：同券每人限领 1 次。
func claimCoupon(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !dbRequired(c, db) {
			return
		}
		couponID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "无效的优惠券"})
			return
		}
		var coupon model.Coupon
		if db.First(&coupon, couponID).Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "优惠券不存在"})
			return
		}
		var n int64
		db.Model(&model.UserCoupon{}).Where("user_id = ? AND coupon_id = ?", currentUser(c), couponID).Count(&n)
		if n > 0 {
			c.JSON(http.StatusConflict, gin.H{"message": "已领取过该券"})
			return
		}
		uc := model.UserCoupon{UserID: currentUser(c), CouponID: uint(couponID), Status: model.UserCouponStatusUnused}
		if err := db.Create(&uc).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "领取失败"})
			return
		}
		c.JSON(http.StatusOK, uc)
	}
}
