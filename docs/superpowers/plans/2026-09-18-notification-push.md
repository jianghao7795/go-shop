# 实时通知系统（订单 + 优惠券）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让服务器通过 SSE 主动推送订单状态变动与优惠券通知到客户端，并驱动「我的」tab 圆点（未读 > 0 时显示）。

**Architecture:** 在现有 Gin HTTP API（`internal/api`）上新增一个通知子系统：`notificationHub` 按 UserID 维护 SSE 订阅通道；通知落库到新表 `shop_notifications` 并实时广播；后台两个模拟器（商家发货 / 营销发券）触发真正的服务器主动推送。前端新增 Pinia store 用 `EventSource` 订阅，未读数驱动 tab 圆点，并提供通知列表页。

**Tech Stack:** Go + Gin + GORM（后端），Vue 3 + Pinia + Vant 4（前端），SSE（Server-Sent Events）。

**Spec:** `docs/superpowers/specs/2026-09-18-notification-push-design.md`

## Global Constraints

- Go 模块名 `shop`，工作目录 `C:\Users\jianghao\man\demo`（仓库根目录）。
- 通知 UserID = 登录用户名（`currentUser(c)` 返回的 username）。
- 通知类型常量：`order`、`coupon`（定义在 `model.NotificationType*`）。
- 订单状态文案与前端 `OrderListView.statusMap` 一致：`pending→待付款`、`shipped→待收货`、`completed→待评价`、`aftersale→售后`、`finished→已完成`。
- 前端 API 地址统一用 `import.meta.env.VITE_API_BASE || "http://127.0.0.1:8080"`。
- 前端 token 存于 `localStorage` 的 `shop_token` 键（与 `stores/user.ts` 一致）。
- 本目录当前**没有 git 仓库**（`git status` 报 `not a git repository`）。各任务的 Commit 步骤在 git 不可用时跳过；如需版本控制先运行 `git init`。

---

### Task 1: 通知模型 + 通知中心（hub / notify / statusText）

**Files:**
- Create: `internal/model/notification.go`
- Create: `internal/api/notify.go`
- Test: `internal/api/notify_test.go`

**Interfaces:**
- Consumes: 无（首个任务）。
- Produces:
  - `model.Notification` 结构体及常量 `NotificationTypeOrder = "order"`、`NotificationTypeCoupon = "coupon"`，方法 `TableName() string`（返回 `shop_notifications`）。
  - `newNotificationHub() *notificationHub`、`(*notificationHub).subscribe(userID string) chan model.Notification`、`.unsubscribe(userID, ch)`、`.broadcast(userID string, n model.Notification)`。
  - `notify(db *gorm.DB, hub *notificationHub, userID, typ, title, content, orderNo string)`。
  - `statusText map[string]string`（订单状态→中文文案）。

- [ ] **Step 1: 写失败测试**

Create `internal/api/notify_test.go`:

```go
package api

import (
	"testing"
	"time"

	"shop/internal/model"
)

func TestHubBroadcastDeliversToSubscriber(t *testing.T) {
	hub := newNotificationHub()
	ch := hub.subscribe("alice")
	defer hub.unsubscribe("alice", ch)

	hub.broadcast("alice", model.Notification{Title: "hi"})

	select {
	case got := <-ch:
		if got.Title != "hi" {
			t.Fatalf("unexpected title %q", got.Title)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broadcast")
	}
}

func TestHubBroadcastDoesNotCrossUsers(t *testing.T) {
	hub := newNotificationHub()
	alice := hub.subscribe("alice")
	defer hub.unsubscribe("alice", alice)
	bob := hub.subscribe("bob")
	defer hub.unsubscribe("bob", bob)

	hub.broadcast("alice", model.Notification{Title: "a"})

	select {
	case got := <-alice:
		if got.Title != "a" {
			t.Fatalf("unexpected title %q", got.Title)
		}
	case <-time.After(time.Second):
		t.Fatal("alice should receive")
	}
	select {
	case <-bob:
		t.Fatal("bob should not receive alice's notification")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestNotifyBroadcastsWithoutDB(t *testing.T) {
	hub := newNotificationHub()
	ch := hub.subscribe("alice")
	defer hub.unsubscribe("alice", ch)

	notify(nil, hub, "alice", model.NotificationTypeOrder, "订单已发货", "内容", "NO123")

	select {
	case got := <-ch:
		if got.Type != model.NotificationTypeOrder || got.OrderNo != "NO123" {
			t.Fatalf("unexpected notification %+v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for notify broadcast")
	}
}

func TestStatusTextCoversAllStatuses(t *testing.T) {
	for _, s := range []string{
		model.OrderStatusPending,
		model.OrderStatusShipped,
		model.OrderStatusCompleted,
		model.OrderStatusAftersale,
		model.OrderStatusFinished,
	} {
		if statusText[s] == "" {
			t.Fatalf("missing status text for %q", s)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/api/ -run 'TestHub|TestNotify|TestStatusText' -v`
