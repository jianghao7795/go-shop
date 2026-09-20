package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shop/internal/model"
)

// listCategories 返回商品分类列表：数据库可用时读 shop_categories，否则回退到内置种子数据。
func listCategories(db *gorm.DB, databaseReady bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !databaseReady || db == nil {
			c.JSON(http.StatusOK, model.Categories)
			return
		}
		var cats []model.Category
		if err := db.Order("id ASC").Find(&cats).Error; err != nil || len(cats) == 0 {
			c.JSON(http.StatusOK, model.Categories)
			return
		}
		c.JSON(http.StatusOK, cats)
	}
}
