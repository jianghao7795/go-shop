# 动态权限 RBAC Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 RBAC 权限点从硬编码常量重构为数据库驱动：新增权限点表 + 角色↔权限多对多，提供权限点管理页，并统一前端权限判断。

**Architecture:** 新增 `shop_permissions`（code/name/path）+ `shop_role_permissions`（多对多）；移除 `Role.Permissions` JSON 与权限常量；`effectivePermissions` 改为三表 join；`requirePermission(db, code)` 的 code 用 `shop_permissions.code` 字面量；后台新增权限管理页。

**Tech Stack:** Go + Gin + GORM；Vue 3 + Pinia + Vant。

**Spec:** `docs/superpowers/specs/2026-09-28-rbac-dynamic-design.md`

## Global Constraints

- 权限点数据驱动：`shop_permissions.code` 唯一；路由里用 `requirePermission(db, "<code>")` 字面量。
- 角色↔权限多对多，`Role` 不再有 `permissions` JSON 字段。
- 权限点管理（新建/删除）仅限 `user:manage`；被角色引用的权限禁止删除；不做权限点编辑。
- 内置角色 admin/customer 仍不可改删；user:manage 仍保留给内置 admin 角色（防提权）。
- 提交信息用中文。

---

## 文件结构

- `internal/model/permission.go`（新增）：`Permission`、`RolePermission` 模型 + `SeedPermissions`。
- `internal/model/role.go`（修改）：移除 `Role.Permissions`、`PermXxx`、`AllPermissions`。
- `internal/api/permission.go`（修改）：`effectivePermissions` 三表 join；保留 `hasPermission`；移除 `unionPermissions`。
- `internal/api/permission_test.go`（修改）：更新单测。
- `internal/api/handlers_admin.go`（修改）：角色 CRUD 用 `permissionIds`；新增权限点 CRUD；`validateRole` 改为校验 name + 权限 ID 存在。
- `internal/api/server.go`（修改）：种子权限 + admin 角色关联；注册权限点路由；路由 code 字面量。
- `frontend/src/stores/user.ts`（修改）：加 `hasPerm(code)`。
- `frontend/src/lib/admin-menu.ts`（新增）：集中式菜单配置。
- `frontend/src/layouts/AdminLayout.vue`、`frontend/src/views/admin/DashboardView.vue`、`frontend/src/views/ProfileView.vue`（修改）：走 `hasPerm` / 菜单配置。
- `frontend/src/views/admin/RolesView.vue`（修改）：权限复选从 `/api/admin/permissions` 拉取。
- `frontend/src/views/admin/PermissionsView.vue`（新增）：权限管理页。
- `frontend/src/router/index.ts`（修改）：`/admin/permissions` 路由。

---

## 阶段一：后端模型与种子

### Task 1: Permission/RolePermission 模型与权限种子

**Files:**
- Create: `internal/model/permission.go`
- Modify: `internal/model/role.go`（移除 `Permissions`/`PermXxx`/`AllPermissions`）
- Modify: `internal/api/server.go`（AutoMigrate + 种子）

- [ ] **Step 1: 写 Permission/RolePermission 模型**

```go
package model

import (
	"time"

	"gorm.io/gorm"
)

type Permission struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Code      string         `json:"code" gorm:"uniqueIndex;size:64"`
	Name      string         `json:"name" gorm:"size:64"`
	Path      string         `json:"path" gorm:"size:128"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `json:"-"`
}

func (Permission) TableName() string { return "shop_permissions" }

type RolePermission struct {
	ID           uint `json:"id" gorm:"primaryKey"`
	RoleID       uint `json:"roleId" gorm:"index"`
	PermissionID uint `json:"permissionId" gorm:"index"`
}

func (RolePermission) TableName() string { return "shop_role_permissions" }

