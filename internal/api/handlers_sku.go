package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"shop/internal/model"
	"strings"
)

func adminListSkus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := lifecycleID(c, db)
		if !ok {
			return
		}
		var product model.Product
		if db.First(&product, id).Error != nil {
			c.JSON(404, gin.H{"message": "商品不存在"})
			return
		}
		skus := []model.ProductSku{}
		if db.Where("product_id = ?", id).Order("id ASC").Find(&skus).Error != nil {
			c.JSON(500, gin.H{"message": "读取规格失败"})
			return
		}
		c.JSON(200, skus)
	}
}

func adminUpdateSku(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := lifecycleID(c, db)
		if !ok {
			return
		}
		var req struct {
			Specs string      `json:"specs"`
			Price model.Price `json:"price"`
			Stock int         `json:"stock"`
		}
		if c.ShouldBindJSON(&req) != nil || strings.TrimSpace(req.Specs) == "" || len([]rune(req.Specs)) > 128 || req.Price < 0 || req.Stock < 0 {
			c.JSON(400, gin.H{"message": "请填写规格，价格和库存不能为负"})
			return
		}
		var sku model.ProductSku
		if db.First(&sku, id).Error != nil {
			c.JSON(404, gin.H{"message": "规格不存在"})
			return
		}
		if db.Model(&sku).Updates(map[string]any{"specs": strings.TrimSpace(req.Specs), "price": req.Price, "stock": req.Stock}).Error != nil {
			c.JSON(500, gin.H{"message": "更新失败"})
			return
		}
		invalidateProductCache(sku.ProductID)
		c.JSON(200, gin.H{"message": "已更新"})
	}
}
