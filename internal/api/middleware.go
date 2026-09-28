package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// requirePermission 校验当前用户是否拥有指定权限点；数据库不可用时一律拒绝。
func requirePermission(db *gorm.DB, perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db != nil && hasPermission(effectivePermissions(db, currentUser(c)), perm) {
			c.Next()
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"message": "无权限"})
		c.Abort()
	}
}
