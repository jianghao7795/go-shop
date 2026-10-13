package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shop/internal/model"
)

// listCategories 返回商品分类列表：优先读缓存，miss 则查库并回填；库不可用/为空回退种子数据。
func listCategories(db *gorm.DB, databaseReady bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !databaseReady || db == nil {
			c.JSON(http.StatusOK, model.Categories)
			return
		}
		if cats, ok := cachedCategories(); ok {
			c.JSON(http.StatusOK, cats)
			return
		}
		var cats []model.Category
		if err := db.Order("id ASC").Find(&cats).Error; err != nil || len(cats) == 0 {
			c.JSON(http.StatusOK, model.Categories)
			return
		}
		cacheCategories(cats)
		c.JSON(http.StatusOK, cats)
	}
}