Expected: 编译失败（`newNotificationHub`、`notify`、`statusText` 未定义）。

- [ ] **Step 3: 写最小实现**

Create `internal/model/notification.go`:

```go
package model

import "time"

// 通知类型。
const (
	NotificationTypeOrder  = "order"  // 订单变动
	NotificationTypeCoupon = "coupon" // 优惠券
)

// Notification 是推送给用户的站内通知。
type Notification struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    string    `json:"userId" gorm:"index;size:64"`
	Type      string    `json:"type" gorm:"size:20"`
	Title     string    `json:"title" gorm:"size:128"`
	Content   string    `json:"content" gorm:"size:255"`
	OrderNo   string    `json:"orderNo" gorm:"size:64"`
	Read      bool      `json:"read" gorm:"default:false"`
	CreatedAt time.Time `json:"createdAt"`
}

// TableName 使用独立的 shop_notifications 表。
func (Notification) TableName() string { return "shop_notifications" }
```

Create `internal/api/notify.go`:

```go
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
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/api/ -run 'TestHub|TestNotify|TestStatusText' -v`
Expected: 4 个测试全部 PASS。

- [ ] **Step 5: Commit**

```bash
git add internal/model/notification.go internal/api/notify.go internal/api/notify_test.go
git commit -m "feat: add notification model and in-memory hub"
```
（git 不可用时跳过本步）

---

### Task 2: SSE 端点 + 通知 REST 处理器 + 路由注册

**Files:**
- Create: `internal/api/notify_handlers.go`
- Modify: `internal/api/server.go`
- Test: `internal/api/notify_handlers_test.go`

**Interfaces:**
- Consumes: Task 1 的 `notificationHub`（`subscribe`/`unsubscribe`）、`currentUser(c)`（已存在于 `handlers_address.go`）、`model.Notification`。
- Produces:
  - `tokenFromQuery() gin.HandlerFunc`
  - `streamNotifications(hub *notificationHub) gin.HandlerFunc`
  - `listNotifications(db *gorm.DB) gin.HandlerFunc`
  - `unreadCount(db *gorm.DB) gin.HandlerFunc`
  - `markNotificationRead(db *gorm.DB) gin.HandlerFunc`
  - `markAllNotificationsRead(db *gorm.DB) gin.HandlerFunc`

- [ ] **Step 1: 写失败测试（tokenFromQuery）**

Create `internal/api/notify_handlers_test.go`:

```go
package api

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTokenFromQuerySetsAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/notifications/stream?token=abc.def.ghi", nil)

	tokenFromQuery()(c)

	if got := c.Request.Header.Get("Authorization"); got != "Bearer abc.def.ghi" {
		t.Fatalf("Authorization = %q", got)
	}
}

func TestTokenFromQueryKeepsExistingAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/notifications/stream?token=xyz", nil)
	c.Request.Header.Set("Authorization", "Bearer existing")

	tokenFromQuery()(c)

	if got := c.Request.Header.Get("Authorization"); got != "Bearer existing" {
		t.Fatalf("Authorization = %q", got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/api/ -run TestTokenFromQuery -v`
Expected: 编译失败（`tokenFromQuery` 未定义）。

- [ ] **Step 3: 写实现 —— 处理器**

Create `internal/api/notify_handlers.go`:

```go
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
```

- [ ] **Step 4: 写实现 —— server.go 注册路由**

Modify `internal/api/server.go`，做三处改动：

(a) AutoMigrate 增加 `model.Notification`。把：

