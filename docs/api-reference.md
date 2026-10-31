# API 参考

## 通用约定

- 基础地址默认为 `http://127.0.0.1:8080`，前端可通过 `VITE_API_BASE` 覆盖。
- 需要登录的接口使用 `Authorization: Bearer <JWT>`。
- JSON 字段使用 camelCase；错误通常返回 `{ "message": "..." }`。
- 金额接口使用整数分。例：`19900` 表示 `199.00` 元。
- 列表接口通常支持 `page`、`pageSize`，具体处理器会限制最大页大小。
- 订单、购物车和支付接口以后端数据库数据为准，客户端提交的名称、价格和展示字段不会直接成为最终业务数据。

## 健康与目录

| 方法 | 路径 | 登录 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/health` | 否 | 服务和数据库可用性 |
| GET | `/api/categories` | 否 | 分类列表 |
| GET | `/api/products` | 否 | 商品列表，支持分类、精选和排序参数 |
| GET | `/api/products/:id` | 否 | 商品详情和规格 |
| GET | `/api/products/:id/reviews` | 否 | 商品评价摘要和列表 |
| POST | `/api/cart/preview` | 否 | 只读刷新游客/本地购物车商品、价格、库存状态 |

## 认证与用户

| 方法 | 路径 | 登录 | 说明 |
| --- | --- | --- | --- |
| POST | `/api/register` | 否 | 注册用户名和密码 |
| POST | `/api/login` | 否 | 用户名或手机号登录，返回 JWT |
| GET | `/api/me` | 是 | 当前用户资料和权限 |
| PUT | `/api/profile` | 是 | 更新昵称、手机号、邮箱、头像信息 |
| POST | `/api/profile/avatar` | 是 | 上传用户头像 |

## 地址、优惠券和通知

| 方法 | 路径 | 登录 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/addresses` | 是 | 地址列表 |
| POST | `/api/addresses` | 是 | 新增地址 |
| PUT | `/api/addresses/:id` | 是 | 修改地址 |
| DELETE | `/api/addresses/:id` | 是 | 删除地址 |
| PUT | `/api/addresses/:id/default` | 是 | 设置默认地址 |
| GET | `/api/coupons` | 是 | 可领取优惠券 |
| GET | `/api/coupons/mine` | 是 | 当前用户优惠券 |
| POST | `/api/coupons/:id/claim` | 是 | 领取优惠券 |
| GET | `/api/notifications` | 是 | 通知列表 |
| GET | `/api/notifications/unread` | 是 | 未读数量 |
| GET | `/api/notifications/stream?token=...` | 是 | SSE 实时通知 |
| PUT | `/api/notifications/:id/read` | 是 | 标记单条已读 |
| PUT | `/api/notifications/read-all` | 是 | 全部已读 |
| DELETE | `/api/notifications/:id` | 是 | 删除单条通知 |
| DELETE | `/api/notifications` | 是 | 清空通知 |

## 购物车与结算

| 方法 | 路径 | 登录 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/cart` | 是 | 返回 `{ version, items }`，含当前价格、库存和失效原因 |
| PUT | `/api/cart` | 是 | 按版本提交购物车快照；过期版本返回 409 |
| POST | `/api/cart/checkout` | 是 | 原子创建订单、扣除购买数量并记录幂等请求 |

`PUT /api/cart` 请求示例：

```json
{
  "version": 3,
  "items": [
    { "productId": 12, "skuId": 4, "quantity": 2 }
  ]
}
```

`POST /api/cart/checkout` 至少包含：

```json
{
  "requestId": "checkout-unique-id",
  "version": 4,
  "addressId": 8,
  "userCouponId": 0,
  "expectedAmount": 39800,
  "items": [
    { "productId": 12, "skuId": 4, "quantity": 2, "price": 19900 }
  ]
}
```

同一用户重复提交相同 `requestId` 和内容会返回同一订单；内容不同返回 409。价格、库存、优惠券、地址或购物车版本变化会拒绝结算，页面需要刷新并重新确认。

## 订单、支付、评价和售后

| 方法 | 路径 | 登录 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/orders` | 是 | 当前用户订单列表，可按状态分页 |
| POST | `/api/orders` | 是 | 兼容的直接下单接口；新购物车页面使用 `/cart/checkout` |
| GET | `/api/orders/:id` | 是 | 订单详情和物流/售后信息 |
| POST | `/api/orders/:id/pay` | 是 | 创建支付宝当面付预下单 |
| GET | `/api/orders/:id/pay/status` | 是 | 查询支付状态 |
| PUT | `/api/orders/:id/status` | 是 | 用户取消、确认收货或申请允许的状态变更 |
| POST | `/api/orders/:id/reviews` | 是 | 提交订单商品评价 |
| POST | `/api/orders/:id/aftersale` | 是 | 提交整单售后申请 |
| PUT | `/api/orders/:id/return` | 是 | 提交退货物流信息 |
| POST | `/api/pay/alipay/notify` | 否 | 支付宝异步通知回调 |

## 客服

| 方法 | 路径 | 登录 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/support` | 是 | 当前用户咨询列表 |
| POST | `/api/support` | 是 | 创建咨询或发送用户消息 |
| GET | `/api/support/:id` | 是 | 咨询详情和消息 |
| POST | `/api/support/:id/messages` | 是 | 用户回复 |

## 管理接口

所有管理接口都需要登录，并由 `requirePermission` 校验权限。权限代码包括 `user:manage`、`product:manage`、`category:manage`、`order:manage`、`coupon:manage`、`notification:manage`、`review:manage` 和 `support:manage`。

| 资源 | 路径 | 主要操作 |
| --- | --- | --- |
| 用户 | `/api/admin/users` | 列表、启停用、重置密码 |
| 角色 | `/api/admin/roles` | 列表、新增、修改、删除 |
| 权限 | `/api/admin/permissions` | 列表、新增、删除 |
| 用户角色 | `/api/admin/users/:id/roles` | 查询和设置角色 |
| 商品 | `/api/admin/products` | 列表、新增、修改、删除、精选、图片上传 |
| SKU | `/api/admin/products/:id/skus`、`/api/admin/skus/:id` | 查看、新增、修改、删除规格 |
| 分类 | `/api/admin/categories` | 列表、新增、修改、删除 |
| 订单 | `/api/admin/orders` | 列表、详情、状态、发货、物流、售后 |
| 优惠券 | `/api/admin/coupons` | 列表、新增、修改、删除 |
| 通知 | `/api/admin/notifications` | 发送站内通知 |
| 评价 | `/api/admin/reviews` | 列表和管理 |
| 客服 | `/api/admin/support` | 查看人工队列、回复和结束会话 |

## 状态码重点

| 状态码 | 常见含义 |
| --- | --- |
| 400 | 请求字段、商品、规格、库存、优惠券或地址不合法 |
| 401 | 未登录或 JWT 无效 |
| 403 | 没有管理权限或不允许当前状态流转 |
| 404 | 商品、订单、地址或资源不存在 |
| 409 | 购物车版本、金额、价格或幂等请求冲突 |
| 503 | 支付服务未配置或支付服务暂不可用 |
