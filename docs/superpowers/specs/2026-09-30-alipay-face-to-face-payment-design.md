# 支付宝沙箱 + 当面付扫码支付设计

- 日期：2026-09-30
- 状态：待评审
- 规模：架构级（新增外部支付子系统，替换现有模拟支付）

## 目标

把当前"模拟支付"（`payOrder` 直接把订单 `pending → paid`）替换为真实支付宝对接：收银台生成当面付二维码，用户用手机支付宝扫码付款，后端确认到账后把订单标记为已支付。使用支付宝**沙箱环境** + **当面付预下单（`alipay.trade.precreate`）**，不接真实商户号、不扣真实金额。

## 现状问题

- `internal/api/handlers_order.go:150` 的 `payOrder` 是模拟实现：不调任何外部渠道，直接改状态。
- `frontend/src/views/PayView.vue` 收银台提供"微信/支付宝/银行卡"三种方式，但都打同一个假接口。
- `internal/api/simulators.go:36-45` 的"商家自动发货"每 20s 把最早的 `pending` 订单直接改成 `shipped`——一旦支付真实化，未付款订单会被"发货"，必须修正为只发 `paid` 订单。
- 订单模型无任何支付流水字段（交易号、支付方式、支付时间）。

## 关键决策

1. **支付确认：轮询为主 + 异步通知兜底**。本地 Wails 应用没有公网地址，沙箱/支付宝回调到不了，故以 `alipay.trade.query` 主动查询作为主闭环；`notify_url` 处理器一并实现，生产部署到公网服务器后自动生效。
2. **产品选型：当面付预下单**。后端用 `alipay.trade.precreate` 拿二维码串，前端展示二维码，用户手机扫码付款，最贴合桌面/手机 App 收银台场景。
3. **环境：沙箱**。网关默认 `https://openapi-sandbox.dl.alipaydev.com/gateway.do`；配置全部走环境变量，未配置时优雅降级（返回 503 "支付未配置"），App 其余功能不受影响。
4. **二维码渲染：后端生成 base64 PNG**，复用已有 `github.com/skip2/go-qrcode`，前端只 `<img :src>`，零新增前端依赖。
5. **数据存储：Order 加字段**（不建独立 Payment 表），符合现有"订单自包含"风格，将来对账/退款再拆表。
6. **SDK：`github.com/smartwalle/alipay/v3`**（Go 生态事实标准），支持 precreate / query / 通知验签。

## 数据模型

`shop_orders` 表新增三个字段（`AutoMigrate` 自动加列）：

| 字段 | 类型 | 说明 |
|---|---|---|
| trade_no | string(64) | 支付宝交易号，支付成功后回填 |
| pay_method | string(16) | 支付方式，固定 `alipay` |
| paid_at | *time.Time | 支付成功时间 |

状态流转变为：`pending`（待付款）→ 支付宝确认后 → `paid`（待发货）→ 后续不变。

## 后端设计

- 新增 `internal/api/payment.go`：
  - `alipayClient` 封装（持 `*alipay.Client`），注入接口便于测试 mock。
  - `newAlipayClient()` 从环境变量读 `ALIPAY_APP_ID` / `ALIPAY_PRIVATE_KEY` / `ALIPAY_PUBLIC_KEY` / `ALIPAY_GATEWAY`（默认沙箱），任一缺失返回"未配置"。
  - `precreatePayment`：`alipay.trade.precreate`（`out_trade_no` = OrderNo，`total_amount` = `%.2f` 金额，`subject` = 订单号），返回二维码串。
  - `queryPayment`：`alipay.trade.query`，识别 `TRADE_SUCCESS` / `WAIT_BUYER_PAY` / `TRADE_CLOSED`。
  - `markPaid(db, orderID, tradeNo)`：幂等——仅当 `status == pending` 才写 `paid` + `trade_no` + `paid_at` + `pay_method`，防轮询/通知并发双写。
  - `handleNotify`：`client.DecodeNotification` 验签，验签通过且 `TRADE_SUCCESS`/`FINISHED` 时 `markPaid`，回 `"success"`。
- 替换 `handlers_order.go` 的 `payOrder`：
  - `POST /api/orders/:id/pay` → 校验 `pending` → `precreatePayment` → 返回 `{ qrImage, orderNo, amount }`（`qrImage` 为 base64 data URL）。
  - `GET /api/orders/:id/pay/status` → 若订单已 `paid` 直接返回 `paid`；否则 `queryPayment`，成功则 `markPaid` 后返回 `paid`；未支付返回 `pending`；已关闭返回 `closed`。
- 新增公开路由 `POST /api/pay/alipay/notify`（**不走 JWT**）→ `handleNotify`。
- 修正 `simulators.go`：`shipNextOrder` 只把 `paid` 订单改成 `shipped`，不再碰 `pending`。
- `server.go`：注册上述路由；`AutoMigrate` 无需改（GORM 自动加列）。

## 前端设计

- 重写 `PayView.vue`：去掉"微信/支付宝/银行卡"三选一，改成单一支付宝当面付面板——金额 + 二维码 + 订单号 + "已付款？点此刷新"。
- 挂载即 `POST /api/orders/:id/pay` 拉二维码；`setInterval` 每 2s `GET /api/orders/:id/pay/status`，`paid` 时清理定时器 + toast + 跳 `/orders`；`closed` 或接口报错时给提示并停止轮询。
- 订单已支付时（进入收银台前已 paid）直接提示并跳转。

## 错误处理

- 配置缺失 → `503 "支付未配置"`，前端提示"支付服务暂未配置"。
- 支付宝 API 失败 → 透传错误 message，前端 toast。
- 非 `pending` 订单请求支付 → `400 "订单状态不可支付"`。
- 幂等：`markPaid` 以 `status == pending` 为前置条件，轮询与通知并发只落一次库。

## 测试

- mock 支付宝 client（注入接口），单测：
  - 预下单成功返回二维码 / 配置缺失返回 503。
  - 查询未支付 → `pending`；成功 → 落库 `paid` + `trade_no`。
  - 幂等：重复通知/重复轮询不会二次更新。
  - 通知验签失败不落库。
- `simulators` 单测：只发 `paid`，不发 `pending`。

## 兼容性与迁移

- 已有 `pending` 订单不受影响；无支付字段的旧订单，字段留空即可。
- 移除旧 `payOrder` 模拟实现与其相关断言。

## 沙箱实测步骤（写入 spec 供联调）

1. 在支付宝开放平台申请沙箱应用，获取 `app_id`、应用私钥、支付宝公钥。
2. 生成沙箱买家账号（手机号 + 支付密码）。
3. 设置环境变量后启动，下单进入收银台，出现二维码。
4. 手机支付宝登录沙箱买家（或使用开放平台沙箱收银台工具）扫码付款。
5. 前端轮询到 `paid`，订单流转为待发货，不扣真实金额。

## 非目标（YAGNI）

- 不接微信支付 / 银行卡。
- 不做退款（`alipay.trade.refund`）、对账、支付流水表。
- 不做真实商户号接入（仅沙箱；生产仅需换网关 + 密钥 + 公网 notify_url，代码不变）。
- 不在前端引入二维码 npm 库（后端生成）。