// SeedPermissions 是内置的 6 个初始权限点。
var SeedPermissions = []Permission{
	{Code: "product:manage", Name: "商品管理", Path: "/admin/products"},
	{Code: "category:manage", Name: "分类管理", Path: "/admin/categories"},
	{Code: "order:manage", Name: "订单管理", Path: "/admin/orders"},
	{Code: "user:manage", Name: "用户管理", Path: "/admin/users"},
	{Code: "coupon:manage", Name: "优惠券管理", Path: "/admin/coupons"},
	{Code: "notification:manage", Name: "通知管理", Path: "/admin/notifications"},
}
```

- [ ] **Step 2: 暂不删除旧字段/常量（本任务保持增量，清理放到 Task 4）**

本任务只新增模型与种子，`Role.Permissions`、`PermXxx`、`AllPermissions`、`unionPermissions`、`validateRole` 等旧代码**全部保留**，保证中间态可编译。种子改为不再引用 `AllPermissions`（改由 role_permissions 关联）。

- [ ] **Step 3: 改 server.go 的 AutoMigrate 与种子**

AutoMigrate 追加 `&model.Permission{}, &model.RolePermission{}`。种子段追加：

```go
	// 种子权限点
	for _, p := range model.SeedPermissions {
		var existing model.Permission
		if db.Where("code = ?", p.Code).First(&existing).Error != nil {
			db.Create(&p)
		}
	}
	// 种子 admin 角色并关联全部权限（含旧 roles.permissions JSON 的迁移）
	var adminRole model.Role
	if db.Where("name = ?", model.RoleAdmin).First(&adminRole).Error != nil {
		adminRole = model.Role{Name: model.RoleAdmin, Description: "超级管理员"}
		db.Create(&adminRole)
	}
	var allPerms []model.Permission
	db.Find(&allPerms)
	var linked int64
	db.Model(&model.RolePermission{}).Where("role_id = ?", adminRole.ID).Count(&linked)
	if linked == 0 {
		for _, p := range allPerms {
			db.Create(&model.RolePermission{RoleID: adminRole.ID, PermissionID: p.ID})
		}
	}
	var customerRole model.Role
	if db.Where("name = ?", model.RoleCustomer).First(&customerRole).Error != nil {
		db.Create(&model.Role{Name: model.RoleCustomer, Description: "普通用户"})
	}
```

- [ ] **Step 4: 编译 + 提交**

```bash
go build ./internal/...
git add internal/model/permission.go internal/model/role.go internal/api/server.go
git commit -m "feat: 权限点表 + 角色权限多对多模型与种子"
```

### Task 2: effectivePermissions 三表 join 与单测

**Files:**
- Modify: `internal/api/permission.go`
- Modify: `internal/api/permission_test.go`

- [ ] **Step 1: 更新单测（先删 unionPermissions 测试，加 hasPermission 保留）**

删除 `TestUnionPermissions`；保留 `TestHasPermission`（`hasPermission` 仍用于 requirePermission）。

- [ ] **Step 2: 实现三表 join**

```go
func effectivePermissions(db *gorm.DB, username string) []string {
	var user model.User
	if db.Where("username = ?", username).First(&user).Error != nil {
		return nil
	}
	var roleIDs []uint
	db.Model(&model.UserRole{}).Where("user_id = ?", user.ID).Pluck("role_id", &roleIDs)
	if len(roleIDs) == 0 {
		return nil
	}
	var permIDs []uint
	db.Model(&model.RolePermission{}).Where("role_id IN ?", roleIDs).Pluck("permission_id", &permIDs)
	if len(permIDs) == 0 {
		return nil
	}
	var codes []string
	db.Model(&model.Permission{}).Where("id IN ?", permIDs).Pluck("code", &codes)
	return codes
}
```

删除 `unionPermissions`。

- [ ] **Step 3: 编译 + 测试 + 提交**

```bash
go build ./internal/...
go test ./internal/api/ -run TestHasPermission
git add internal/api/permission.go internal/api/permission_test.go
git commit -m "refactor: effectivePermissions 改为权限表三表 join"
```

---

## 阶段二：后端接口

### Task 3: 角色 CRUD 用 permissionIds + 权限点 CRUD

**Files:**
- Modify: `internal/api/handlers_admin.go`
- Modify: `internal/api/handlers_admin_test.go`
- Modify: `internal/api/server.go`

- [ ] **Step 1: 角色 payload 改为 permissionIds**

`rolePayload` 改为：

```go
type rolePayload struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	PermissionIDs []uint `json:"permissionIds"`
}