```go
		} else if migrateErr := db.AutoMigrate(&model.Product{}, &model.User{}, &model.Address{}, &model.Order{}); migrateErr != nil {
```

改为：

```go
		} else if migrateErr := db.AutoMigrate(&model.Product{}, &model.User{}, &model.Address{}, &model.Order{}, &model.Notification{}); migrateErr != nil {
```

(b) 在 `jwtMiddleware, err := auth.New(db)` 之后、`router.POST("/api/login", ...)` 之前，新增 hub 并注册 SSE 路由。在：

```go
	jwtMiddleware, err := auth.New(db)
	if err != nil {
		log.Fatalf("auth init failed: %v", err)
	}
```

之后插入：

```go
	hub := newNotificationHub()
	router.GET("/api/notifications/stream", tokenFromQuery(), jwtMiddleware.MiddlewareFunc(), streamNotifications(hub))
```

(c) 在 `protected.GET("/coupons", listCoupons())` 之后，新增通知 REST 路由：

```go
	protected.GET("/notifications", listNotifications(db))
	protected.GET("/notifications/unread", unreadCount(db))
	protected.PUT("/notifications/read-all", markAllNotificationsRead(db))
	protected.PUT("/notifications/:id/read", markNotificationRead(db))
```

- [ ] **Step 5: 运行测试 + 构建**

Run:
```
go test ./internal/api/ -run TestTokenFromQuery -v
go build ./...
```
Expected: 测试 PASS，`go build` 无输出（成功）。

- [ ] **Step 6: Commit**

```bash
git add internal/api/notify_handlers.go internal/api/notify_handlers_test.go internal/api/server.go
git commit -m "feat: add SSE stream and notification REST endpoints"
```
（git 不可用时跳过本步）

---

### Task 3: 订单状态变化触发通知

**Files:**
- Modify: `internal/api/handlers_order.go`（`updateOrderStatus` 签名与逻辑）
- Modify: `internal/api/server.go`（`updateOrderStatus(db)` → `updateOrderStatus(db, hub)`）

**Interfaces:**
- Consumes: Task 1 的 `notify`、`statusText`、`model.NotificationTypeOrder`；Task 2 的 `hub`。
- Produces: 无新符号（仅改内部逻辑）。

- [ ] **Step 1: 改写 `updateOrderStatus`**

Modify `internal/api/handlers_order.go`，把 `updateOrderStatus` 整个函数替换为：

```go
func updateOrderStatus(db *gorm.DB, hub *notificationHub) gin.HandlerFunc {
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
			c.JSON(http.StatusBadRequest, gin.H{"message": "无效的状态"})
			return
		}
		var order model.Order
		if db.Where("id = ? AND user_id = ?", id, currentUser(c)).First(&order).Error != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "订单不存在"})
			return
		}
		db.Model(&model.Order{}).Where("id = ? AND user_id = ?", id, currentUser(c)).Update("status", req.Status)

		text := statusText[req.Status]
		if text == "" {
			text = req.Status
		}
		notify(db, hub, currentUser(c), model.NotificationTypeOrder, "订单状态更新",
			"您的订单 "+order.OrderNo+" 已更新为 "+text, order.OrderNo)
		c.JSON(http.StatusOK, gin.H{"message": "已更新"})
	}
}
```

（`strconv`、`model`、`currentUser`、`notify`、`statusText` 均已在本包或已 import；无需新增 import。）

- [ ] **Step 2: 更新路由注册**

Modify `internal/api/server.go`，把：

```go
	protected.PUT("/orders/:id/status", updateOrderStatus(db))
```

改为：

```go
	protected.PUT("/orders/:id/status", updateOrderStatus(db, hub))
```

- [ ] **Step 3: 构建**

Run: `go build ./...`
Expected: 无输出（成功）。

- [ ] **Step 4: Commit**

```bash
git add internal/api/handlers_order.go internal/api/server.go
git commit -m "feat: push notification on order status change"
```
（git 不可用时跳过本步）

---

### Task 4: 模拟商家发货 + 优惠券营销推送

**Files:**
- Modify: `internal/api/handlers_coupon.go`（把优惠券数据抽成包级变量）
- Create: `internal/api/simulators.go`
- Modify: `internal/api/server.go`（启动模拟器 goroutine）

