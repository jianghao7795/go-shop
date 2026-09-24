# 管理后台 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 给商城加一个管理后台：管理员可管理商品、分类、订单、用户、优惠券，并发送站内通知。

**Architecture:** 单代码库内扩展。后端在现有 Gin API 上新增 `/api/admin/*` 路由组（JWT + `adminRequired()` 中间件），复用现有 model 并补齐 CRUD handler；前端在现有 Vue 应用新增 `/admin/*` 路由段 + 独立 `AdminLayout`，普通用户不可见不可进。

**Tech Stack:** Go + Gin + GORM（后端）；Vue 3 + Pinia + Vant + vue-router（前端）。

**Spec:** `docs/superpowers/specs/2026-09-24-admin-backend-design.md`

## Global Constraints

- 管理员由手动在数据库把 `users.shop_users.role` 设为 `admin`，不写种子、不做提权 UI。
- 管理接口不回退到种子/兜底数据；数据库不可用时返回 503。
- 管理接口全部要求 `role == "admin"`，非管理员返回 403。
- 前端 `/admin/*` 要求已登录且 `role === "admin"`，否则跳回商城。
- 商品上/下架用新增字段 `Product.OnShelf bool`（默认 `true`）。
- 提交信息用中文；每个任务结束提交一次。

---

## 文件结构

- `internal/api/middleware.go`（新增）：`adminRequired()` 中间件。
- `internal/api/handlers_admin.go`（新增）：全部 admin handler（商品/分类/订单/用户/优惠券/通知），按模块分段。
- `internal/api/handlers_admin_test.go`（新增）：校验纯函数的表驱动单测。
- `internal/api/server.go`（修改）：注册 `/api/admin` 路由组。
- `internal/model/product.go`（修改）：`Product` 增加 `OnShelf bool`。
- `frontend/src/stores/user.ts`（修改）：增加 `role` 状态。
- `frontend/src/router/index.ts`（修改）：新增 `/admin/*` 路由与守卫。
- `frontend/src/layouts/AdminLayout.vue`（新增）：后台侧边导航布局。
- `frontend/src/views/admin/`（新增）：`DashboardView.vue`、`ProductsView.vue`、`CategoriesView.vue`、`OrdersView.vue`、`UsersView.vue`、`CouponsView.vue`、`NotificationsView.vue`。
- `frontend/src/views/ProfileView.vue`（修改）：管理员显示「管理后台」入口。

---

## 阶段一：权限 + 后台布局 + 用户管理

### Task 1: 管理员鉴权中间件与路由组

**Files:**
- Create: `internal/api/middleware.go`
- Modify: `internal/api/server.go:71-88`

**Interfaces:**
- Produces: `adminRequired() gin.HandlerFunc` —— 供后续所有 admin 路由使用。

- [ ] **Step 1: 写中间件**

```go
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
```

- [ ] **Step 2: 注册 admin 路由组**

在 `server.go` 的 `protected` 组之后追加：

```go
	admin := router.Group("/api/admin")
	admin.Use(jwtMiddleware.MiddlewareFunc(), adminRequired())
```

- [ ] **Step 3: 编译验证**

Run: `go build ./internal/...`
Expected: 编译通过。

- [ ] **Step 4: 提交**

```bash
git add internal/api/middleware.go internal/api/server.go
git commit -m "feat: 新增管理员鉴权中间件与 /api/admin 路由组"
```

### Task 2: 管理员用户列表与禁用接口

**Files:**
- Create: `internal/api/handlers_admin.go`
- Modify: `internal/api/server.go`（admin 组内注册）

**Interfaces:**
- Consumes: `adminRequired()`（Task 1）、`model.User`、`currentUser`（已存在，返回当前用户名）
- Produces:
  - `adminListUsers(db *gorm.DB) gin.HandlerFunc` → `GET /admin/users`
  - `adminUpdateUserStatus(db *gorm.DB) gin.HandlerFunc` → `PUT /admin/users/:id/status`

- [ ] **Step 1: 写 handler**

```go
package api

import (
	"net/http"
	"strconv"

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
```

- [ ] **Step 2: 注册路由**

在 `server.go` 的 `admin` 组内加：

```go
	admin.GET("/users", adminListUsers(db))
	admin.PUT("/users/:id/status", adminUpdateUserStatus(db))
```

- [ ] **Step 3: 编译验证**

Run: `go build ./internal/...`
Expected: 编译通过。

