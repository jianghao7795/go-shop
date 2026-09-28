# 动态权限 RBAC 重构设计

- 日期：2026-09-28
- 状态：待评审
- 规模：架构级（在现有 RBAC 上重构权限为数据驱动）

## 目标

解决当前 RBAC 权限点硬编码、无法扩展的问题：把权限点改为数据库驱动，新增一个模块的权限只需「插一行数据 + 路由写 code」，不用再改 Go 常量 / 前端标签列表。同时提供权限点管理页，并统一前端权限判断。

## 现状问题

- 权限点是硬编码 Go 常量（`PermXxx`）+ `AllPermissions` 列表 + 前端 `PERMISSIONS` 标签列表 + 种子四处。
- 加一个新模块的权限，要同时改这四处。
- 前端权限判断散落各处（`permissions.includes(...)`）。

## 关键决策

1. **权限点数据驱动**：新增 `shop_permissions` 表（`code` 唯一 + `name` 显示名 + `path` 后台路由）。
2. **角色↔权限多对多**：新增 `shop_role_permissions` 关联表，替换 `Role.Permissions` JSON 字段。
3. **权限点管理**：后台可新建/删除权限点（仅限持有 `user:manage` 的超管）。
4. **前端统一**：权限判断收敛到 store 的 `hasPerm(code)` 助手 + 集中式后台菜单配置，侧边栏/概览按权限渲染。
5. **内置权限与角色保护**：`admin`/`customer` 角色仍不可改删；权限点删除时若有角色引用需处理。

## 数据模型

- `shop_permissions` 表：

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint PK | 主键 |
| code | string(64) uniqueIndex | 权限编码，如 `product:manage` |
| name | string(64) | 显示名，如「商品管理」|
| path | string(128) | 后台路由，如 `/admin/products` |
| created_at / updated_at / deleted_at | | 时间戳 + 软删除 |

- `shop_role_permissions` 关联表：

| 字段 | 类型 | 说明 |
|---|---|---|
| id | uint PK | 主键 |
| role_id | uint index | 角色 ID |
| permission_id | uint index | 权限 ID |

- `shop_roles`：去掉 `permissions` 字段。
- `shop_user_roles`：不变。

## 后端设计

- 删除 `PermXxx` / `AllPermissions` 常量（角色名常量 `RoleAdmin`/`RoleCustomer` 保留）。
- `effectivePermissions(db, username)`：user_roles → role_permissions → permissions.code 三表 join 取并集。
- `requirePermission(db, code)`：判断有效权限集是否含 `code`（code 是 `shop_permissions.code` 字符串）。
- 权限点接口（`/api/admin/permissions`，均需 `user:manage`）：`GET`（列表）、`POST`（新建，code 唯一校验）、`DELETE /:id`（删除；被角色引用时禁止删除）。
- 角色 CRUD：角色带 `permissionIds`（多选），创建/更新时重写 role_permissions。
- 种子：6 个初始权限（code/name/path）+ admin 角色关联全部权限 + customer 空权限。

## 前端设计

- 新增「权限管理」页：权限列表（编码/显示名/路由）+ 新建/删除。
- 角色管理表单的权限复选，从 `GET /api/admin/permissions` 动态拉取。
- 集中式菜单配置 + `hasPerm`：侧边栏/概览/个人中心入口统一走 `hasPerm(code)`，不再散落 `includes`。

## 兼容性与迁移

- 现有角色（admin/customer）迁移：admin 角色关联全部 6 个权限；其余自定义角色（若有）按原 permissions JSON 迁移到 role_permissions。
- 现有用户-角色关联不变。

## 测试

- `effectivePermissions` 的 join 逻辑 + 权限并集纯函数单测。
- 端到端：新建权限 → 分配给角色 → 受限用户访问对应路由 200 / 无权限 403。

## 非目标（YAGNI）

- 不做权限点的编辑（code 是路由引用的稳定标识，改名需改代码，只做新建/删除）。
- 不做菜单项在数据库里任意排序/分组（path 即路由，侧边栏按权限表顺序展示）。
- 不做多角色继承/层级。
