package api

import (
	"net/http"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"

	"shop/internal/model"
)

// adminRequired 校验当前登录用户是否为管理员，非管理员返回 403。
func adminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := jwt.ExtractClaims(c)
		role, _ := claims["role"].(string)
		if role != model.RoleAdmin {
			c.JSON(http.StatusForbidden, gin.H{"message": "无管理员权限"})
			c.Abort()
			return
		}
		c.Next()
	}
}
