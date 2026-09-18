package api

import (
	"net/http"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"shop/internal/model"
)

func registerHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "用户名和密码不能为空"})
			return
		}
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用，无法注册"})
			return
		}
		var count int64
		db.Model(&model.User{}).Where("username = ?", req.Username).Count(&count)
		if count > 0 {
			c.JSON(http.StatusConflict, gin.H{"message": "用户名已存在"})
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "密码加密失败"})
			return
		}
		if err := db.Create(&model.User{Username: req.Username, Password: string(hash)}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "注册失败，请稍后重试"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "注册成功", "username": req.Username})
	}
}

func meHandler(c *gin.Context) {
	claims := jwt.ExtractClaims(c)
	username, _ := claims["user"].(string)
	role, _ := claims["role"].(string)
	c.JSON(http.StatusOK, gin.H{"username": username, "role": role})
}