func validateRole(p rolePayload) string {
	if strings.TrimSpace(p.Name) == "" {
		return "角色名不能为空"
	}
	return ""
}
```

- [ ] **Step 2: 角色 CRUD 处理 role_permissions**

`adminCreateRole`/`adminUpdateRole`：创建/更新角色后，重写 `role_permissions`（删除旧的，批量建新的）。校验 `PermissionIDs` 都在权限表存在（无效返回 400）。

- [ ] **Step 3: 权限点 CRUD**

```go
func adminListPermissions(db *gorm.DB) gin.HandlerFunc  // GET /api/admin/permissions
func adminCreatePermission(db *gorm.DB) gin.HandlerFunc  // POST（code 唯一校验）
func adminDeletePermission(db *gorm.DB) gin.HandlerFunc  // DELETE /:id（被角色引用返回 409）
```

`adminCreatePermission` 校验 code 非空、唯一；`adminDeletePermission` 校验无 role_permissions 引用。

- [ ] **Step 4: 注册路由**

server.go admin 组加：

```go
	admin.GET("/permissions", requirePermission(db, "user:manage"), adminListPermissions(db))
	admin.POST("/permissions", requirePermission(db, "user:manage"), adminCreatePermission(db))
	admin.DELETE("/permissions/:id", requirePermission(db, "user:manage"), adminDeletePermission(db))
