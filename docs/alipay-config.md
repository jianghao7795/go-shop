# 支付宝当面付配置说明

对接支付宝当面付（`alipay.trade.precreate`）。后端启动时自动读取本地配置文件，同时兼容环境变量。未配置时支付功能返回 503，App 其它功能不受影响。

## 当前准备状态

本地配置为 `config/alipay.local.json`，商户 AppID、应用私钥和支付宝公钥均留空。文件只保存在本机，`config/alipay*` 已加入 Git 忽略列表，不提交商户配置。环境默认为沙箱，未配置时收银台显示支付暂不可用。

打开本地 JSON 文件填写以下字段，保存后重启后端即可生效：

| 配置项 | 说明 |
|---|---|
| `appId` | 商户应用 AppID，暂留空 |
| `privateKey` | 应用私钥，暂留空；PEM 换行在 JSON 字符串中写成 `\n` |
| `publicKey` | 支付宝公钥，暂留空 |
| `production` | `false` 为沙箱，正式收款改为 `true`（JSON 布尔值） |
| `notifyUrl` | 公网 HTTPS 回调地址，暂留空 |

默认先找当前工作目录中的 `config/alipay.local.json`，找不到时再找可执行文件旁的同一路径。也可以用 `ALIPAY_CONFIG_FILE` 指定绝对路径或相对当前工作目录的路径。配置从磁盘读取，不嵌入应用文件，也不会通过 API 返回。

JSON 格式错误、字段拼写错误或明确指定的文件不存在时，支付服务禁用并记录不包含密钥内容的错误。默认文件不存在时仍可使用环境变量；两者都未配置时保持支付禁用。

## 环境变量

| 变量 | 必填 | 说明 |
|---|---|---|
| `ALIPAY_CONFIG_FILE` | 否 | 指定本地 JSON 配置文件路径；默认自动寻找 `config/alipay.local.json` |
| `ALIPAY_APP_ID` | 是 | 应用 AppID |
| `ALIPAY_PRIVATE_KEY` | 是 | 应用私钥，PKCS8（`-----BEGIN PRIVATE KEY-----`）；SDK 也兼容 PKCS1 |
| `ALIPAY_PUBLIC_KEY` | 是 | 支付宝公钥，PKCS8/PKIX（`-----BEGIN PUBLIC KEY-----`） |
| `ALIPAY_PRODUCTION` | 否 | 仅当值严格等于 `true` 时走生产网关，其余一律沙箱（**大小写敏感**：`True`/`TRUE`/`1` 都不算） |
| `ALIPAY_NOTIFY_URL` | 否 | 异步通知回调地址，生产环境需公网可达，如 `https://example.com/api/pay/alipay/notify` |

环境变量在显式设置时优先于对应 JSON 字段，设为空字符串也会覆盖文件值。使用本地文件前，应清除以前设置的同名环境变量；不需要再导入 PowerShell 配置脚本。

文件和环境变量合并后任一必填项缺失 → 支付返回 503「支付未配置」；必填项齐全但密钥非法（格式错误）→ 503「支付配置错误，请联系管理员」。
仅有空白字符的凭据同样视为未配置。没有凭据时不生成真实收款码，也不会将订单标记为付款成功。

## 密钥格式

- **应用私钥**：开放平台生成后导出为 PKCS8（`BEGIN PRIVATE KEY`）。若拿到的是 PKCS1（`BEGIN RSA PRIVATE KEY`），SDK 也能识别（先试 PKCS1 再试 PKCS8）。
- **支付宝公钥**：开放平台「支付宝公钥」内容，必须为 PKCS8/PKIX（`BEGIN PUBLIC KEY`），SDK 用 `PKIX().RSAPublicKey()` 解析。

## 沙箱 vs 生产

- 默认走沙箱网关 `https://openapi-sandbox.dl.alipaydev.com/gateway.do`。
- 设置 JSON 中的 `production: true` 或 `ALIPAY_PRODUCTION=true`（小写、精确匹配）切到生产网关。

## 回调与联调

- 收银台每次查询结束后等待 2 秒再查询，避免请求重叠；失败可手动重试，离开页面时终止请求。付款倒计时使用后端返回的订单截止时间和服务器时间，过期后隐藏收款码并继续核对最终结果。
- 后端每 30 秒运行一批支付补查，查询已发起的待付款订单；因此用户关闭收银台后仍可更新支付结果。每批最多处理 100 单，使用游标继续扫描，并限制每批执行时间。实际同步延迟取决于待处理订单量与支付宝响应。
- 预下单请求在调用支付宝前记录，超时也会进入补查。付款期限仍为创建订单后 30 分钟，过期订单尝试关单；真实支付交易无法确认关闭时保留原状态并继续核对。
- 查询结果校验商户订单号和金额；回调校验签名、AppID、交易号和金额。金额使用整数分比较，缺失或不匹配时不确认付款；数据库读写失败会返回失败，允许支付宝重试通知。
- 支付状态明确区分待付款、已付款、已关闭、售后和已退款；支付成功但库存或优惠券不可用时，收银台引导查看售后详情。
- 本地 Wails 应用无公网地址时，支付宝异步回调不可达，由页面轮询和后台补查同步结果；`notifyUrl` / `ALIPAY_NOTIFY_URL` 在有公网 HTTPS 服务后填写。当前本地配置暂留空，不使用虚构的回调域名。
- 真实支付 / 验签需在支付宝开放平台申请沙箱应用后用真实账号联调，无法用单测覆盖。步骤见 `docs/superpowers/specs/2026-09-30-alipay-face-to-face-payment-design.md` 的「沙箱实测步骤」。

## 不使用商户凭据的验证

```powershell
go test ./internal/api
cd frontend
npm run build
cd ..
go test ./...
```

测试使用临时数据库和支付接口替身，覆盖未配置禁用、支付倒计时、预下单超时补查、分页补查、过期关单、回调金额缺失/不匹配及库存不足转售后。不要将测试替身接入正式收款接口。完整 Go 编译会嵌入 `frontend/dist`，因此应等前端构建结束后再执行 `go test ./...`。
