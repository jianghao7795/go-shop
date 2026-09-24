package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

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

type categoryPayload struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Icon string `json:"icon"`
	Note string `json:"note"`
}

func validateCategory(p categoryPayload) string {
	if strings.TrimSpace(p.Key) == "" || strings.TrimSpace(p.Name) == "" {
		return "分类 key 和名称不能为空"
	}
	return ""
}

func adminListCategories(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		var items []model.Category
		if err := db.Order("id DESC").Find(&items).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "查询失败"})
			return
		}
		c.JSON(http.StatusOK, items)
	}
}

func adminCreateCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var p categoryPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请求格式错误"})
			return
		}
		if msg := validateCategory(p); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": msg})
			return
		}
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		var existing model.Category
		if err := db.Where("key = ?", p.Key).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"message": "分类 key 已存在"})
			return
		}
		item := model.Category{Key: p.Key, Name: p.Name, Icon: p.Icon, Note: p.Note}
		if err := db.Create(&item).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "创建失败"})
			return
		}
		c.JSON(http.StatusOK, item)
	}
}

func adminUpdateCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "非法分类 ID"})
			return
		}
		var p categoryPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请求格式错误"})
			return
		}
		if msg := validateCategory(p); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": msg})
			return
		}
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		var existing model.Category
		if err := db.Where("key = ? AND id != ?", p.Key, id).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"message": "分类 key 已存在"})
			return
		}
		updates := map[string]any{"key": p.Key, "name": p.Name, "icon": p.Icon, "note": p.Note}
		if err := db.Model(&model.Category{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "更新失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
	}
}

func adminDeleteCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "非法分类 ID"})
			return
		}
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		if err := db.Delete(&model.Category{}, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "删除失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
	}
}

// adminListOrders 返回全部订单（可选 ?status= 过滤，无用户隔离）。
func adminListOrders(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		status := c.Query("status")
		q := db.Model(&model.Order{})
		if status != "" {
			q = q.Where("status = ?", status)
		}
		var list []model.Order
		if err := q.Order("id DESC").Find(&list).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "查询失败"})
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

// adminGetOrder 按 id 查询单个订单（无用户隔离）。
func adminGetOrder(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "无效的订单"})
			return
		}
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		var order model.Order
		if err := db.Where("id = ?", id).First(&order).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "订单不存在"})
			return
		}
		c.JSON(http.StatusOK, order)
	}
}

// adminUpdateOrderStatus 管理员更新任意订单状态，并通知订单所属用户。
func adminUpdateOrderStatus(db *gorm.DB, hub *notificationHub) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "无效的订单"})
			return
		}
		var req struct {
			Status string `json:"status" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "非法状态"})
			return
		}
		switch req.Status {
		case model.OrderStatusPending, model.OrderStatusShipped, model.OrderStatusCompleted, model.OrderStatusAftersale, model.OrderStatusFinished:
		default:
			c.JSON(http.StatusBadRequest, gin.H{"message": "非法状态"})
			return
		}
		var order model.Order
		if err := db.Where("id = ?", id).First(&order).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "订单不存在"})
			return
		}
		if err := db.Model(&model.Order{}).Where("id = ?", id).Update("status", req.Status).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "更新失败"})
			return
		}
		text := statusText[req.Status]
		if text == "" {
			text = req.Status
		}
		notify(db, hub, order.UserID, model.NotificationTypeOrder, "订单状态更新",
			"您的订单 "+order.OrderNo+" 已更新为 "+text, order.OrderNo)
		c.JSON(http.StatusOK, gin.H{"message": "已更新"})
	}
}

type couponPayload struct {
	Title     string `json:"title"`
	Amount    int    `json:"amount"`
	MinAmount int    `json:"minAmount"`
	Condition string `json:"condition"`
	StartAt   string `json:"startAt"` // 2006-01-02 15:04:05
	EndAt     string `json:"endAt"`
}

func validateCoupon(p couponPayload) string {
	if strings.TrimSpace(p.Title) == "" {
		return "券标题不能为空"
	}
	if p.Amount <= 0 || p.MinAmount < 0 {
		return "金额不合法"
	}
	return ""
}

// parseCouponPeriod 解析优惠券有效期；两个时间都为空时使用默认有效期。
func parseCouponPeriod(p couponPayload) (time.Time, time.Time, bool) {
	if strings.TrimSpace(p.StartAt) == "" && strings.TrimSpace(p.EndAt) == "" {
		start, end := model.DefaultCouponPeriod()
		return start, end, true
	}
	if strings.TrimSpace(p.StartAt) == "" || strings.TrimSpace(p.EndAt) == "" {
		return time.Time{}, time.Time{}, false
	}
	startAt, err := time.Parse("2006-01-02 15:04:05", p.StartAt)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	endAt, err := time.Parse("2006-01-02 15:04:05", p.EndAt)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	if !startAt.Before(endAt) {
		return time.Time{}, time.Time{}, false
	}
	return startAt, endAt, true
}

func adminListCoupons(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		var items []model.Coupon
		if err := db.Order("id DESC").Find(&items).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "查询失败"})
			return
		}
		c.JSON(http.StatusOK, items)
	}
}

func adminCreateCoupon(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var p couponPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请求格式错误"})
			return
		}
		if msg := validateCoupon(p); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": msg})
			return
		}
		startAt, endAt, ok := parseCouponPeriod(p)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"message": "有效期不合法"})
			return
		}
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		item := model.Coupon{Title: p.Title, Amount: p.Amount, MinAmount: p.MinAmount, Condition: p.Condition, StartAt: startAt, EndAt: endAt}
		if err := db.Create(&item).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "创建失败"})
			return
		}
		c.JSON(http.StatusOK, item)
	}
}

func adminUpdateCoupon(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "非法优惠券 ID"})
			return
		}
		var p couponPayload
		if err := c.ShouldBindJSON(&p); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "请求格式错误"})
			return
		}
		if msg := validateCoupon(p); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": msg})
			return
		}
		startAt, endAt, ok := parseCouponPeriod(p)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"message": "有效期不合法"})
			return
		}
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		updates := map[string]any{"title": p.Title, "amount": p.Amount, "min_amount": p.MinAmount, "condition": p.Condition, "start_at": startAt, "end_at": endAt}
		if err := db.Model(&model.Coupon{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "更新失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
	}
}

func adminDeleteCoupon(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "非法优惠券 ID"})
			return
		}
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		if err := db.Delete(&model.Coupon{}, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "删除失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
	}
}

// adminSendNotification 给指定用户（或全体）发送站内通知。
func adminSendNotification(db *gorm.DB, hub *notificationHub) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username"` // 空 = 广播给所有人
			Title    string `json:"title"`
			Content  string `json:"content"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "标题和内容不能为空"})
			return
		}
		if db == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"message": "数据库不可用"})
			return
		}
		if req.Username != "" {
			notify(db, hub, req.Username, model.NotificationTypeCoupon, req.Title, req.Content, "")
		} else {
			var users []model.User
			if err := db.Find(&users).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "查询失败"})
				return
			}
			for _, u := range users {
				notify(db, hub, u.Username, model.NotificationTypeCoupon, req.Title, req.Content, "")
			}
		}
		c.JSON(http.StatusOK, gin.H{"message": "已发送"})
	}
}
