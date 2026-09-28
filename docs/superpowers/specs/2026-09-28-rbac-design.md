# RBAC 权限管理设计

- 日期：2026-09-28
- 状态：待评审
- 规模：架构级（完全重构权限子系统）

## 目标

完全重构权限系统：用规范的多对多 RBAC 替换现有「单一 admin 角色 + 手动改库」——自定义角色（可建/编辑/删除）+ 固定权限点 + 用户可分配多个角色。

## 现状

- `User.Role`（`customer`/`admin`）单字段，JWT 携带 `role` claim。
- `adminRequired()` 中间件只判断「是否 admin」，无权限粒度。
- 改角色靠手动改数据库。
- 后台模块：商品、分类、订单、用户、优惠券、通知。

## 关键决策

1. **权限点**：固定 6 个，按模块划分：
   - `product:manage`、`category:manage`、`order:manage`、`user:manage`、`coupon:manage`、`notification:manage`。
2. **用户-角色**：**多对多**（`user_roles` 关联表），一个用户可有多个角色，有效权限 = 所有角色权限的并集。
3. **移除 `User.Role`**：彻底废弃单角色字段，改为角色关联表驱动授权。
4. **内置角色**（不可删/改，防锁死）：`admin`（全部权限）、`customer`（无后台权限）。
5. **进入后台的隐含条件**：拥有任一权限点即可进 `/admin`。

## 数据模型

- 新增 `shop_roles` 表：

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint PK | 主键 |
| name | string(64) uniqueIndex | 角色名（唯一）|
| description | string(128) | 描述 |
| permissions | JSON（`[]string`）| 权限点列表 |
| created_at / updated_at / deleted_at | | 时间戳 + 软删除 |

- 新增 `shop_user_roles` 关联表：

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint PK | 主键 |
| user_id | uint index | 用户 ID（`shop_users.id`）|
| role_id | uint index | 角色 ID（`shop_roles.id`）|

- `User` 模型**移除** `Role` 字段（`shop_users.role` 列保留但不再使用，或后续清理）。

## 后端设计

- 新增 `internal/model/role.go`：权限常量、`Role`、`UserRole` 模型、`TableName`。
- 启动时种子：确保存在 `admin`（全部权限）、`customer`（空权限）两个内置角色；迁移现有 `role='admin'` 的用户 → 关联到 `admin` 角色。
- 新增 `effectivePermissions(db, userID) []string`：查该用户所有角色的权限并集。
- `adminRequired` 改为 `requirePermission(db *gorm.DB, perm string)`：
  - 取当前用户 → 查其有效权限集 → 含所需权限点则放行，否则 403。
  - 数据库不可用时回退到 JWT `role`（仅 `admin` 放行，其余拒绝）。
- 各 admin 路由挂对应权限点（商品路由→`product:manage`，订单→`order:manage`，…）。
- 新增角色管理接口（`/api/admin/roles`）：GET/POST/PUT/DELETE；`admin`/`customer` 内置角色不可改/删。
- 新增用户角色分配接口：`GET /api/admin/users/:id/roles`、`PUT /api/admin/users/:id/roles`（传角色 ID 列表）。
- `/api/me` 返回 `permissions`（当前用户有效权限列表），替换原来的 `role`。

## 前端设计

- 新增「角色管理」页：角色列表 + 新建/编辑（名称 + 描述 + 权限多选）+ 删除（内置角色禁用删除/编辑）。
- 「用户管理」页加「分配角色」：多选角色。
- 侧边栏按当前用户权限过滤模块。
- store：`role` 改为 `permissions`（string[]）；「管理后台」入口/路由守卫改为 `permissions.length > 0`。

## 兼容性与迁移

- 现有 `role='admin'` 用户 → 迁移为关联 `admin` 角色（仍超管）。
- 现有 `role='customer'` 用户 → 无角色（无后台权限）。
- 权限变更即时生效（授权读数据库）。

## 测试

- 权限并集/校验逻辑抽纯函数单测。
- 端到端：不同角色组合访问各 admin 模块，验证 200/403。

## 非目标（YAGNI）

- 不做权限点动态增删（权限点是固定常量）。
- 不做菜单级细粒度（一个模块一个权限点）。
- 不做角色继承/层级。
