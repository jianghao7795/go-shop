package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"shop/internal/model"
)

// listCategories 返回商品分类元数据列表。
func listCategories() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, model.Categories)
	}
}
