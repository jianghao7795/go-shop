# 支付宝当面付配置说明

对接支付宝当面付（`alipay.trade.precreate`）。所有配置走环境变量，未配置时支付功能优雅降级（返回 503），App 其它功能不受影响。

## 环境变量

| 变量 | 必填 | 说明 |
|---|---|---|
| `ALIPAY_APP_ID` | 是 | 应用 AppID |
| `ALIPAY_PRIVATE_KEY` | 是 | 应用私钥，PKCS8（`-----BEGIN PRIVATE KEY-----`）；SDK 也兼容 PKCS1 |
| `ALIPAY_PUBLIC_KEY` | 是 | 支付宝公钥，PKCS8/PKIX（`-----BEGIN PUBLIC KEY-----`） |
| `ALIPAY_PRODUCTION` | 否 | 仅当值严格等于 `true` 时走生产网关，其余一律沙箱（**大小写敏感**：`True`/`TRUE`/`1` 都不算） |
| `ALIPAY_NOTIFY_URL` | 否 | 异步通知回调地址，生产环境需公网可达，如 `https://example.com/api/pay/alipay/notify` |

任一必填项缺失 → 支付返回 503「支付未配置」；必填项齐全但密钥非法（格式错误）→ 503「支付配置错误，请联系管理员」。

## 密钥格式

- **应用私钥**：开放平台生成后导出为 PKCS8（`BEGIN PRIVATE KEY`）。若拿到的是 PKCS1（`BEGIN RSA PRIVATE KEY`），SDK 也能识别（先试 PKCS1 再试 PKCS8）。
- **支付宝公钥**：开放平台「支付宝公钥」内容，必须为 PKCS8/PKIX（`BEGIN PUBLIC KEY`），SDK 用 `PKIX().RSAPublicKey()` 解析。

## 沙箱 vs 生产

- 默认走沙箱网关 `https://openapi-sandbox.dl.alipaydev.com/gateway.do`。
- 设置 `ALIPAY_PRODUCTION=true`（小写、精确匹配）切到生产网关。

## 回调与联调

- 本地 Wails 应用无公网地址，支付宝异步回调到不了，故以前端轮询 `alipay.trade.query` 为主闭环；`ALIPAY_NOTIFY_URL` 只在部署到公网服务器后生效。
- 真实支付 / 验签需在支付宝开放平台申请沙箱应用后用真实账号联调，无法用单测覆盖。步骤见 `docs/superpowers/specs/2026-09-30-alipay-face-to-face-payment-design.md` 的「沙箱实测步骤」。
