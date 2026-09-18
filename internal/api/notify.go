package api

import (
	"log"
	"sync"

	"gorm.io/gorm"

	"shop/internal/model"
)

// statusText 是订单状态到中文文案的映射，与前端 OrderListView 的 statusMap 保持一致。
var statusText = map[string]string{
	model.OrderStatusPending:   "待付款",
	model.OrderStatusShipped:   "待收货",
	model.OrderStatusCompleted: "待评价",
	model.OrderStatusAftersale: "售后",
	model.OrderStatusFinished:  "已完成",
}

// notificationHub 维护每个用户到其订阅通道集合的映射，用于把通知实时推给在线客户端。
type notificationHub struct {
	mu      sync.Mutex
	clients map[string]map[chan model.Notification]struct{}
}

func newNotificationHub() *notificationHub {
	return &notificationHub{clients: make(map[string]map[chan model.Notification]struct{})}
}

func (h *notificationHub) subscribe(userID string) chan model.Notification {
	ch := make(chan model.Notification, 8)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[chan model.Notification]struct{})
	}
	h.clients[userID][ch] = struct{}{}
	return ch
}

func (h *notificationHub) unsubscribe(userID string, ch chan model.Notification) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set := h.clients[userID]; set != nil {
		delete(set, ch)
		if len(set) == 0 {
			delete(h.clients, userID)
		}
	}
}

func (h *notificationHub) broadcast(userID string, n model.Notification) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients[userID] {
		select {
		case ch <- n:
		default: // 订阅者缓冲区满，丢弃本次推送，避免阻塞
		}
	}
}

// onlineUsers 返回当前有 SSE 连接的在线用户名列表。
func (h *notificationHub) onlineUsers() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	users := make([]string, 0, len(h.clients))
	for u := range h.clients {
		users = append(users, u)
	}
	return users
}

// notify 落库并实时广播一条通知；db 为 nil 时仅广播、不落库。
func notify(db *gorm.DB, hub *notificationHub, userID, typ, title, content, orderNo string) {
	n := model.Notification{
		UserID:  userID,
		Type:    typ,
		Title:   title,
		Content: content,
		OrderNo: orderNo,
	}
	if db != nil {
		if err := db.Create(&n).Error; err != nil {
			log.Printf("notification create failed: %v", err)
		}
	}
	hub.broadcast(userID, n)
}
