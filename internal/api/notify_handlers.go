package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shop/internal/model"
)

// tokenFromQuery 允许 SSE（EventSource）通过 ?token= 传 JWT：
// 在请求没有 Authorization header 时，把 query 中的 token 写入 header，
// 以便复用现有 JWT 中间件。
func tokenFromQuery() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			if token := c.Query("token"); token != "" {
				c.Request.Header.Set("Authorization", "Bearer "+token)
			}
		}
		c.Next()
	}
}

// streamNotifications 建立 SSE 长连接，把当前用户的实时通知推给客户端。
func streamNotifications(hub *notificationHub) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := currentUser(c)
		ch := hub.subscribe(userID)
		defer hub.unsubscribe(userID, ch)

		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")
		c.Writer.Flush()

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case n := <-ch:
				data, _ := json.Marshal(n)
				c.SSEvent("notification", string(data))
				c.Writer.Flush()
			case <-ticker.C:
				if _, err := c.Writer.Write([]byte(": ping\n\n")); err != nil {
					return
				}
				c.Writer.Flush()
			case <-c.Request.Context().Done():
				return
			}
		}
	}
}

func listNotifications(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		var list []model.Notification
		db.Where("user_id = ?", currentUser(c)).Order("id DESC").Find(&list)
		c.JSON(http.StatusOK, list)
	}
}

func unreadCount(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		var count int64
		db.Model(&model.Notification{}).Where("user_id = ? AND read = ?", currentUser(c), false).Count(&count)
		c.JSON(http.StatusOK, gin.H{"count": count})
	}
}

func markNotificationRead(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "无效的通知"})
			return
		}
		db.Model(&model.Notification{}).Where("id = ? AND user_id = ?", id, currentUser(c)).Update("read", true)
		c.JSON(http.StatusOK, gin.H{"message": "已读"})
	}
}

func markAllNotificationsRead(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		db.Model(&model.Notification{}).Where("user_id = ?", currentUser(c)).Update("read", true)
		c.JSON(http.StatusOK, gin.H{"message": "已读"})
	}
}
