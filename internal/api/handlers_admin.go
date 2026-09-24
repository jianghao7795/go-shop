package api

import (
	"net/http"
	"strconv"

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