**Interfaces:**
- Consumes: Task 1 的 `notify`、`onlineUsers`、`model.NotificationTypeCoupon`；`coupons` 包级变量（本任务从 `handlers_coupon.go` 抽出）。
- Produces: `startSimulators(db *gorm.DB, hub *notificationHub)`。

- [ ] **Step 1: 抽取优惠券数据**

Modify `internal/api/handlers_coupon.go`，把 `coupon` 类型和券列表提到函数外、改为包级变量，并让 `listCoupons` 返回该变量：

```go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// coupon 是优惠券（当前为静态演示数据）。
type coupon struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Amount    int    `json:"amount"`
	Condition string `json:"condition"`
}

// coupons 是内置优惠券列表，供列表接口与营销模拟器共用。
var coupons = []coupon{
	{ID: 1, Title: "新人专享券", Amount: 10, Condition: "满 99 元可用"},
	{ID: 2, Title: "全场通用券", Amount: 20, Condition: "满 199 元可用"},
	{ID: 3, Title: "数码品类券", Amount: 50, Condition: "满 499 元可用"},
	{ID: 4, Title: "满减优惠券", Amount: 100, Condition: "满 999 元可用"},
}

func listCoupons() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, coupons)
	}
}
```

- [ ] **Step 2: 写模拟器**

Create `internal/api/simulators.go`:

```go
package api

import (
	"log"
	"time"

	"gorm.io/gorm"

	"shop/internal/model"
)

// startSimulators 启动两个后台模拟器，用于演示服务器主动推送：
// 1) 模拟商家定时发货：把最早的待付款订单改为待收货；
// 2) 模拟营销：定时向在线用户推送优惠券通知。
func startSimulators(db *gorm.DB, hub *notificationHub) {
	if db == nil {
		return
	}
	go func() {
		shipTicker := time.NewTicker(20 * time.Second)
		couponTicker := time.NewTicker(90 * time.Second)
		couponIndex := 0
		for {
			select {
			case <-shipTicker.C:
				shipNextOrder(db, hub)
			case <-couponTicker.C:
				if len(coupons) == 0 {
					continue
				}
				pushCouponToOnline(db, hub, coupons[couponIndex%len(coupons)])
				couponIndex++
			}
		}
	}()
}

func shipNextOrder(db *gorm.DB, hub *notificationHub) {
	var order model.Order
	if db.Where("status = ?", model.OrderStatusPending).Order("id ASC").First(&order).Error != nil {
		return // 没有待付款订单，跳过
	}
	db.Model(&model.Order{}).Where("id = ?", order.ID).Update("status", model.OrderStatusShipped)
	notify(db, hub, order.UserID, model.NotificationTypeOrder, "订单已发货",
		"您的订单 "+order.OrderNo+" 已发货，请注意查收", order.OrderNo)
	log.Printf("simulated merchant shipped order %s", order.OrderNo)
}

func pushCouponToOnline(db *gorm.DB, hub *notificationHub, cp coupon) {
	for _, userID := range hub.onlineUsers() {
		notify(db, hub, userID, model.NotificationTypeCoupon, cp.Title+"已到账",
			cp.Title+"（"+cp.Condition+"）已发放到您的账户", "")
	}
}
```

- [ ] **Step 3: 启动模拟器**

Modify `internal/api/server.go`，在 `port := os.Getenv("API_PORT")` 之前插入：

```go
	go startSimulators(db, hub)
```

（放在 `router.Run` 之前、路由注册之后，确保 db/hub 已就绪且不影响阻塞的 Run。）

- [ ] **Step 4: 构建 + 静态检查**

Run:
```
go build ./...
go vet ./...
```
Expected: 均无输出（成功）。

- [ ] **Step 5: 手动验证（需运行中的后端与 MySQL）**

启动后端后：登录 → 下单（产生一条 `pending` 订单）→ 等 ~20s，观察日志出现 `simulated merchant shipped order ...`，前端收到"订单已发货"推送。

- [ ] **Step 6: Commit**

```bash
git add internal/api/handlers_coupon.go internal/api/simulators.go internal/api/server.go
git commit -m "feat: add merchant shipping and coupon marketing simulators"
```
（git 不可用时跳过本步）

---

### Task 5: 前端通知 store

**Files:**
- Create: `frontend/src/stores/notification.ts`

