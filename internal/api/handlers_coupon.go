package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// coupon 是优惠券（当前为静态演示数据）。
type coupon struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Amount    int    `json:"amount"`
	Condition string `json:"condition"`
}

func listCoupons() gin.HandlerFunc {
	return func(c *gin.Context) {
		coupons := []coupon{
			{ID: 1, Title: "新人专享券", Amount: 10, Condition: "满 99 元可用"},
			{ID: 2, Title: "全场通用券", Amount: 20, Condition: "满 199 元可用"},
			{ID: 3, Title: "数码品类券", Amount: 50, Condition: "满 499 元可用"},
			{ID: 4, Title: "满减优惠券", Amount: 100, Condition: "满 999 元可用"},
		}
		c.JSON(http.StatusOK, coupons)
	}
}