- [ ] **Step 4: 端到端验证（先手动把一个用户 role 设为 admin）**

```sql
UPDATE shop.shop_users SET role = 'admin' WHERE username = '<某账号>';
```

Run（admin token 登录后）: `curl -H "Authorization: Bearer $ADMIN_TOKEN" http://localhost:8080/api/admin/users`
Expected: 返回用户列表 JSON。

Run（普通用户 token）: 同上
Expected: HTTP 403 `{"message":"无管理员权限"}`。

- [ ] **Step 5: 提交**

```bash
git add internal/api/handlers_admin.go internal/api/server.go
git commit -m "feat: 管理员用户列表与禁用接口"
```

### Task 3: 前端 role 状态 + admin 路由守卫

**Files:**
- Modify: `frontend/src/stores/user.ts`
- Modify: `frontend/src/router/index.ts`

**Interfaces:**
- Produces: `userStore.role`（`ref<string>`，取值 `"customer"`/`"admin"`）；`/admin/*` 路由守卫。

- [ ] **Step 1: store 增加 role**

在 `user.ts` 中：

```ts
const role = ref("");
// applyProfile 内追加：
role.value = p.role || "customer";
// logout 内追加：
role.value = "";
// 返回对象内追加 role
```

- [ ] **Step 2: 路由守卫**

在 `router/index.ts` 的 `beforeEach` 中，`requiresAuth` 判断之后追加：

```ts
  if (to.path.startsWith("/admin")) {
    if (!userStore.isLoggedIn) return { name: "login", query: { redirect: to.fullPath } };
    if (userStore.role !== "admin") return { name: "home" };
  }
```

- [ ] **Step 3: 构建验证**

Run: `cd frontend && npm run build`
Expected: 构建通过。

- [ ] **Step 4: 提交**

```bash
git add frontend/src/stores/user.ts frontend/src/router/index.ts
git commit -m "feat: 前端 role 状态与 /admin 路由守卫"
```

### Task 4: AdminLayout 与用户管理页

**Files:**
- Create: `frontend/src/layouts/AdminLayout.vue`
- Create: `frontend/src/views/admin/UsersView.vue`
- Modify: `frontend/src/router/index.ts`（注册 admin 路由 + 布局）

**Interfaces:**
- Consumes: `userStore.role`（Task 3）、`http`（`lib/http`）
- Produces: `AdminLayout`（含侧边导航，`<router-view>` 子路由出口）

- [ ] **Step 1: AdminLayout**

```vue
<template>
  <div class="admin-layout">
    <aside class="admin-sidebar">
      <div class="admin-brand">🛍️ 管理后台</div>
      <router-link to="/admin">概览</router-link>
      <router-link to="/admin/products">商品</router-link>
      <router-link to="/admin/categories">分类</router-link>
      <router-link to="/admin/orders">订单</router-link>
      <router-link to="/admin/users">用户</router-link>
      <router-link to="/admin/coupons">优惠券</router-link>
      <router-link to="/admin/notifications">通知</router-link>
      <router-link to="/profile">← 返回商城</router-link>
    </aside>
    <main class="admin-main"><router-view /></main>
  </div>
</template>
```

- [ ] **Step 2: UsersView（列表 + 禁用/启用按钮）**

```vue
<script setup lang="ts">
import { onMounted, ref } from "vue";
import { showToast } from "vant";
import http from "../../lib/http";

const list = ref<any[]>([]);
async function load() {
  const res = await http.get("/api/admin/users");
  list.value = res.data;
}
async function toggleStatus(u: any) {
  const next = u.status === 1 ? 0 : 1;
  await http.put("/api/admin/users/" + u.id + "/status", { status: next });
  showToast("已更新");
  load();
}
onMounted(load);
</script>

<template>
  <div class="admin-page">
    <h2>用户管理</h2>
    <table class="admin-table">
      <thead><tr><th>ID</th><th>用户名</th><th>昵称</th><th>手机号</th><th>角色</th><th>状态</th><th>操作</th></tr></thead>
      <tbody>
        <tr v-for="u in list" :key="u.id">
          <td>{{ u.id }}</td><td>{{ u.username }}</td><td>{{ u.nickname || "-" }}</td>
          <td>{{ u.mobile || "-" }}</td><td>{{ u.role }}</td>
          <td>{{ u.status === 1 ? "正常" : "禁用" }}</td>
          <td><van-button size="small" @click="toggleStatus(u)">{{ u.status === 1 ? "禁用" : "启用" }}</van-button></td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
```

