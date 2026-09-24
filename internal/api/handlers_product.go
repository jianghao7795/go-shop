package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shop/internal/model"
)

func healthHandler(databaseReady bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "database": databaseReady})
	}
}

func productDetailHandler(db *gorm.DB, databaseReady bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !databaseReady {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "database unavailable"})
			return
		}
		id, parseErr := strconv.Atoi(c.Param("id"))
		if parseErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid product id"})
			return
		}
		var item model.Product
		if queryErr := db.First(&item, id).Error; queryErr != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "product not found"})
			return
		}
		if !item.OnShelf {
			c.JSON(http.StatusNotFound, gin.H{"message": "product not found"})
			return
		}
		c.JSON(http.StatusOK, item)
	}
}

func productsHandler(db *gorm.DB, databaseReady bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !databaseReady {
			c.JSON(http.StatusOK, model.FallbackProducts)
			return
		}
		search := c.Query("search")
		category := c.Query("category")
		featured := c.Query("featured") == "true"
		hasFilter := search != "" || (category != "" && category != "all") || featured
		query := db.Model(&model.Product{}).Where("on_shelf = ?", true)
		if featured {
			query = query.Where("featured = ?", true)
		}
		if search != "" {
			query = query.Where("name LIKE ?", "%"+search+"%")
		}
		if category != "" && category != "all" {
			query = query.Where("category = ?", category)
		}
		var items []model.Product
		order := "id DESC"
		if featured {
			order = "sales DESC"
		}
		if err := query.Order(order).Find(&items).Error; err != nil {
			items = nil
		}
		if len(items) == 0 && !hasFilter {
			// 仅在商品表确实为空时才回退到内置目录，避免「全部下架」时误返回兜底数据
			var total int64
			db.Model(&model.Product{}).Count(&total)
			if total == 0 {
				items = model.FallbackProducts
			}
		}
		c.JSON(http.StatusOK, items)
	}
}