**Interfaces:**
- Consumes: `stores/user.ts` 的 `useUserStore`（`token`）；`vant` 的 `showToast`。
- Produces:
  - `AppNotification` 类型（`id, userId, type, title, content, orderNo, read, createdAt`）。
  - `useNotificationStore`（Pinia），暴露：`items`、`unread`、`fetchList()`、`fetchUnread()`、`markRead(id)`、`markAllRead()`、`connect()`、`disconnect()`。

- [ ] **Step 1: 写 store**

Create `frontend/src/stores/notification.ts`:

```ts
import { ref } from "vue";
import { defineStore } from "pinia";
import { showToast } from "vant";
import { useUserStore } from "./user";

export interface AppNotification {
  id: number;
  userId: string;
  type: "order" | "coupon";
  title: string;
  content: string;
  orderNo: string;
  read: boolean;
  createdAt: string;
}

const apiBase = () => import.meta.env.VITE_API_BASE || "http://127.0.0.1:8080";

export const useNotificationStore = defineStore("notification", () => {
  const userStore = useUserStore();
  const items = ref<AppNotification[]>([]);
  const unread = ref(0);
  let source: EventSource | null = null;

  const authHeaders = () => ({ Authorization: "Bearer " + userStore.token });

  async function fetchList() {
    const response = await fetch(apiBase() + "/api/notifications", { headers: authHeaders() });
    if (response.ok) items.value = await response.json();
  }

  async function fetchUnread() {
    const response = await fetch(apiBase() + "/api/notifications/unread", { headers: authHeaders() });
    if (response.ok) {
      const data = await response.json();
      unread.value = data.count || 0;
    }
  }

  async function markRead(id: number) {
    await fetch(apiBase() + "/api/notifications/" + id + "/read", {
      method: "PUT",
      headers: authHeaders(),
    });
    const item = items.value.find(i => i.id === id);
    if (item && !item.read) {
      item.read = true;
      unread.value = Math.max(0, unread.value - 1);
    }
  }

  async function markAllRead() {
    await fetch(apiBase() + "/api/notifications/read-all", {
      method: "PUT",
      headers: authHeaders(),
    });
    items.value.forEach(i => { i.read = true; });
    unread.value = 0;
  }

  function connect() {
    if (!userStore.token || source) return;
    source = new EventSource(apiBase() + "/api/notifications/stream?token=" + encodeURIComponent(userStore.token));
    source.addEventListener("notification", (e: MessageEvent) => {
      try {
        const n = JSON.parse(e.data) as AppNotification;
        items.value.unshift(n);
        unread.value += 1;
        showToast(n.title);
      } catch { /* 忽略无法解析的数据 */ }
    });
    source.onopen = () => { fetchUnread(); };
  }

  function disconnect() {
    if (source) {
      source.close();
      source = null;
    }
  }

  return { items, unread, fetchList, fetchUnread, markRead, markAllRead, connect, disconnect };
});
```

- [ ] **Step 2: 构建前端**

Run: `cd frontend && npm run build`
Expected: 构建成功（Vite 无报错）。注意：`vite build` 不做类型检查，逻辑正确性靠后续手动验证。

- [ ] **Step 3: Commit**

```bash
git add frontend/src/stores/notification.ts
git commit -m "feat: add frontend notification store with SSE"
```
（git 不可用时跳过本步）

---

### Task 6: 通知列表页 + 路由 + Profile 入口

**Files:**
- Create: `frontend/src/views/NotificationsView.vue`
- Modify: `frontend/src/router/index.ts`
- Modify: `frontend/src/views/ProfileView.vue`
- Modify: `frontend/src/main.ts`（注册 `Badge`）

**Interfaces:**
- Consumes: Task 5 的 `useNotificationStore`（`items`、`unread`、`fetchList`、`fetchUnread`、`markRead`、`markAllRead`）。
- Produces: 路由 `notifications`（`/notifications`）。

- [ ] **Step 1: 写通知列表页**

Create `frontend/src/views/NotificationsView.vue`:

