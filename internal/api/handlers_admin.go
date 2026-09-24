package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shop/internal/model"
)

// adminListUsers 返回全部用户（含昵称/手机号/角色/状态）。
func adminListUsers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		var users []model.User
		if err := db.Order("id DESC").Find(&users).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "查询失败"})
			return
		}
		c.JSON(http.StatusOK, users)
	}
}

// adminUpdateUserStatus 禁用/启用用户（status: 1 正常，0 禁用）。
func adminUpdateUserStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "非法用户 ID"})
			return
		}
		var req struct {
			Status int `json:"status"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || (req.Status != 0 && req.Status != 1) {
			c.JSON(http.StatusBadRequest, gin.H{"message": "status 必须是 0 或 1"})
			return
		}
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		if err := db.Model(&model.User{}).Where("id = ?", id).Update("status", req.Status).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "更新失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
	}
}

type productPayload struct {
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	Price         float64 `json:"price"`
	OriginalPrice float64 `json:"originalPrice"`
	Emoji         string  `json:"emoji"`
	Color         string  `json:"color"`
	Category      string  `json:"category"`
	OnShelf       *bool   `json:"onShelf"`
}

func validateProduct(p productPayload) string {
	if strings.TrimSpace(p.Name) == "" {
		return "商品名称不能为空"
	}
	if p.Price < 0 || p.OriginalPrice < 0 {
		return "价格不能为负"
	}
	if strings.TrimSpace(p.Category) == "" {
		return "请选择分类"
	}
	return ""
}

func adminListProducts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		var items []model.Product
		if err := db.Order("id DESC").Find(&items).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "查询失败"})
			return
		}
		c.JSON(http.StatusOK, items)
	}
}

func adminCreateProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var p productPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请求格式错误"})
			return
		}
		if msg := validateProduct(p); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": msg})
			return
		}
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		item := model.Product{Name: p.Name, Description: p.Description, Price: p.Price, OriginalPrice: p.OriginalPrice, Emoji: p.Emoji, Color: p.Color, Category: p.Category}
		if p.OnShelf != nil {
			item.OnShelf = *p.OnShelf
		} else {
			item.OnShelf = true
		}
		if err := db.Create(&item).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "创建失败"})
			return
		}
		c.JSON(http.StatusOK, item)
	}
}

func adminUpdateProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "非法商品 ID"})
			return
		}
		var p productPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请求格式错误"})
			return
		}
		if msg := validateProduct(p); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": msg})
			return
		}
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		updates := map[string]any{"name": p.Name, "description": p.Description, "price": p.Price, "original_price": p.OriginalPrice, "emoji": p.Emoji, "color": p.Color, "category": p.Category}
		if p.OnShelf != nil {
			updates["on_shelf"] = *p.OnShelf
		}
		if err := db.Model(&model.Product{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "更新失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
	}
}

func adminDeleteProduct(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "非法商品 ID"})
			return
		}
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		if err := db.Delete(&model.Product{}, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "删除失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
	}
}
