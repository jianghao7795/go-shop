# 部署与运维

## 生产部署建议

推荐使用 `server` 模式部署 HTTP 服务，前置 HTTPS 反向代理。桌面 Wails 构建适合本地客户端，不建议把生产数据库凭据写入客户端环境。

```powershell
# 在发布机上构建前端和服务端
cd frontend
npm ci
npm run build
cd ..
go build -tags server,production -trimpath -buildvcs=false -ldflags="-s -w" -o bin/shop-server.exe
```

Linux/macOS 使用对应的可执行文件扩展名。也可以：

```powershell
task build:server
task build:docker
```

Docker 服务端镜像默认使用纯 Go 静态构建和 distroless 运行时，容器内部监听 8080。通过反向代理暴露 HTTPS，并将 `API_PORT`、MySQL、Redis、支付宝和 CORS 环境变量注入容器。

## 启动检查

启动时按顺序检查：

1. MySQL 连接和数据库创建权限。
2. 金额、购物车和订单生命周期迁移。
3. GORM 表结构和索引。
4. 商品、分类、优惠券和角色权限种子数据。
5. Redis Ping；失败时记录警告并降级。
6. 支付宝配置；缺失时仅禁用支付。
7. Gin 监听 `API_PORT`。

健康检查：

```powershell
curl http://127.0.0.1:8080/api/health
```

## 数据备份

至少备份以下内容：

- MySQL 全库，尤其是 `shop_orders`、`shop_user_coupons`、`shop_cart_items`、`shop_checkout_requests` 和 `shop_migrations`。
- `uploads/` 目录，它包含头像和商品图片。
- 支付宝配置文件应通过密钥管理系统单独备份，不放入代码仓库。

示例：

```powershell
mysqldump --single-transaction --routines --triggers shop > shop-backup.sql
```

恢复前先停止写入，确认应用版本与数据库迁移兼容，再恢复数据库和上传目录。

## 迁移和回滚

应用启动会自动迁移，但生产建议先做备份并在维护窗口重启。购物车专项迁移可单独执行：

```powershell
go run ./cmd/cart-migrate
go run ./cmd/cart-migrate -apply
```

`-apply` 使用 MySQL 命名锁；购物车非空时先创建备份表。迁移失败时保留备份表和错误日志，不要直接删除原表。金额迁移由 `shop_migrations` 中的版本键保护，重复启动不会重复乘以 100。

## 监控重点

- API `/api/health` 状态和响应时间。
- MySQL 连接池、锁等待、慢查询和磁盘空间。
- Redis 可用性、缓存命中和限流降级日志。
- 支付预下单、支付补查、关单和退款重试日志。
- `uploads/` 磁盘使用量。
- 待付款订单数量、过期订单和售后处理中订单。
- `shop_checkout_requests` 与订单数量是否出现异常增长。

## 故障排查

### API 启动后只有目录可用

检查 MySQL 地址、账号、密码和创建数据库权限。服务端在数据库不可用时可以提供目录兜底，但用户、订单、购物车和管理功能无法正常使用。

### 登录后频繁失效

多实例部署时必须设置相同的 `JWT_SECRET`。未设置时每次进程启动会随机生成密钥，旧 token 会全部失效。

### Redis 报错

确认 `REDIS_ADDR`、密码和网络。Redis 只影响目录缓存、权限缓存和分布式限流；恢复后重启服务即可重新 Ping，缓存会按 TTL 重建。

### 购物车版本冲突

409 表示其他设备或页面已经更新购物车。前端会加载服务端快照并提示重新操作。不要通过手工重放旧的 `PUT /api/cart` 覆盖新版本。

### 订单结果未知

结算请求会写入本地 pending 请求并使用固定 `requestId`。刷新确认页面后重试同一请求即可，不能创建新的 requestId 规避状态。服务器会根据 `shop_checkout_requests` 返回原订单。

### 支付不可用

检查 `config/alipay.local.json` 或 `ALIPAY_*` 环境变量，确认私钥、公钥格式和 `ALIPAY_PRODUCTION` 值。没有公网 HTTPS 时支付宝异步回调不可达，但页面轮询和后端补查仍可同步支付结果。

### 上传文件丢失

确认 `uploads/` 是持久化卷，反向代理允许上传大小，并检查应用用户的读写权限。数据库中的图片路径不会恢复文件内容。

## 安全基线

- 生产覆盖代码中的 MySQL 开发默认值，使用最小权限数据库账号。
- 固定并保密 `JWT_SECRET`，不要把支付宝私钥和 Redis 密码写入日志。
- 仅在必要时配置 `CORS_ALLOWED_ORIGINS`，不要使用任意来源。
- 关闭 `ENABLE_SIMULATORS`，除非是隔离测试环境。
- 支付通知必须使用公网 HTTPS，验签和金额校验失败时保持订单未确认。
- 管理权限由服务端校验；前端菜单隐藏只用于体验，不是授权机制。