- [ ] **Step 3: 注册 admin 路由**

```ts
    {
      path: "/admin",
      component: () => import("../layouts/AdminLayout.vue"),
      meta: { requiresAuth: true },
      children: [
        { path: "", name: "admin-dashboard", component: () => import("../views/admin/DashboardView.vue"), meta: { title: "概览" } },
        { path: "users", name: "admin-users", component: () => import("../views/admin/UsersView.vue"), meta: { title: "用户管理" } },
        { path: "products", name: "admin-products", component: () => import("../views/admin/ProductsView.vue"), meta: { title: "商品管理" } },
        { path: "categories", name: "admin-categories", component: () => import("../views/admin/CategoriesView.vue"), meta: { title: "分类管理" } },
        { path: "orders", name: "admin-orders", component: () => import("../views/admin/OrdersView.vue"), meta: { title: "订单管理" } },
        { path: "coupons", name: "admin-coupons", component: () => import("../views/admin/CouponsView.vue"), meta: { title: "优惠券管理" } },
        { path: "notifications", name: "admin-notifications", component: () => import("../views/admin/NotificationsView.vue"), meta: { title: "通知管理" } },
      ],
    },
```

> 说明：本步先创建 `UsersView.vue` 和 `DashboardView.vue`（概览页可为简单占位），其余 admin 页面在后续任务逐个补上；为避免路由 import 缺失导致构建失败，请在本步一并创建所有 `views/admin/*.vue` 的**最小占位组件**（`<template><div class="admin-page"><h2>占位</h2></div></template>`），后续任务再填充。

- [ ] **Step 4: 构建验证**

Run: `cd frontend && npm run build`
Expected: 构建通过。

- [ ] **Step 5: 提交**

```bash
git add frontend/src/layouts/AdminLayout.vue frontend/src/views/admin frontend/src/router/index.ts
git commit -m "feat: 后台布局与用户管理页"
```

### Task 5: 个人中心管理后台入口

**Files:**
- Modify: `frontend/src/views/ProfileView.vue`

- [ ] **Step 1: 管理员显示入口**

在 `profile-tools` 组内（「编辑资料」之后）追加：

```html
      <van-cell v-if="userStore.role === 'admin'" title="管理后台" icon="manager-o" is-link @click="router.push('/admin')" />
```

- [ ] **Step 2: 构建验证 + 提交**

```bash
cd frontend && npm run build
git add frontend/src/views/ProfileView.vue
git commit -m "feat: 个人中心管理员后台入口"
```

---

## 阶段二：商品 + 分类管理

### Task 6: Product 增加 OnShelf 字段

**Files:**
- Modify: `internal/model/product.go:10-23`

- [ ] **Step 1: 加字段**

在 `Product` 结构体中 `Category` 之后追加：

```go
	OnShelf      bool           `json:"onShelf" gorm:"default:true"`
```

- [ ] **Step 2: 编译 + 提交**

```bash
go build ./internal/...
git add internal/model/product.go
git commit -m "feat: 商品增加 OnShelf 上下架字段"
```

### Task 7: 商品管理接口（CRUD + 上下架）

**Files:**
- Modify: `internal/api/handlers_admin.go`
- Create: `internal/api/handlers_admin_test.go`
- Modify: `internal/api/server.go`

**Interfaces:**
- Produces:
  - `validateProduct(p productPayload) string`（纯函数，空串合法）
  - `adminListProducts(db) gin.HandlerFunc`、`adminCreateProduct(db)`、`adminUpdateProduct(db)`、`adminDeleteProduct(db)`

- [ ] **Step 1: 写校验纯函数的失败测试**

`handlers_admin_test.go`：

```go
package api

import "testing"

func TestValidateProduct(t *testing.T) {
	tests := []struct {
		name string
		p    productPayload
		ok   bool
	}{
		{"合法", productPayload{Name: "商品A", Price: 10, Category: "Home"}, true},
		{"缺名称", productPayload{Price: 10, Category: "Home"}, false},
		{"价格为负", productPayload{Name: "A", Price: -1, Category: "Home"}, false},
		{"缺分类", productPayload{Name: "A", Price: 10}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateProduct(tt.p); (got == "") != tt.ok {
				t.Fatalf("validateProduct(%+v)=%q, ok=%v", tt.p, got, tt.ok)
			}
		})
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/api/ -run TestValidateProduct`
Expected: 编译失败（`productPayload`/`validateProduct` 未定义）。