```vue
<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useNotificationStore } from "../stores/notification";

const store = useNotificationStore();
const loading = ref(false);

const typeEmoji: Record<string, string> = {
  order: "📦",
  coupon: "🎟️",
};

async function load() {
  loading.value = true;
  await store.fetchList();
  await store.fetchUnread();
  loading.value = false;
}

onMounted(load);
</script>

<template>
  <div class="sub-page">
    <van-nav-bar title="消息通知" left-arrow @click-left="$router.back()" />
    <div class="notify-toolbar">
      <van-button v-if="store.unread > 0" size="small" round plain type="danger" @click="store.markAllRead()">全部已读</van-button>
    </div>
    <div v-if="loading" class="loading"><van-loading color="#ff4d67" /></div>
    <van-empty v-else-if="!store.items.length" description="暂无消息" />
    <van-cell-group v-else inset>
      <van-cell v-for="n in store.items" :key="n.id" class="notify-cell" :class="{ 'is-unread': !n.read }" @click="store.markRead(n.id)">
        <template #icon>
          <span class="notify-emoji">{{ typeEmoji[n.type] || "🔔" }}</span>
        </template>
        <template #title>
          <span class="notify-title">{{ n.title }}</span>
          <span v-if="!n.read" class="notify-dot" />
        </template>
        <template #label>{{ n.content }}</template>
        <template #value>
          <span class="notify-time">{{ n.createdAt.slice(5, 16).replace("T", " ") }}</span>
        </template>
      </van-cell>
    </van-cell-group>
  </div>
</template>
```

- [ ] **Step 2: 加路由**

Modify `frontend/src/router/index.ts`：在 import 区加：

```ts
import NotificationsView from "../views/NotificationsView.vue";
```

在 routes 数组 `{ path: "/service", ... }` 之后加：

```ts
    { path: "/notifications", name: "notifications", component: NotificationsView, meta: { title: "消息通知", hideTabbar: true, requiresAuth: true } },
```

- [ ] **Step 3: Profile 入口**

Modify `frontend/src/views/ProfileView.vue`：

(a) 在 `<script setup>` 中引入 store（在 `const userStore = ...` 之后）：

```ts
import { useNotificationStore } from "../stores/notification";
```
并在 `const userStore = useUserStore();` 之后加：

```ts
const notificationStore = useNotificationStore();
```

(b) 在 `profile-tools` 的 `van-cell-group` 内、`客服与帮助` cell 之前加：

```vue
      <van-cell title="消息通知" icon="bell" is-link @click="router.push('/notifications')">
        <template #value>
          <van-badge v-if="notificationStore.unread > 0" :content="notificationStore.unread" />
        </template>
      </van-cell>
```

- [ ] **Step 4: 注册 Badge 组件**

Modify `frontend/src/main.ts`：把 `Badge` 加入 import 与 `app.use`。将 import 行：

```ts
import { ActionBar, ActionBarButton, ActionBarIcon, Button, Card, Cascader, Cell, CellGroup, Checkbox, Empty, Field, Form, Grid, GridItem, Loading, NavBar, Popup, Search, Sidebar, SidebarItem, Stepper, Swipe, SwipeCell, SwipeItem, Tab, Tabbar, TabbarItem, Tabs } from "vant";
```

改为：

```ts
import { ActionBar, ActionBarButton, ActionBarIcon, Badge, Button, Card, Cascader, Cell, CellGroup, Checkbox, Empty, Field, Form, Grid, GridItem, Loading, NavBar, Popup, Search, Sidebar, SidebarItem, Stepper, Swipe, SwipeCell, SwipeItem, Tab, Tabbar, TabbarItem, Tabs } from "vant";
```

将 `app.use(ActionBar).use(ActionBarButton).use(ActionBarIcon).use(Button)...` 的链中，在 `.use(ActionBarIcon)` 之后插入 `.use(Badge)`：

```ts
app.use(ActionBar).use(ActionBarButton).use(ActionBarIcon).use(Badge).use(Button).use(Card).use(Cascader).use(Cell).use(CellGroup).use(Checkbox).use(Empty).use(Field).use(Form).use(Grid).use(GridItem).use(Loading).use(NavBar).use(Popup).use(Search).use(Sidebar).use(SidebarItem).use(Stepper).use(Swipe).use(SwipeCell).use(SwipeItem).use(Tab).use(Tabbar).use(TabbarItem).use(Tabs);
```

- [ ] **Step 5: 构建前端**

Run: `cd frontend && npm run build`
Expected: 构建成功。

- [ ] **Step 6: Commit**

