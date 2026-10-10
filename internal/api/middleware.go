package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shop/internal/logx"
)

// RequestLogger 记录每个 HTTP 请求的 method/path/status/耗时/IP。
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logx.L().Infow("http request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", float64(time.Since(start).Microseconds())/1000.0,
			"ip", c.ClientIP(),
		)
	}
}

// dbRequired 检查数据库是否可用；不可用时已写好 503 响应并返回 false。
func dbRequired(c *gin.Context, db *gorm.DB) bool {
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
		return false
	}
	return true
}

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