- [ ] **Step 3: 实现校验 + CRUD handler**

在 `handlers_admin.go` 追加：

```go
type productPayload struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	OriginalPrice float64 `json:"originalPrice"`
	Emoji       string  `json:"emoji"`
	Color       string  `json:"color"`
	Category    string  `json:"category"`
	OnShelf     *bool   `json:"onShelf"`
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
		if err := db.Delete(&model.Product{}, id).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "删除失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
	}
}
```

- [ ] **Step 4: 运行测试通过**

Run: `go test ./internal/api/ -run TestValidateProduct`
Expected: PASS。

- [ ] **Step 5: 注册路由**

```go
	admin.GET("/products", adminListProducts(db))
	admin.POST("/products", adminCreateProduct(db))
	admin.PUT("/products/:id", adminUpdateProduct(db))
	admin.DELETE("/products/:id", adminDeleteProduct(db))
```

- [ ] **Step 6: 编译 + 端到端 + 提交**

```bash
go build ./internal/...
# 端到端：POST 创建、PUT 上下架、GET 列表、DELETE
git add internal/api/handlers_admin.go internal/api/handlers_admin_test.go internal/api/server.go
git commit -m "feat: 商品管理接口（CRUD + 上下架）"
```

### Task 8: 分类管理接口

**Files:**
- Modify: `internal/api/handlers_admin.go`、`internal/api/handlers_admin_test.go`、`internal/api/server.go`

- [ ] **Step 1: 校验测试 + 实现**

```go
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
```

handler：`adminListCategories`、`adminCreateCategory`、`adminUpdateCategory`、`adminDeleteCategory`（逻辑与商品一致，操作 `model.Category`，`Key` 唯一冲突返回 409）。

- [ ] **Step 2: 注册路由 + 测试 + 提交**

```go
	admin.GET("/categories", adminListCategories(db))
	admin.POST("/categories", adminCreateCategory(db))
	admin.PUT("/categories/:id", adminUpdateCategory(db))
	admin.DELETE("/categories/:id", adminDeleteCategory(db))
```

```bash
go test ./internal/api/ -run TestValidateCategory
go build ./internal/...
git add internal/api/handlers_admin.go internal/api/handlers_admin_test.go internal/api/server.go
git commit -m "feat: 分类管理接口"
```

### Task 9: 商品与分类管理页

**Files:**
- Modify: `frontend/src/views/admin/ProductsView.vue`（替换占位）
- Modify: `frontend/src/views/admin/CategoriesView.vue`（替换占位）

- [ ] **Step 1: ProductsView（表格 + 新建/编辑弹窗 + 上下架/删除）**

用 `van-dialog`/`van-popup` 承载表单；字段：名称、描述、价格、原价、emoji、颜色、分类（下拉选现有分类）、上架开关。提交走 `POST /api/admin/products` 或 `PUT /api/admin/products/:id`。

- [ ] **Step 2: CategoriesView（表格 + 新建/编辑/删除）**

字段：Key、名称、图标（emoji）、备注。

- [ ] **Step 3: 构建验证 + 提交**

```bash
cd frontend && npm run build
git add frontend/src/views/admin/ProductsView.vue frontend/src/views/admin/CategoriesView.vue
git commit -m "feat: 商品与分类管理页"
```

---

## 阶段三：订单管理

### Task 10: 订单管理接口

**Files:**
- Modify: `internal/api/handlers_admin.go`、`internal/api/server.go`

**Interfaces:**
- Produces: `adminListOrders(db)`、`adminGetOrder(db)`、`adminUpdateOrderStatus(db)`

- [ ] **Step 1: 实现**

```go
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
```

`adminGetOrder` 按 id 查单个订单；`adminUpdateOrderStatus` 校验 `status` 属于 `pending/shipped/completed/aftersale/finished` 后 `Update("status", ...)` 并调用 `notify(...)` 推送订单状态变更通知。

- [ ] **Step 2: 注册路由**

```go
	admin.GET("/orders", adminListOrders(db))
	admin.GET("/orders/:id", adminGetOrder(db))
	admin.PUT("/orders/:id/status", adminUpdateOrderStatus(db))
```

- [ ] **Step 3: 编译 + 端到端 + 提交**

```bash
go build ./internal/...
git add internal/api/handlers_admin.go internal/api/server.go
git commit -m "feat: 订单管理接口"
```

### Task 11: 订单管理页

**Files:**
- Modify: `frontend/src/views/admin/OrdersView.vue`（替换占位）

