# 管理后台设计

- 日期：2026-09-24
- 状态：待评审
- 规模：架构级（新增子系统）

## 目标

给商城加一个管理后台：管理员可以管理商品、分类、订单、用户、优惠券，并发送站内通知。

## 现状

- `User.Role`（`customer`/`admin`）字段与 JWT `role` claim 已存在，但**没有任何地方校验角色**，`admin` 目前只是存着没用。
- 商品/分类/优惠券目前只有**查询**接口（`/api/products`、`/api/categories`、`/api/coupons`），无增删改。
- 订单仅支持查看/修改**自己**的订单。
- 无管理员界面、无种子管理员。

## 关键决策

1. **范围**：全模块（商品、分类、订单、用户、优惠券、通知）。
2. **管理员认定**：手动在数据库把某用户的 `role` 改成 `admin`。不写种子、不做「提权」UI。
3. **形态**：同一个 Wails 应用内新增 `/admin` 路由区 + 独立后台布局（复用同一套 API 与前端工程，不另起应用）。

## 总体架构

单代码库内扩展：

- 后端：在现有 Gin API 上新增 `/api/admin/*` 路由组，挂「JWT + 管理员」双重中间件；复用现有 model，补齐缺失的 CRUD handler。
- 前端：在现有 Vue 应用里新增 `/admin/*` 路由段 + 独立 `AdminLayout`；普通用户不可见、不可进。

## 后端设计

### 权限

新增 `adminRequired()` 中间件：从 JWT 取 `role`，非 `admin` 返回 403。挂到 `/api/admin` 组（该组外层同时挂 JWT 中间件）。

### Admin API（均在 `/api/admin` 下，全部要求 admin）

| 模块 | 端点 |
|---|---|
| 商品 | `GET/POST /products`、`PUT/DELETE /products/:id`（含上/下架） |
| 分类 | `GET/POST /categories`、`PUT/DELETE /categories/:id` |
| 订单 | `GET /orders`（全部）、`GET /orders/:id`、`PUT /orders/:id/status` |
| 用户 | `GET /users`（全部）、`PUT /users/:id/status`（禁用/启用） |
| 优惠券 | `GET/POST /coupons`、`PUT/DELETE /coupons/:id` |
| 通知 | `POST /notifications`（发给指定用户或广播） |

约定：管理接口返回全部数据，不回退到种子/兜底数据；写操作做基本校验（必填、类型、状态取值）。

## 前端设计

- 路由：`/admin`（概览）、`/admin/products`、`/admin/categories`、`/admin/orders`、`/admin/users`、`/admin/coupons`、`/admin/notifications`。
- 布局：独立 `AdminLayout`（侧边导航），**不含**商城 tabbar。
- 入口：个人中心在 `role === admin` 时显示「管理后台」入口。
- 守卫：`/admin/*` 要求已登录且 `role === admin`，否则跳回商城。
- store：`user.ts` 增加 `role` 字段（从 `/api/me` 读取并保存）。

## 数据模型变更

- `Product` 增加 `OnShelf bool`（默认 `true`），用于上/下架。由 AutoMigrate 加列。
- 其余复用现有 model，不改表结构。

## 分阶段实施（每阶段独立验收）

1. 权限中间件 + admin 布局 + 用户管理
2. 商品 + 分类管理
3. 订单管理
4. 优惠券 + 通知管理

## 测试

- 后端：校验/权限逻辑尽量抽成纯函数做单测；管理 CRUD 走 curl 端到端验证（含非 admin 访问 `/api/admin/*` 应 403）。
- 前端：`vite build` 通过 + 手动验证后台页面与守卫。

## 非目标（YAGNI）

- 不做独立的管理端应用。
- 不做角色分配/提权 UI（管理员手动设库）。
- 不做富文本、图片上传、审计日志、数据统计图表（概览页先给简单统计即可）。
