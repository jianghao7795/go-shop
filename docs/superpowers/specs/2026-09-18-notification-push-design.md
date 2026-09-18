# 实时通知系统设计（订单 + 优惠券）

日期：2026-09-18
状态：待评审

## 1. 目标

让服务器能主动把信息推送到客户端（不依赖客户端轮询），并驱动「我的」tab 上的圆点角标。

支持两类通知：

1. **订单变动通知** —— 订单状态变化时推送（含"模拟商家定时发货"这种真正的服务器主动场景）
2. **优惠券通知** —— 模拟营销推送（"新人专享券已到账"等）

圆点语义：**未读通知数 > 0 时显示**，查看并标记已读后消失。

## 2. 现有架构与约束

- Wails v3 桌面应用：Go 后端内嵌 Vue 前端（`frontend/dist`）。
- 同时跑一个 Gin HTTP API（`internal/api`，默认监听 `:8080`），前端数据全部通过 `fetch(apiBase + ...)` 访问它。
- 前端已引入 `@wailsio/runtime`（仅调用 `WML.Enable()`），未使用 Wails 事件/服务。
- JWT 认证由 `github.com/appleboy/gin-jwt/v2` 提供，`IdentityKey = "user"`，`currentUser(c)` 返回用户名字符串作为 UserID。
- 订单状态：`pending(待付款) → shipped(待收货) → completed(待评价) → finished(已完成)`，另有 `aftersale(售后)`。
- 优惠券为静态演示数据（`listCoupons` 内硬编码 4 张券）。
- 后端无消息/通知领域模型。

## 3. 推送通道：SSE

选择 **Server-Sent Events（SSE）**，理由：

- 天然单向（服务器→客户端），匹配"服务器主动推"这一需求。
- 走现有 Gin HTTP，浏览器 dev 模式与 Wails webview 都可用，与"前端全走 HTTP"的架构一致。
- `gin-contrib/sse` 已在依赖树中。

否决方案：

- **WebSocket**：双向能力用不上，复杂度更高。
- **Wails Events**：仅在桌面 webview 生效，浏览器 dev 收不到；且订单/优惠券逻辑都在 Gin 侧，跨进程接线别扭。

## 4. 后端设计

### 4.1 数据模型

新增 `internal/model/notification.go`：

```go
type Notification struct {
    ID        uint      `json:"id" gorm:"primaryKey"`
    UserID    string    `json:"userId" gorm:"index;size:64"`
    Type      string    `json:"type" gorm:"size:20"` // order | coupon
    Title     string    `json:"title" gorm:"size:128"`
    Content   string    `json:"content" gorm:"size:255"`
    OrderNo   string    `json:"orderNo" gorm:"size:64"`
    Read      bool      `json:"read" gorm:"default:false"`
    CreatedAt time.Time `json:"createdAt"`
}
```

表名 `shop_notifications`，加入 `Start()` 的 AutoMigrate。

### 4.2 通知中心（hub + 落库 + 广播）

新增 `internal/api/notify.go`：

- `notificationHub`：`map[string]map[chan model.Notification]struct{}`，按 UserID 维护订阅通道集合，互斥锁保护。
- `subscribe(userID) / unsubscribe(userID, ch) / broadcast(userID, n)`。
- `notify(db, hub, userID, typ, title, content, orderNo)`：先写 DB，再 `broadcast`。DB 不可用（`db == nil`）时仅广播、不落库。

### 4.3 SSE 端点

`GET /api/notifications/stream`：

- 认证：EventSource 无法携带自定义 header，改用 `?token=` 传 JWT。`tokenFromQuery` 中间件在无 Authorization header 时把 query 中的 token 写入 `Authorization: Bearer <token>`，再复用现有 `jwtMiddleware.MiddlewareFunc()`。
- 逻辑：`currentUser(c)` → `hub.subscribe(userID)`，defer 退订；设置 `Content-Type: text/event-stream`、`Cache-Control: no-cache` 等头并 `Flush()`；循环 `select`：
  - 收到广播 → `c.SSEvent("notification", json)` + `Flush()`
  - 30s ticker → 写心跳注释 `: ping\n\n` 保活
  - `c.Request.Context().Done()` → 退出