- [ ] **Step 1: 表格 + 状态筛选 + 改状态**

表格列：订单号、用户、金额、状态、下单时间；状态筛选下拉；「改状态」弹窗选择目标状态后 `PUT /api/admin/orders/:id/status`。

- [ ] **Step 2: 构建验证 + 提交**

```bash
cd frontend && npm run build
git add frontend/src/views/admin/OrdersView.vue
git commit -m "feat: 订单管理页"
```

---

## 阶段四：优惠券 + 通知管理

### Task 12: 优惠券管理接口

**Files:**
- Modify: `internal/api/handlers_admin.go`、`internal/api/handlers_admin_test.go`、`internal/api/server.go`

- [ ] **Step 1: 校验 + CRUD**

```go
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
```

handler：`adminListCoupons`、`adminCreateCoupon`（`StartAt/EndAt` 解析 `time.Parse("2006-01-02 15:04:05", ...)`，解析失败或 start>=end 返回 400）、`adminUpdateCoupon`、`adminDeleteCoupon`。

- [ ] **Step 2: 注册路由 + 测试 + 提交**

```go
	admin.GET("/coupons", adminListCoupons(db))
	admin.POST("/coupons", adminCreateCoupon(db))
	admin.PUT("/coupons/:id", adminUpdateCoupon(db))
	admin.DELETE("/coupons/:id", adminDeleteCoupon(db))
```

```bash
go test ./internal/api/ -run TestValidateCoupon
go build ./internal/...
git add internal/api/handlers_admin.go internal/api/handlers_admin_test.go internal/api/server.go
git commit -m "feat: 优惠券管理接口"
```

### Task 13: 通知发送接口

**Files:**
- Modify: `internal/api/handlers_admin.go`、`internal/api/server.go`

**Interfaces:**
- Consumes: `notify(db, hub, userID, type, title, content, orderNo)`（已存在于 `notify.go`）

- [ ] **Step 1: 实现**

```go
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
		if req.Username != "" {
			notify(db, hub, req.Username, model.NotificationTypeCoupon, req.Title, req.Content, "")
		} else {
			var users []model.User
			db.Find(&users)
			for _, u := range users {
				notify(db, hub, u.Username, model.NotificationTypeCoupon, req.Title, req.Content, "")
			}
		}
		c.JSON(http.StatusOK, gin.H{"message": "已发送"})
	}
}
```

- [ ] **Step 2: 注册路由**

```go
	admin.POST("/notifications", adminSendNotification(db, hub))
```

- [ ] **Step 3: 编译 + 提交**

```bash
go build ./internal/...
git add internal/api/handlers_admin.go internal/api/server.go
git commit -m "feat: 通知发送接口"
```

### Task 14: 优惠券与通知管理页

**Files:**
- Modify: `frontend/src/views/admin/CouponsView.vue`（替换占位）
- Modify: `frontend/src/views/admin/NotificationsView.vue`（替换占位）
- Modify: `frontend/src/views/admin/DashboardView.vue`（替换占位为简单统计）

- [ ] **Step 1: CouponsView（表格 + 新建/编辑/删除，起止时间用时间选择器）**

- [ ] **Step 2: NotificationsView（表单：用户名可空=广播、标题、内容；提交 `POST /api/admin/notifications`）**

- [ ] **Step 3: DashboardView（简单统计：商品数/订单数/用户数，调 `GET /api/admin/products|orders|users` 取 `length`）**

- [ ] **Step 4: 构建验证 + 提交**

```bash
cd frontend && npm run build
git add frontend/src/views/admin/CouponsView.vue frontend/src/views/admin/NotificationsView.vue frontend/src/views/admin/DashboardView.vue
git commit -m "feat: 优惠券、通知、概览管理页"
```

---

## 收尾

### Task 15: 全量回归与推送

- [ ] **Step 1: 后端全量测试（跳过偶发 SSE 用例）**

Run: `go test ./internal/api/ -skip TestSSEPushEndToEnd ./...`
Expected: 通过。

- [ ] **Step 2: 前端构建**

Run: `cd frontend && npm run build`
Expected: 通过。

- [ ] **Step 3: 重建桌面应用并启动自测**

Run: `wails3 build` 后重启，管理员账号登录 → 进入 `/admin` → 各模块冒烟。

- [ ] **Step 4: 提交并推送**

```bash
git add -A
git commit -m "feat: 管理后台完成" # 若前面任务均已提交，此步可省略
git push
```