```bash
git add frontend/src/views/NotificationsView.vue frontend/src/router/index.ts frontend/src/views/ProfileView.vue frontend/src/main.ts
git commit -m "feat: add notification list page and profile entry"
```
（git 不可用时跳过本步）

---

### Task 7: 「我的」tab 圆点绑定 + 应用挂载/生命周期

**Files:**
- Modify: `frontend/src/components/ShopTabbar.vue`
- Modify: `frontend/src/App.vue`

**Interfaces:**
- Consumes: Task 5 的 `useNotificationStore`（`unread`、`connect`、`disconnect`）。
- Produces: 无。

- [ ] **Step 1: 绑定圆点**

Modify `frontend/src/components/ShopTabbar.vue`：

(a) `<script setup>` 改为：

```ts
import { useShopCart } from "../stores/shop";
import { useNotificationStore } from "../stores/notification";
const { count } = useShopCart();
const notificationStore = useNotificationStore();
```

(b) 「我的」tab 改为：

```vue
    <van-tabbar-item icon="user-o" to="/profile" :dot="notificationStore.unread > 0">我的</van-tabbar-item>
```

（替换掉此前静态的 `dot` 属性。）

- [ ] **Step 2: 挂载与生命周期**

Modify `frontend/src/App.vue`，替换整个 `<script setup>` 块为：

```ts
import { onMounted, watch } from "vue";
import { useRoute } from "vue-router";
import ShopTabbar from "./components/ShopTabbar.vue";
import { useUserStore } from "./stores/user";
import { useNotificationStore } from "./stores/notification";

const route = useRoute();
const userStore = useUserStore();
const notificationStore = useNotificationStore();

onMounted(() => {
  userStore.validate();
  if (userStore.isLoggedIn) notificationStore.connect();
});

watch(() => userStore.isLoggedIn, (loggedIn) => {
  if (loggedIn) notificationStore.connect();
  else notificationStore.disconnect();
});
```

- [ ] **Step 3: 构建前端**

Run: `cd frontend && npm run build`
Expected: 构建成功。

- [ ] **Step 4: 端到端手动验证**

启动后端（`go run .` 或 `shop.exe`）与前端 dev（`cd frontend && npm run dev`，或 Wails 构建后运行桌面应用）：

1. 登录（可用 `admin`/`123456` 或注册新用户）。
2. 下单，产生一条 `pending` 订单。
3. 等 ~20s：收到"订单已发货"推送，toast 弹出，「我的」tab 出现圆点。
4. 等 ~90s：收到优惠券通知。
5. 进入「个人中心」→「消息通知」：列表可见，未读有红点；点"全部已读"后「我的」圆点消失；刷新后未读状态仍在。

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/ShopTabbar.vue frontend/src/App.vue
git commit -m "feat: wire unread dot and notification lifecycle"
```
（git 不可用时跳过本步）

---

## Self-Review

**1. Spec coverage:**
- 推送通道 SSE → Task 2（`streamNotifications` + 路由）。
- 数据模型 `Notification` → Task 1。
- 通知中心 hub/notify → Task 1。
- SSE 端点 + tokenFromQuery → Task 2。
- REST 端点（list/unread/read/read-all）→ Task 2。
- 订单状态变化触发 → Task 3。
- 模拟商家定时发货 + 优惠券推送 → Task 4。
- 前端 store → Task 5。
- 「我的」圆点绑定 → Task 7。
- 通知列表页 → Task 6。
- Profile 入口 → Task 6。
- 生命周期挂载 → Task 7。

**2. Placeholder scan:** 无 TBD/TODO；所有代码步骤均含完整实现。

**3. Type consistency:**
- `notify(db, hub, userID, typ, title, content, orderNo)` 在 Task 1 定义，Task 3/4 按此签名调用 ✓。
- `statusText`、`NotificationTypeOrder/Coupon`、`newNotificationHub` 在 Task 1 定义，后续任务引用一致 ✓。
- `useNotificationStore` 暴露 `items/unread/fetchList/fetchUnread/markRead/markAllRead/connect/disconnect`，Task 6/7 均按此使用 ✓。
- `AppNotification` 字段与后端 `Notification` 的 JSON 标签一致（`id/userId/type/title/content/orderNo/read/createdAt`）✓。
