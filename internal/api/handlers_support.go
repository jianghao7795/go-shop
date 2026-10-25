package api

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"shop/internal/model"
)

func listSupport(db *gorm.DB, staff bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !dbRequired(c, db) {
			return
		}
		page, size := parsePagination(c)
		q := db.Model(&model.SupportTicket{})
		if !staff {
			q = q.Where("user_id = ?", currentUser(c))
		} else {
			switch c.Query("mode") {
			case "", "queue":
				q = q.Where("mode IN ?", []string{model.SupportModeWaiting, model.SupportModeHuman})
			case model.SupportModeBot, model.SupportModeWaiting, model.SupportModeHuman:
				q = q.Where("mode = ?", c.Query("mode"))
			case "all":
			default:
				c.JSON(400, gin.H{"message": "无效的客服接待方式"})
				return
			}
		}
		if status := c.Query("status"); status == "open" || status == "closed" {
			q = q.Where("status = ?", status)
		}
		var total int64
		items := []model.SupportTicket{}
		if err := q.Count(&total).Error; err != nil {
			c.JSON(500, gin.H{"message": "查询失败"})
			return
		}
		if err := q.Order("updated_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
			c.JSON(500, gin.H{"message": "查询失败"})
			return
		}
		c.JSON(200, gin.H{"items": items, "total": total})
	}
}

func createSupport(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !dbRequired(c, db) {
			return
		}
		var req struct {
			Subject string `json:"subject"`
			Content string `json:"content"`
		}
		if c.ShouldBindJSON(&req) != nil {
			c.JSON(400, gin.H{"message": "请填写咨询内容"})
			return
		}
		req.Subject, req.Content = strings.TrimSpace(req.Subject), strings.TrimSpace(req.Content)
		if req.Subject == "" {
			title := []rune(strings.Join(strings.Fields(req.Content), " "))
			if len(title) > 40 {
				req.Subject = string(title[:40]) + "…"
			} else {
				req.Subject = string(title)
			}
		}
		if req.Subject == "" || len([]rune(req.Subject)) > 100 || req.Content == "" || len([]rune(req.Content)) > 2000 {
			c.JSON(400, gin.H{"message": "标题需为 1-100 字，内容需为 1-2000 字"})
			return
		}
		ticket := model.SupportTicket{UserID: currentUser(c), Subject: req.Subject, Status: "open", Mode: model.SupportModeBot}
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&ticket).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.SupportMessage{TicketID: ticket.ID, Content: req.Content}).Error; err != nil {
				return err
			}
			if err := appendSupportAutoReply(tx, &ticket, req.Content); err != nil {
				return err
			}
			return tx.Model(&ticket).Updates(map[string]any{"mode": ticket.Mode, "human_requested_at": ticket.HumanRequestedAt}).Error
		})
		if err != nil {
			c.JSON(500, gin.H{"message": "提交失败，请稍后重试"})
			return
		}
		c.JSON(200, ticket)
	}
}

func supportTicket(db *gorm.DB, staff bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := lifecycleID(c, db)
		if !ok {
			return
		}
		q := db.Where("id = ?", id)
		if !staff {
			q = q.Where("user_id = ?", currentUser(c))
		}
		var ticket model.SupportTicket
		if err := q.First(&ticket).Error; err != nil {
			c.JSON(404, gin.H{"message": "咨询不存在"})
			return
		}
		messages := []model.SupportMessage{}
		// Conversations are small and bounded by the message limit on replies.
		if err := db.Where("ticket_id = ?", id).Order("id ASC").Find(&messages).Error; err != nil {
			c.JSON(500, gin.H{"message": "读取消息失败"})
			return
		}
		c.JSON(200, gin.H{"ticket": ticket, "messages": messages})
	}
}

func replySupport(db *gorm.DB, hub *notificationHub, staff bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := lifecycleID(c, db)
		if !ok {
			return
		}
		var req struct {
			Content string `json:"content"`
			Close   bool   `json:"close"`
		}
		if c.ShouldBindJSON(&req) != nil {
			c.JSON(400, gin.H{"message": "请填写回复"})
			return
		}
		req.Content = strings.TrimSpace(req.Content)
		if len([]rune(req.Content)) > 2000 || (req.Content == "" && !req.Close) {
			c.JSON(400, gin.H{"message": "回复需为 1-2000 字"})
			return
		}
		var ticket model.SupportTicket
		err := db.Transaction(func(tx *gorm.DB) error {
			q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id)
			if !staff {
				q = q.Where("user_id = ?", currentUser(c))
			}
			if err := q.First(&ticket).Error; err != nil {
				return err
			}
			if ticket.Status != "open" {
				return fmt.Errorf("咨询已结束，请重新发起咨询")
			}
			if staff && ticket.Mode == model.SupportModeBot {
				return fmt.Errorf("用户尚未请求人工客服，当前咨询由自动客服接待")
			}
			if req.Content != "" {
				var count int64
				if err := tx.Model(&model.SupportMessage{}).Where("ticket_id = ?", id).Count(&count).Error; err != nil {
					return err
				}
				needed := int64(1)
				if !staff && ticket.Mode == model.SupportModeBot && !req.Close {
					needed++
				}
				if count+needed > 200 {
					return fmt.Errorf("本次咨询消息已达上限，请结束后重新发起")
				}
				if err := tx.Create(&model.SupportMessage{TicketID: id, Staff: staff, Content: req.Content}).Error; err != nil {
					return err
				}
				if staff {
					ticket.Mode = model.SupportModeHuman
				} else if !req.Close {
					if err := appendSupportAutoReply(tx, &ticket, req.Content); err != nil {
						return err
					}
				}
			}
			if req.Close {
				ticket.Status = "closed"
			}
			return tx.Model(&ticket).Updates(map[string]any{"status": ticket.Status, "mode": ticket.Mode, "human_requested_at": ticket.HumanRequestedAt, "updated_at": time.Now()}).Error
		})
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(404, gin.H{"message": "咨询不存在"})
			return
		}
		if err != nil {
			c.JSON(400, gin.H{"message": err.Error()})
			return
		}
		if staff {
			notify(db, hub, ticket.UserID, model.NotificationTypeSupport, "客服已回复", "您的咨询「"+ticket.Subject+"」有新进展，请前往客服与帮助查看", "")
		}
		c.JSON(200, gin.H{"message": "已提交"})
	}
}
