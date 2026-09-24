package api

import (
	"net/http"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shop/internal/model"
)

// adminRequired 校验当前登录用户是否为管理员。按数据库中的角色判断（角色变更即时生效，
// 无需重新登录），数据库不可用时回退到 JWT 中的 role；非管理员返回 403。
func adminRequired(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin := false
		if db != nil {
			var user model.User
			if err := db.Where("username = ?", currentUser(c)).First(&user).Error; err == nil && user.Role == model.RoleAdmin {
				isAdmin = true
			}
		} else {
			role, _ := jwt.ExtractClaims(c)["role"].(string)
			isAdmin = role == model.RoleAdmin
		}
		if !isAdmin {
			c.JSON(http.StatusForbidden, gin.H{"message": "无管理员权限"})
			c.Abort()
			return
		}
		c.Next()
	}
}
