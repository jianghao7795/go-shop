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
		if item, ok := cachedProduct(id); ok {
			if !item.OnShelf {
				c.JSON(http.StatusNotFound, gin.H{"message": "product not found"})
				return
			}
			c.JSON(http.StatusOK, item)
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
		db.Where("product_id = ?", id).Find(&item.Skus)
		cacheProduct(item)
		c.JSON(http.StatusOK, item)
	}
}

// productSortOrder 根据 sort 参数返回排序子句。
func productSortOrder(sort string) string {
	switch sort {
	case "price_asc":
		return "price ASC"
	case "price_desc":
		return "price DESC"
	case "sales_desc":
		return "sales DESC"
	case "rating_desc":
		return "rating DESC"
	default:
		return "id DESC"
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
		if category == "all" {
			category = ""
		}
		featured := c.Query("featured") == "true"
		sort := c.Query("sort")
		if featured {
			sort = "sales_desc"
		}
		hasFilter := search != "" || category != "" || featured

		if search == "" {
			if items, ok := cachedProducts(featured, category, sort); ok {
				c.JSON(http.StatusOK, items)
				return
			}
		}

		// 查库前捕获 epoch，回填用它：避免「写后 epoch + 写前 items」的脏组合（同权限缓存）。
		var epoch int64
		if search == "" {
			epoch, _ = productEpoch()
		}

		query := db.Model(&model.Product{}).Where("on_shelf = ?", true)
		if featured {
			query = query.Where("featured = ?", true)
		}
		if search != "" {
			query = query.Where("name LIKE ? OR description LIKE ?", "%"+search+"%", "%"+search+"%")
		}
		if category != "" {
			query = query.Where("category = ?", category)
		}
		var items []model.Product
		if err := query.Order(productSortOrder(sort)).Find(&items).Error; err != nil {
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
		if search == "" && len(items) > 0 {
			cacheProducts(featured, category, sort, items, epoch)
		}
		c.JSON(http.StatusOK, items)
	}
}
