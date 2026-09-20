package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shop/internal/model"
)

// listCoupons 返回优惠券列表：数据库可用时读 shop_coupons，否则回退到内置种子数据。
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
