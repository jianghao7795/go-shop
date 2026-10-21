package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shop/internal/model"
)

// listProductReviews 返回商品的评价列表与平均分。
func listProductReviews(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		pid, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "非法商品 ID"})
			return
		}
		if !dbRequired(c, db) {
			return
		}
		var total int64
		db.Model(&model.Review{}).Where("product_id = ?", pid).Count(&total)
		var avg float64
		db.Model(&model.Review{}).Where("product_id = ?", pid).Select("COALESCE(AVG(rating), 0)").Scan(&avg)
		page, pageSize := parsePagination(c)
		var reviews []model.Review
		db.Model(&model.Review{}).Where("product_id = ?", pid).Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&reviews)
		c.JSON(http.StatusOK, gin.H{"reviews": reviews, "average": avg, "count": total, "page": page, "pageSize": pageSize})
	}
}

// submitOrderReviews 提交整单评价：对订单内各商品打分写评，订单「待评价」→「已完成」。
func submitOrderReviews(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "无效的订单"})
			return
		}
		if !dbRequired(c, db) {
			return
		}
		var order model.Order
		if db.Where("id = ? AND user_id = ?", id, currentUser(c)).First(&order).Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "订单不存在"})
			return
		}
		if order.Status != model.OrderStatusCompleted {
			c.JSON(http.StatusBadRequest, gin.H{"message": "订单状态不可评价"})
			return
		}
		var req struct {
			Reviews []struct {
				ProductID int     `json:"productId"`
				Rating    float64 `json:"rating"`
				Content   string  `json:"content"`
			} `json:"reviews"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || len(req.Reviews) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请填写评价"})
			return
		}
		for _, r := range req.Reviews {
			if r.Rating < 1 || r.Rating > 5 {
				c.JSON(http.StatusBadRequest, gin.H{"message": "评分需在 1-5 之间"})
				return
			}
			found := false
			for _, it := range order.Items {
				if it.ProductID == r.ProductID {
					found = true
					break
				}
			}
			if !found {
				c.JSON(http.StatusBadRequest, gin.H{"message": "商品不属于该订单"})
				return
			}
		}
		stateChanged := errors.New("order state changed")
		err = db.Transaction(func(tx *gorm.DB) error {
			// Claim completion atomically so concurrent aftersale/refund operations cannot be overwritten.
			r := tx.Model(&model.Order{}).Where("id = ? AND user_id = ? AND status = ?", id, currentUser(c), model.OrderStatusCompleted).Update("status", model.OrderStatusFinished)
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected == 0 {
				return stateChanged
			}
			for _, review := range req.Reviews {
				if err := tx.Create(&model.Review{UserID: currentUser(c), ProductID: review.ProductID, OrderID: order.ID, Rating: review.Rating, Content: review.Content}).Error; err != nil {
					return err
				}
			}
			for _, review := range req.Reviews {
				var avg float64
				if err := tx.Model(&model.Review{}).Where("product_id = ?", review.ProductID).Select("COALESCE(AVG(rating), 0)").Scan(&avg).Error; err != nil {
					return err
				}
				if err := tx.Model(&model.Product{}).Where("id = ?", review.ProductID).Update("rating", avg).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if errors.Is(err, stateChanged) {
			c.JSON(http.StatusConflict, gin.H{"message": "订单状态已变化，请刷新后重试"})
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "评价保存失败，请稍后重试"})
			return
		}
		for _, review := range req.Reviews {
			invalidateProductCache(review.ProductID)
		}
		c.JSON(http.StatusOK, gin.H{"message": "评价成功"})
	}
}