```

- [ ] **Step 5: 测试 + 编译 + 提交**

`handlers_admin_test.go` 加 `TestValidateRole`（name 空 → 无效；合法 → 通过）。注意：原 `TestValidateRole` 用 `Permissions []string` 改 `PermissionIDs []uint`。

```bash
go test ./internal/api/ -run TestValidateRole
go build ./internal/...
git add internal/api/handlers_admin.go internal/api/handlers_admin_test.go internal/api/server.go
git commit -m "feat: 角色按权限ID分配 + 权限点管理接口"
```

### Task 4: 清理旧代码 + 路由 code 字面量替换

**Files:**
- Modify: `internal/model/role.go`
- Modify: `internal/api/permission.go`
- Modify: `internal/api/server.go`
- Modify: `internal/api/handlers_admin.go`（防提权守卫里的 `model.PermUser` → `"user:manage"`）

- [ ] **Step 1: 删除旧字段/常量/死代码**

`internal/model/role.go` 删除 `Role.Permissions []string`、`PermXxx` 常量块、`AllPermissions`（保留 `RoleAdmin`/`RoleCustomer`）。`internal/api/permission.go` 删除 `unionPermissions`。`handlers_admin.go` 的 `validateRole` 已在 Task 3 改为只校验 name，确认不再引用 `AllPermissions`。

- [ ] **Step 2: 替换所有 `model.PermXxx` 为字面量**

把 server.go 里 `requirePermission(db, model.PermProduct)` 等改为 `requirePermission(db, "product:manage")` 等；`adminUpdateUserStatus` / 角色创建守卫里的 `model.PermUser` → `"user:manage"`。

- [ ] **Step 3: 编译 + 提交**

```bash
go build ./internal/...
git add internal/model/role.go internal/api/permission.go internal/api/server.go internal/api/handlers_admin.go
git commit -m "refactor: 移除硬编码权限常量，权限点改用字面量 code"
```

---

## 阶段三：前端

### Task 5: store hasPerm + 集中式菜单配置

**Files:**
- Modify: `frontend/src/stores/user.ts`
- Create: `frontend/src/lib/admin-menu.ts`
- Modify: `frontend/src/layouts/AdminLayout.vue`
- Modify: `frontend/src/views/admin/DashboardView.vue`
- Modify: `frontend/src/views/ProfileView.vue`

- [ ] **Step 1: store 加 hasPerm**

```ts
function hasPerm(code: string) {
  return permissions.value.includes(code);
}
```
返回对象暴露 `hasPerm`。

- [ ] **Step 2: 菜单配置**

`admin-menu.ts`：

```ts
export interface AdminMenuItem {
  title: string;
  path: string;
  perm?: string; // 空 = 始终显示（概览）
}
export const adminMenu: AdminMenuItem[] = [
  { title: "概览", path: "/admin" },
  { title: "商品", path: "/admin/products", perm: "product:manage" },
  { title: "分类", path: "/admin/categories", perm: "category:manage" },
  { title: "订单", path: "/admin/orders", perm: "order:manage" },
  { title: "用户", path: "/admin/users", perm: "user:manage" },
  { title: "优惠券", path: "/admin/coupons", perm: "coupon:manage" },
  { title: "通知", path: "/admin/notifications", perm: "notification:manage" },
  { title: "权限", path: "/admin/permissions", perm: "user:manage" },
  { title: "角色", path: "/admin/roles", perm: "user:manage" },
];
```

- [ ] **Step 3: 侧边栏/概览/入口走 hasPerm 或菜单配置**

`AdminLayout.vue` 侧边栏改为 `v-for` 遍历 `adminMenu`，`v-if="!item.perm || userStore.hasPerm(item.perm)"`。`DashboardView.vue` 用 `hasPerm`。`ProfileView.vue` 入口用 `hasPerm`（`isAdmin` 保留亦可）。

- [ ] **Step 4: 构建 + 提交**

```bash
cd frontend && npm run build
git add frontend/src/stores/user.ts frontend/src/lib/admin-menu.ts frontend/src/layouts/AdminLayout.vue frontend/src/views/admin/DashboardView.vue frontend/src/views/ProfileView.vue
git commit -m "feat: 前端 hasPerm 与集中式菜单配置"
```

### Task 6: 角色权限动态拉取 + 权限管理页

**Files:**
- Modify: `frontend/src/views/admin/RolesView.vue`
- Create: `frontend/src/views/admin/PermissionsView.vue`
- Modify: `frontend/src/router/index.ts`

- [ ] **Step 1: RolesView 权限复选动态拉取**

`RolesView` 从 `GET /api/admin/permissions` 拉权限列表，表单复选改为 `permissionIds`；`openEdit` 时从角色已有权限回填（后端角色列表应返回 `permissionIds` 或单独查 role_permissions）。

- [ ] **Step 2: PermissionsView（列表 + 新建/删除）**

表格：编码/显示名/路由 + 删除；新建弹窗：code/name/path。调 `GET/POST /api/admin/permissions`、`DELETE /api/admin/permissions/:id`。

- [ ] **Step 3: 注册 `/admin/permissions` 路由 + 构建 + 提交**

```bash
cd frontend && npm run build
git add frontend/src/views/admin/RolesView.vue frontend/src/views/admin/PermissionsView.vue frontend/src/router/index.ts
git commit -m "feat: 权限管理页与角色权限动态拉取"
```

---

## 收尾

### Task 7: 全量回归与推送

- [ ] **Step 1: 后端全量测试**

Run: `go test ./internal/... -skip TestSSEPushEndToEnd`
Expected: 通过。

- [ ] **Step 2: 前端构建 + 重建桌面应用**

`cd frontend && npm run build`；`wails3 build` 后重启。

- [ ] **Step 3: 端到端冒烟**

新建权限点 → 建角色并分配 → 受限用户访问对应路由 200 / 无权限 403；删除被引用权限 → 409。

- [ ] **Step 4: 提交并推送**

```bash
git add -A
git commit -m "feat: 动态权限 RBAC 重构完成"  # 若前面已提交则省略
git push
```
