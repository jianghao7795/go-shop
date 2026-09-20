package api

import (
	"net/http"
	"strconv"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shop/internal/model"
)

// currentUser 从 JWT claims 中取出当前登录用户名。
func currentUser(c *gin.Context) string {
	claims := jwt.ExtractClaims(c)
	username, _ := claims["user"].(string)
	return username
}

func listAddresses(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		var addrs []model.Address
		db.Where("user_id = ?", currentUser(c)).Order("is_default DESC, id DESC").Find(&addrs)
		c.JSON(http.StatusOK, addrs)
	}
}

func createAddress(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		var req struct {
			Name      string `json:"name" binding:"required"`
			Phone     string `json:"phone" binding:"required"`
			Region    string `json:"region" binding:"required"`
			Detail    string `json:"detail" binding:"required"`
			Tag       string `json:"tag"`
			IsDefault bool   `json:"isDefault"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请填写完整的收货信息"})
			return
		}
		if req.IsDefault {
			db.Model(&model.Address{}).Where("user_id = ?", currentUser(c)).Update("is_default", false)
		}
		addr := model.Address{
			UserID:    currentUser(c),
			Name:      req.Name,
			Phone:     req.Phone,
			Region:    req.Region,
			Detail:    req.Detail,
			Tag:       req.Tag,
			IsDefault: req.IsDefault,
		}
		if err := db.Create(&addr).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "保存地址失败"})
			return
		}
		c.JSON(http.StatusOK, addr)
	}
}

func updateAddress(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "无效的地址"})
			return
		}
		var addr model.Address
		if db.Where("id = ? AND user_id = ?", id, currentUser(c)).First(&addr).Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "地址不存在"})
			return
		}
		var req struct {
			Name      string `json:"name" binding:"required"`
			Phone     string `json:"phone" binding:"required"`
			Region    string `json:"region" binding:"required"`
			Detail    string `json:"detail" binding:"required"`
			Tag       string `json:"tag"`
			IsDefault bool   `json:"isDefault"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请填写完整的收货信息"})
			return
		}
		if req.IsDefault {
			db.Model(&model.Address{}).Where("user_id = ?", currentUser(c)).Update("is_default", false)
		}
		db.Model(&addr).Updates(map[string]interface{}{
			"name":       req.Name,
			"phone":      req.Phone,
			"region":     req.Region,
			"detail":     req.Detail,
			"tag":        req.Tag,
			"is_default": req.IsDefault,
		})
		c.JSON(http.StatusOK, addr)
	}
}

func deleteAddress(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "无效的地址"})
			return
		}
		db.Where("id = ? AND user_id = ?", id, currentUser(c)).Delete(&model.Address{})
		c.JSON(http.StatusOK, gin.H{"message": "已删除"})
	}
}

func setDefaultAddress(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "无效的地址"})
			return
		}
		db.Model(&model.Address{}).Where("user_id = ?", currentUser(c)).Update("is_default", false)
		db.Model(&model.Address{}).Where("id = ? AND user_id = ?", id, currentUser(c)).Update("is_default", true)
		c.JSON(http.StatusOK, gin.H{"message": "已设为默认"})
	}
}