### 4.4 通知 REST 端点（需登录）

- `GET /api/notifications` → 当前用户通知列表（`id DESC`）
- `GET /api/notifications/unread` → `{ "count": n }`
- `PUT /api/notifications/:id/read` → 单条标记已读
- `PUT /api/notifications/read-all` → 全部标记已读

### 4.5 触发源

**订单状态变化**（`updateOrderStatus`）：更新成功后查出订单，`notify(..., "order", "订单状态更新", "您的订单 <OrderNo> 已更新为 <状态文案>", orderNo)`。

状态文案映射（后端维护一份 `statusText`，与前端 `OrderListView` 的 `statusMap` 保持一致）：`pending→待付款`、`shipped→待收货`、`completed→待评价`、`aftersale→售后`、`finished→已完成`。

**模拟商家定时发货**：新增 `startSimulators(db, hub)` 后台 goroutine（在 `Start()` 中 db 打开后启动），内部两个 ticker：

- 每 ~20s：查一条 `status = pending` 的订单，改为 `shipped`，推"订单已发货"；无 pending 则跳过。
- 每 ~90s：轮换推一条优惠券通知（如"新人专享券已到账，满 99 减 10"）。

均以 `db != nil` 为前提；优惠券数据从 `listCoupons` 中抽出为包级变量，供 handler 与模拟器共用。

## 5. 前端设计

### 5.1 Pinia store `stores/notification.ts`

状态：`unread`（number）、`items`（Notification[]）。

- `connect()`：`new EventSource(apiBase + "/api/notifications/stream?token=" + encodeURIComponent(token))`；
  - `addEventListener("notification", e => { 解析 JSON；items.unshift(n)；unread++；showToast(title) })`
  - `onopen`：`fetchUnread()` 重同步（覆盖重连期间漏掉的推送）
  - 依赖 EventSource 自动重连，不额外写重连逻辑
- `disconnect()`：`source.close()`
- `fetchList()` / `fetchUnread()` / `markRead(id)` / `markAllRead()`：走现有 `fetch(apiBase + ...)` 加 `Authorization` header。

登录时 `connect()`，登出时 `disconnect()`。

### 5.2 `ShopTabbar.vue`

「我的」tab：`dot`（静态）→ `:dot="unread > 0"`。取代上一轮"始终显示圆点"的改动。

### 5.3 新增通知列表页 `views/NotificationsView.vue`

- 路由 `/notifications`（`hideTabbar: true`、`requiresAuth: true`）
- 列表展示：type 图标（订单/优惠券）、title、content、时间、已读/未读视觉区分
- 顶部"全部已读"按钮 → `markAllRead()` 并把本地 `items` 全部置为已读、`unread = 0`

### 5.4 `ProfileView.vue`

新增"消息通知"cell（带未读角标），跳转 `/notifications`。

### 5.5 挂载与生命周期

`App.vue` `onMounted`：若已登录则 `notificationStore.connect()`；`watch(isLoggedIn)` 切换 connect/disconnect。

## 6. 数据流

```
[更新订单状态 / 模拟商家发货 / 模拟营销]
        │ notify()
        ▼
   DB 写入 shop_notifications ──► hub.broadcast(userID)
                                       │
                              SSE stream（长连接）
                                       │
                          前端 notification store
                                       │
                    unread++ ──► 「我的」圆点 + toast
```

## 7. 错误处理

- DB 不可用：通知仅广播不落库；列表/unread 接口返回空或 503。
- SSE 断线：浏览器 EventSource 自动重连，`onopen` 时重拉未读数对账。
- 未登录：SSE 与 REST 端点均走 JWT，未登录返回 401；前端仅在登录态下 connect。

## 8. 验证

1. `cd` 到仓库根目录，`go build ./...`、`go vet ./...`。
2. 前端 `npm run build`（或 dev）。
3. 手动走查：登录 → 下单 → 等 ~20s 收到"订单已发货"推送、圆点点亮 → 等 ~90s 收到优惠券推送 → 进入"消息通知"列表 → 标已读 → 圆点消失；刷新后未读状态仍在。
